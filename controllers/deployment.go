/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubegems.io/ingress-nginx-operator/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func deploymentForNginxIngressController(instance *v1beta1.NginxIngressController, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	runAsUser := new(int64)
	allowPrivilegeEscalation := new(bool)
	*runAsUser = 101
	*allowPrivilegeEscalation = true

	dep := &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Spec.Workload.ExtraLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{"app": instance.Name},
			},
			Replicas: instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Name:      instance.Name,
					Namespace: instance.Namespace,
					Labels:    mergeLabels(map[string]string{"app": instance.Name}, instance.Spec.Workload.ExtraLabels),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: instance.Name,
					Containers: []corev1.Container{
						{
							Name:            instance.Name,
							Image:           generateImage(instance.Spec.Image.Repository, instance.Spec.Image.Tag),
							ImagePullPolicy: instance.Spec.Image.PullPolicy,
							Args:            generatePodArgs(instance),
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
								},
								{
									Name:          "https",
									ContainerPort: 443,
								},
								{
									Name:          "metrics",
									ContainerPort: 10254,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
									Add:  []corev1.Capability{"NET_BIND_SERVICE"},
								},
								RunAsUser:                runAsUser,
								AllowPrivilegeEscalation: allowPrivilegeEscalation,
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "LD_PRELOAD", // Enable mimalloc as a drop-in replacement for malloc.
									Value: "/usr/local/lib/libmimalloc.so",
								},
							},
							Resources: instance.Spec.Workload.Resources,
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/wait-shutdown"},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								FailureThreshold: 5,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(10254),
									},
								},
								InitialDelaySeconds: 10,
							},
							ReadinessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(10254),
									},
								},
								InitialDelaySeconds: 10,
							},
						},
					},
				},
			},
		},
	}
	if err := ctrl.SetControllerReference(instance, dep, scheme); err != nil {
		return nil, err
	}
	return dep, nil
}

func hasDeploymentChanged(dep *appsv1.Deployment, instance *v1beta1.NginxIngressController) bool {
	defaultReplicaCount := int32(1)
	if dep.Spec.Replicas != nil && instance.Spec.Replicas == nil && *dep.Spec.Replicas != defaultReplicaCount ||
		dep.Spec.Replicas != nil && instance.Spec.Replicas != nil && *dep.Spec.Replicas != *instance.Spec.Replicas {
		return true
	}

	// There is only 1 container in our template
	container := dep.Spec.Template.Spec.Containers[0]
	if container.Image != generateImage(instance.Spec.Image.Repository, instance.Spec.Image.Tag) {
		return true
	}

	if container.ImagePullPolicy != instance.Spec.Image.PullPolicy {
		return true
	}

	if instance.Spec.Workload == nil {
		instance.Spec.Workload = &v1beta1.Workload{}
	}
	if !reflect.DeepEqual(dep.Labels, instance.Spec.Workload.ExtraLabels) {
		return true
	}

	if HasDifferentResources(container.Resources, instance.Spec.Workload.Resources) {
		return true
	}

	return hasDifferentArguments(container, instance)
}

func updateDeployment(dep *appsv1.Deployment, instance *v1beta1.NginxIngressController) *appsv1.Deployment {
	dep.Spec.Replicas = instance.Spec.Replicas
	if instance.Spec.Replicas == nil {
		defaultReplicaCount := new(int32)
		*defaultReplicaCount = 1
		dep.Spec.Replicas = defaultReplicaCount
	}
	dep.Spec.Template.Spec.Containers[0].Image = generateImage(instance.Spec.Image.Repository, instance.Spec.Image.Tag)
	dep.Spec.Template.Spec.Containers[0].Args = generatePodArgs(instance)
	dep.Spec.Template.Spec.Containers[0].Resources = instance.Spec.Workload.Resources
	dep.Labels = instance.Spec.Workload.ExtraLabels
	dep.Spec.Template.Labels = mergeLabels(map[string]string{"app": instance.Name}, instance.Spec.Workload.ExtraLabels)
	return dep
}
