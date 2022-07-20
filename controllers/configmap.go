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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"kubegems.io/ingress-nginx-operator/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "k8s.io/api/core/v1"
)

func configMapForNginxIngressController(instance *v1beta1.NginxIngressController, scheme *runtime.Scheme) (*v1.ConfigMap, error) {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Data: instance.Spec.ConfigMapData,
	}
	if err := ctrl.SetControllerReference(instance, cm, scheme); err != nil {
		return nil, err
	}
	return cm, nil
}

func configMapMutateFn(cm *v1.ConfigMap, configMapData map[string]string) controllerutil.MutateFn {
	return func() error {
		cm.Data = configMapData
		return nil
	}
}
