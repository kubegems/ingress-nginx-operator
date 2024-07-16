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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NginxIngressControllerSpec defines the desired state of NginxIngressController
type NginxIngressControllerSpec struct {
	// The image of the Ingress Controller.
	// +optional
	Image Image `json:"image"`
	// The number of replicas of the Ingress Controller pod. The default is 1. Only applies if the type is set to deployment.
	// +optional
	// +nullable
	Replicas *int32 `json:"replicas"`
	// A class of the Ingress controller. The Ingress controller only processes Ingress resources that belong to its class.
	// +optional
	IngressClass string `json:"ingressClass"`
	// The service of the Ingress controller.
	// +optional
	// +nullable
	Service *Service `json:"service"`
	// The Workload of the Ingress controller.
	// +optional
	// +nullable
	Workload *Workload `json:"workload"`
	// Namespace to watch for Ingress resources. By default the Ingress controller watches all namespaces.
	// +optional
	WatchNamespace string `json:"watchNamespace"`
	// Initial values of the Ingress Controller ConfigMap.
	// Check https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/ for
	// more information about possible values.
	// +optional
	// +nullable
	ConfigMapData map[string]string `json:"configMapData,omitempty"`
}

// Image defines the Repository, Tag and ImagePullPolicy of the Ingress Controller Image.
type Image struct {
	// The repository of the image.
	// +optional
	Repository string `json:"repository"`
	// The tag (version) of the image.
	// +optional
	Tag string `json:"tag"`
	// The ImagePullPolicy of the image.
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy"`
}

// Service defines the Service for the Ingress Controller.
type Service struct {
	// The type of the Service for the Ingress Controller. Valid Service types are: NodePort and LoadBalancer.
	// +optional
	Type string `json:"type"`
	// Specifies extra labels of the service.
	// +optional
	// +nullable
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`
	// Specifies extra annotations of the service.
	// +optional
	// +nullable
	ExtraAnnotations map[string]string `json:"extraAnnotations,omitempty"`

	// Ports of the Service.
	// +optional
	Ports []corev1.ServicePort `json:"ports"`
}

// Workload of the Ingress controller.
type Workload struct {
	// Specifies resource request and limit of the nginx container
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Specifies extra labels of the workload(deployment or daemonset) of nginx.
	// +optional
	// +nullable
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`
}

// Metrics defines the Metrics metrics for the Ingress Controller.
type Metrics struct {
	// Enable Prometheus metrics.
	Enable bool `json:"enable"`
	// Sets the port where the Prometheus metrics are exposed. Default is 10254.
	// Format is 1023 - 65535
	// +kubebuilder:validation:Minimum=1023
	// +kubebuilder:validation:Maximum=65535
	// +optional
	// +nullable
	Port *uint16 `json:"port"`
}

// NginxIngressControllerStatus defines the observed state of NginxIngressController
type NginxIngressControllerStatus struct {
	// Deployed is true if the Operator has finished the deployment of the NginxIngressController.
	Deployed bool `json:"deployed"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NginxIngressController is the Schema for the nginxingresscontrollers API
type NginxIngressController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxIngressControllerSpec   `json:"spec,omitempty"`
	Status NginxIngressControllerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NginxIngressControllerList contains a list of NginxIngressController
type NginxIngressControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxIngressController `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NginxIngressController{}, &NginxIngressControllerList{})
}
