package main

import (
	"bytes"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// helper struct for working with vault instance in k8s
type vaultHelper struct {
	client     *kubernetes.Clientset
	restConfig *restclient.Config // this was used to create the client above!
	namespace  string
	pod        *v1.Pod
}

// sets a secret by exec'ing into the pod and running the command line client.
func (v *vaultHelper) setSecret(path string, values map[string]string) error {
	valuePairs := []string{}
	for k, v := range values {
		valuePairs = append(valuePairs, fmt.Sprintf("%s=%s", k, v))
	}

	vaultCmd := append([]string{"vault", "kv", "put", "secret/" + path}, valuePairs...)
	// fmt.Printf("vault cmd: %s\n", strings.Join(vaultCmd, " "))

	execRequest := v.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(v.pod.Namespace).
		Name(v.pod.Name).
		SubResource("exec")
	execRequest.VersionedParams(&v1.PodExecOptions{
		Container: v.pod.Spec.Containers[0].Name,
		Command:   vaultCmd,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(v.restConfig, "POST", execRequest.URL())
	if err != nil {
		return fmt.Errorf("failed to init executor: %v", err)
	}

	var execOut bytes.Buffer
	var execErr bytes.Buffer

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
	})

	// fmt.Printf("create out response: %s\n", execOut.String())
	// fmt.Printf("create out error: %s\n", execErr.String())

	if err != nil {
		return fmt.Errorf("error executing stream: %s", err)
	}

	return nil
}

// returns the url to the pod.
func (v *vaultHelper) url() string {
	return fmt.Sprintf("http://%s:%d", v.pod.Status.PodIP, vaultPort)
}

// create a vault instance in k8s.
func (v *vaultHelper) create(wait time.Duration) error {
	// see if it exists first
	pods := v.client.CoreV1().Pods(v.namespace)
	p, err := pods.Get("vault", metav1.GetOptions{})
	if err == nil && p != nil {
		v.pod = p
		return nil
	}

	vaultPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vault",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "vault",
					Image: "vault:1.1.1",
					Env: []v1.EnvVar{
						{Name: "VAULT_DEV_ROOT_TOKEN_ID", Value: testRootToken},
						{Name: "VAULT_TOKEN", Value: testRootToken}, // so the client will work too
						{Name: "VAULT_DEV_LISTEN_ADDRESS", Value: fmt.Sprintf("0.0.0.0:%d", vaultPort)},
						{Name: "VAULT_ADDR", Value: fmt.Sprintf("http://127.0.0.1:%d", vaultPort)},
					},
					ReadinessProbe: &v1.Probe{
						Handler: v1.Handler{
							Exec: &v1.ExecAction{
								Command: []string{"vault", "status"},
							},
						},
						PeriodSeconds:       2,
						InitialDelaySeconds: 2,
						FailureThreshold:    10,
					},
				},
			},
		},
	}

	created, err := pods.Create(vaultPod)
	if err != nil {
		return err
	}
	v.pod = created

	w, err := v.client.CoreV1().Pods(v.namespace).Watch(
		metav1.ListOptions{
			Watch:           true,
			ResourceVersion: created.ResourceVersion,
			FieldSelector:   fields.Set{"metadata.name": v.pod.Name}.AsSelector().String(),
			LabelSelector:   labels.Everything().String(),
		},
	)

	if err != nil {
		return fmt.Errorf("error creating watcher: %s", err)
	}

	var status v1.PodStatus
	func() {
		for {
			select {
			case events, ok := <-w.ResultChan():
				if !ok {
					return
				}
				resp := events.Object.(*v1.Pod)
				v.pod = resp
				status = resp.Status
				fmt.Println("Pod status: ", resp.Status.Phase)
				if resp.Status.Phase != v1.PodPending {
					w.Stop()
				}
			case <-time.After(wait):
				fmt.Println("timeout to wait for pod success")
				w.Stop()
			}
		}
	}()

	if status.Phase != v1.PodRunning {
		return fmt.Errorf("vault pod not running within %s: %s", wait, status.Phase)
	}

	fmt.Printf("returning from creation : %s", status.Phase)
	return nil
}
