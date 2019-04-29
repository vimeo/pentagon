package main

import (
	"fmt"
	"time"

	"github.com/vimeo/pentagon"
	yaml "gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type pentagonJob struct {
	client       *kubernetes.Clientset
	namespace    string
	serialNumber int
}

func (p *pentagonJob) setConfig(c *pentagon.Config) error {
	c.SetDefaults()
	configYAML, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("error marshalling yaml config: %s", err)
	}

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: k8sNamespace,
		},
		Data: map[string]string{
			"config.yaml": string(configYAML),
		},
	}

	configMaps := p.client.CoreV1().ConfigMaps(k8sNamespace)

	_, err = configMaps.Get(configMapName, metav1.GetOptions{})
	if err == nil {
		_, err = configMaps.Update(configMap)
		if err != nil {
			return fmt.Errorf("error updating configuration: %s", err)
		}
	} else {
		_, err = configMaps.Create(configMap)
		if err != nil {
			return fmt.Errorf("error creating configuration: %s", err)
		}
	}

	return nil
}

func (p *pentagonJob) run(wait time.Duration) error {
	// name with a serial number so we can differentiate
	jobName := fmt.Sprintf("pentagon-%d", p.serialNumber)

	// increase for next run
	// note this will be problematic if running tests in parallel
	p.serialNumber++

	// only run once -- even if it fails
	backoffLimit := int32(0)

	pentagonJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:  "pentagon",
							Image: "pentagon:0.0.0",
							VolumeMounts: []v1.VolumeMount{
								v1.VolumeMount{
									MountPath: "/pentagon-config",
									Name:      "config-map-volume",
								},
							},
							Args: []string{"/pentagon-config/config.yaml"},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
					Volumes: []v1.Volume{
						v1.Volume{
							Name: "config-map-volume",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	jobs := p.client.BatchV1().Jobs(k8sNamespace)
	created, err := jobs.Create(pentagonJob)
	if err != nil {
		return fmt.Errorf("error creating job: %s", err)
	}

	w, err := p.client.BatchV1().Jobs(p.namespace).Watch(
		metav1.ListOptions{
			Watch:           true,
			ResourceVersion: created.ResourceVersion,
			FieldSelector:   fields.Set{"metadata.name": created.Name}.AsSelector().String(),
			LabelSelector:   labels.Everything().String(),
		},
	)

	if err != nil {
		return fmt.Errorf("error creating watcher: %s", err)
	}

	var status batchv1.JobStatus
	func() {
		for {
			select {
			case events, ok := <-w.ResultChan():
				if !ok {
					return
				}
				resp := events.Object.(*batchv1.Job)

				status = resp.Status
				if status.Active != 1 {
					w.Stop()
				}
			case <-time.After(wait):
				fmt.Println("timeout to wait for job success")
				w.Stop()
			}
		}
	}()

	if status.Succeeded != 1 {
		return fmt.Errorf("pentagon job didn't succeed within %s: %+v", wait, status)
	}

	return nil
}
