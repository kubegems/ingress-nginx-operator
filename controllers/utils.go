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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"kubegems.io/ingress-nginx-operator/api/v1beta1"
)

const apiVersionUnsupportedError = "server does not support API version"

// RunningK8sVersion contains the version of k8s
var RunningK8sVersion *version.Version

// generatePodArgs generate a list of arguments for the Ingress Controller pods based on the CRD.
func generatePodArgs(instance *v1beta1.NginxIngressController) []string {
	args := []string{
		"/nginx-ingress-controller",
		fmt.Sprintf("--publish-service=$(POD_NAMESPACE)/%v", instance.Name),
		fmt.Sprintf("--configmap=$(POD_NAMESPACE)/%v", instance.Name),
		fmt.Sprintf("--election-id=%s-lock", instance.Name),
		fmt.Sprintf("--ingress-class=%v", instance.Spec.IngressClass),
		fmt.Sprintf("--controller-class=kubegems.io/ingress-nginx-%v", instance.Spec.IngressClass),
	}

	if instance.Spec.WatchNamespace != "" {
		args = append(args, fmt.Sprintf("-watch-namespace=%v", instance.Spec.WatchNamespace))
	}

	return args
}

// hasDifferentArguments returns whether the arguments of a container are different than the NginxIngressController spec.
func hasDifferentArguments(container corev1.Container, instance *v1beta1.NginxIngressController) bool {
	newArgs := generatePodArgs(instance)
	return !reflect.DeepEqual(newArgs, container.Args)
}

func generateImage(repository string, tag string) string {
	return fmt.Sprintf("%v:%v", repository, tag)
}

func mergeLabels(origin, toadd map[string]string) map[string]string {
	if len(origin) == 0 {
		return toadd
	}
	for k, v := range toadd {
		origin[k] = v
	}
	return origin
}

// only cpu and memory
func HasDifferentResources(origin, newone corev1.ResourceRequirements) bool {
	return !(origin.Requests.Cpu().Equal(newone.Requests.Cpu().DeepCopy()) &&
		origin.Requests.Memory().Equal(newone.Requests.Memory().DeepCopy()) &&
		origin.Limits.Cpu().Equal(newone.Limits.Cpu().DeepCopy()) &&
		origin.Limits.Memory().Equal(newone.Limits.Memory().DeepCopy()))
}

func containsStr(src []string, dest string) bool {
	for _, v := range src {
		if v == dest {
			return true
		}
	}
	return false
}
