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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubegems.io/ingress-nginx-operator/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func serviceForNginxIngressController(instance *v1beta1.NginxIngressController, scheme *runtime.Scheme) (*corev1.Service, error) {
	extraLabels := map[string]string{}
	extraAnnotations := map[string]string{}
	if instance.Spec.Service != nil {
		extraLabels = instance.Spec.Service.ExtraLabels
		extraAnnotations = instance.Spec.Service.ExtraAnnotations
	}

	svc := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Labels:      extraLabels,
			Annotations: extraAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   0,
						IntVal: 80,
					},
				},
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     443,
					TargetPort: intstr.IntOrString{
						Type:   0,
						IntVal: 443,
					},
				},
			},
			Selector: map[string]string{"app": instance.Name},
			Type:     corev1.ServiceType(instance.Spec.Service.Type),
		},
	}

	if err := ctrl.SetControllerReference(instance, svc, scheme); err != nil {
		return nil, err
	}

	return svc, nil
}

func serviceMutateFn(svc *corev1.Service, serviceType string, labels map[string]string, annotations map[string]string) controllerutil.MutateFn {
	return func() error {
		svc.Spec.Type = corev1.ServiceType(serviceType)
		svc.Labels = labels
		svc.Annotations = annotations
		return nil
	}
}
