package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/config"
	"sigs.k8s.io/kind/pkg/cluster/create"

	"github.com/vimeo/pentagon"
)

const testRootToken = "testroottoken"
const vaultPort = 8200
const k8sNamespace = "default"
const configMapName = "pentagon-config"

type integrationTests struct {
	vaultInstance vault
	pj            pentagonJob
	client        *kubernetes.Clientset
}

func TestIntegration(t *testing.T) {
	// skipping as this needs significant setup...
	t.Skip()

	// see if we're overriding from the outside
	kubeConfigPath := os.Getenv("KIND_KUBE_CONFIG")

	var k8sContext *cluster.Context

	if kubeConfigPath == "" {
		k8sContext = cluster.NewContext("pentagon-test")

		clstr := &config.Cluster{}

		// note that we need to set "WaitForReady" because even though parts
		// of the startup block, kubernetes isn't completely initialized when
		// this method returns.  Adding the WaitForReady ensures that kubernetes
		// is completely ready and other dependencies (like service accounts, for
		// instance) are already available
		err := k8sContext.Create(clstr, create.WaitForReady(5*time.Minute))
		if err != nil {
			t.Fatalf("couldn't create cluster")
		}

		kubeConfigPath = k8sContext.KubeConfigPath()
	}

	configData, err := ioutil.ReadFile(kubeConfigPath)
	if err != nil {
		t.Fatalf("couldn't read configuration: %s", err)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(configData)
	if err != nil {
		t.Fatalf("could not construct rest configuration for k8s: %s", err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("unable to create k8s client: %s", err)
	}

	err = initRBAC(clientset)
	if err != nil {
		t.Fatalf("unable to initialize roles: %s", err)
	}

	// cleans up existing jobs from previous runs so names don't collide
	err = deleteExistingJobs(clientset)
	if err != nil {
		t.Fatalf("unable to delete existing jobs: %s", err)
	}

	vaultInstance := vault{
		client:     clientset,
		restConfig: restConfig,
		namespace:  k8sNamespace,
	}

	pj := pentagonJob{
		client:    clientset,
		namespace: k8sNamespace,
	}

	err = vaultInstance.create((1 * time.Minute))
	if err != nil {
		t.Fatalf("error creating vault pod: %s", err)
	}

	// using a separate test struct so we can share access to the clients...
	it := integrationTests{
		vaultInstance: vaultInstance,
		pj:            pj,
		client:        clientset,
	}

	// now run actual tests
	t.Run("simple", it.testSimple)
	t.Run("reconcile", it.testReconcile)
	t.Run("nonExistentSecret", it.testNonExistentSecret)

	// if we created the cluster, tear it down now...
	if k8sContext != nil {
		k8sContext.Delete()
	}
}

func (it *integrationTests) testSimple(t *testing.T) {
	secretData := map[string]string{
		"john":   "guitar",
		"paul":   "bass",
		"george": "guitar",
		"ringo":  "drums",
	}

	err := it.vaultInstance.setSecret("beatles", secretData)

	if err != nil {
		t.Fatalf("error creating secret: %s", err)
	}

	simpleConfig := it.quickConfig(
		"simpleTest",
		map[string]string{
			"secret/data/beatles": "k8s-beatles",
		},
	)

	err = it.pj.setConfig(simpleConfig)
	if err != nil {
		t.Fatalf("error saving pentagon configuration: %s", err)
	}

	err = it.pj.run(1 * time.Minute)
	if err != nil {
		t.Fatalf("error running pentagon job: %s", err)
	}

	secret, err := it.secret("k8s-beatles")
	if err != nil {
		t.Fatalf("error retrieving secret: %s", err)
	}

	if secret.Labels[pentagon.LabelKey] != "simpleTest" {
		t.Fatalf("label for secret was not simpleTest: %s", secret.Labels[pentagon.LabelKey])
	}

	for k, vaultValue := range secretData {
		if k8sValue, ok := secret.Data[k]; ok {
			if vaultValue != string(k8sValue) {
				t.Fatalf("vaultValue '%s' differs from k8s '%s'", vaultValue, string(k8sValue))
			}
		} else {
			t.Fatalf("vault key '%s' not found in k8s secret!", k)
		}
	}

	// now update the secret in vault, run the sync again and see if it's updated
	secretData["john"] = "vocals"

	err = it.vaultInstance.setSecret("beatles", secretData)
	if err != nil {
		t.Fatalf("error updating secret: %s", err)
	}

	// rerun
	err = it.pj.run(1 * time.Minute)
	if err != nil {
		t.Fatalf("error rerunning pentagon job: %s", err)
	}

	secret, err = it.secret("k8s-beatles")
	if err != nil {
		t.Fatalf("error retrieving secret: %s", err)
	}

	if string(secret.Data["john"]) != "vocals" {
		t.Fatalf("secret was not updated! updated %#v k8s: %#v", secretData, secret.Data)
	}
}

func (it *integrationTests) testReconcile(t *testing.T) {
	// start by making a secret that has a different label so we can verify
	// that it doesn't get deleted later...
	newSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-secret",
			Namespace: k8sNamespace,
			Labels: map[string]string{
				pentagon.LabelKey: "non-matching-value",
			},
		},
		Data: map[string][]byte{
			"blah": []byte{1, 2, 3},
		},
	}

	otherSecret, err := it.client.CoreV1().Secrets(k8sNamespace).Create(newSecret)
	if err != nil {
		t.Fatalf("unable to create other secret: %s", err)
	}

	// cleanup after we're done
	defer func(s *v1.Secret) {
		it.client.CoreV1().Secrets(k8sNamespace).Delete(s.Name, &metav1.DeleteOptions{})
	}(otherSecret)

	secretData1 := map[string]string{
		"foo": "bar",
	}

	err = it.vaultInstance.setSecret("reconcileTest1", secretData1)
	if err != nil {
		t.Fatalf("error creating secret: %s", err)
	}

	secretData2 := map[string]string{
		"blah": "blah",
	}

	err = it.vaultInstance.setSecret("reconcileTest2", secretData2)
	if err != nil {
		t.Fatalf("error creating second secret: %s", err)
	}

	simpleConfig := it.quickConfig(
		"reconcileTest",
		map[string]string{
			"secret/data/reconcileTest1": "k8s-reconcile1",
			"secret/data/reconcileTest2": "k8s-reconcile2",
		},
	)

	err = it.pj.setConfig(simpleConfig)
	if err != nil {
		t.Fatalf("error saving pentagon configuration: %s", err)
	}

	err = it.pj.run(1 * time.Minute)
	if err != nil {
		t.Fatalf("error running pentagon job: %s", err)
	}

	secret, err := it.secret("k8s-reconcile1")
	if err != nil || secret == nil {
		t.Fatalf("error retrieving secret: %s %+v", err, secret)
	}

	secret, err = it.secret("k8s-reconcile2")
	if err != nil || secret == nil {
		t.Fatalf("error retrieving second secret: %s %+v", err, secret)
	}

	// both secrets are there, now update the configuration removing the
	// mapping for the second one...
	simpleConfig = it.quickConfig(
		"reconcileTest",
		map[string]string{
			"secret/data/reconcileTest1": "k8s-reconcile1",
		},
	)

	err = it.pj.setConfig(simpleConfig)
	if err != nil {
		t.Fatalf("error saving updated pentagon configuration: %s", err)
	}

	err = it.pj.run(1 * time.Minute)
	if err != nil {
		t.Fatalf("error re-running pentagon job: %s", err)
	}

	secret, err = it.secret("k8s-reconcile1")
	if err != nil || secret == nil {
		t.Fatalf("error re-retrieving secret: %s %+v", err, secret)
	}

	secret, err = it.secret("k8s-reconcile2")
	if !errors.IsNotFound(err) {
		t.Fatalf("the second secret should no longer exist: %s %+v", err, secret)
	}

	_, err = it.secret("other-secret")
	if err != nil {
		t.Fatalf("the other secret should still be there: %s %+v", err, secret)
	}
}

func (it *integrationTests) testNonExistentSecret(t *testing.T) {
	badConfig := it.quickConfig(
		"nonExistentTest",
		map[string]string{
			"secret/data/not-there": "k8s-somewhere",
		},
	)

	err := it.pj.setConfig(badConfig)
	if err != nil {
		t.Fatalf("error saving pentagon configuration: %s", err)
	}

	err = it.pj.run(1 * time.Minute)
	if err == nil {
		t.Fatal("job should have errored")
	}
}

func (it *integrationTests) secret(name string) (*v1.Secret, error) {
	return it.client.CoreV1().Secrets(k8sNamespace).Get(name, metav1.GetOptions{})
}

func (it *integrationTests) quickConfig(label string, quickMappings map[string]string) *pentagon.Config {
	mappings := []pentagon.Mapping{}
	for k, v := range quickMappings {
		mapping := pentagon.Mapping{
			VaultPath:  k,
			SecretName: v,
		}
		mappings = append(mappings, mapping)
	}

	return &pentagon.Config{
		Vault: pentagon.VaultConfig{
			URL:      it.vaultInstance.url(),
			AuthType: "token",
			Token:    testRootToken,
		},
		Namespace: k8sNamespace,
		Label:     label,
		Mappings:  mappings,
	}
}

func deleteExistingJobs(client *kubernetes.Clientset) error {
	jobsAPI := client.BatchV1().Jobs(k8sNamespace)
	jobs, err := jobsAPI.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing jobs: %s", err)
	}

	for _, job := range jobs.Items {
		err = jobsAPI.Delete(job.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error cleaning up job: %s", err)
		}
	}

	return nil
}

func initRBAC(client *kubernetes.Clientset) error {
	clusterRoles := client.RbacV1().ClusterRoles()
	clusterRoleBindings := client.RbacV1().ClusterRoleBindings()

	// check to see if it exists already (which it might if we're not completely
	// tearing everything down each time).  if it's there, then just get out.
	r, err := clusterRoles.Get("secrets-role", metav1.GetOptions{})
	if err == nil && r != nil {
		return nil
	}

	newRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secrets-role",
			Namespace: k8sNamespace,
		},
		Rules: []rbacv1.PolicyRule{
			rbacv1.PolicyRule{
				Verbs:     []string{"get", "list", "create", "update", "delete"},
				Resources: []string{"secrets"},
				APIGroups: []string{rbacv1.APIGroupAll},
			},
		},
	}

	newBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secrets-role-binding",
			Namespace: k8sNamespace,
		},
		Subjects: []rbacv1.Subject{
			rbacv1.Subject{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "default",
				Namespace: k8sNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     newRole.ObjectMeta.Name,
		},
	}

	_, err = clusterRoles.Create(newRole)
	if err != nil {
		return fmt.Errorf("unable to create role: %s", err)
	}

	_, err = clusterRoleBindings.Create(newBinding)
	if err != nil {
		return fmt.Errorf("unable to create binding: %s", err)
	}

	return nil
}
