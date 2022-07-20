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
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	electionID = "ingress-controller-leader"
)

func clusterRoleForNginxIngressController(name string) *rbacv1.ClusterRole {
	rules := []rbacv1.PolicyRule{
		{
			Resources: []string{"namespaces"},
			APIGroups: []string{""},
			Verbs:     []string{"get"},
		},
		{
			Resources: []string{"configmaps", "pods", "secrets", "endpoints", "services"},
			APIGroups: []string{""},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			Resources: []string{"ingresses", "ingressclasses"},
			APIGroups: []string{"networking.k8s.io"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			Resources: []string{"ingresses/status"},
			APIGroups: []string{"networking.k8s.io"},
			Verbs:     []string{"update"},
		},
		{
			Resources:     []string{"configmaps"},
			ResourceNames: []string{electionID},
			APIGroups:     []string{""},
			Verbs:         []string{"get", "update"},
		},
		{
			Resources: []string{"configmaps"},
			APIGroups: []string{""},
			Verbs:     []string{"create"},
		},
		{
			Resources:     []string{"leases"},
			ResourceNames: []string{electionID},
			APIGroups:     []string{"coordination.k8s.io"},
			Verbs:         []string{"get", "update"},
		},
		{
			Resources: []string{"leases"},
			APIGroups: []string{"coordination.k8s.io"},
			Verbs:     []string{"create"},
		},
		{
			Resources: []string{"events"},
			APIGroups: []string{""},
			Verbs:     []string{"create", "patch"},
		},
	}
	rbac := &rbacv1.ClusterRole{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Rules: rules,
	}
	return rbac
}

func subjectForServiceAccount(namespace string, name string) rbacv1.Subject {
	sa := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      name,
		Namespace: namespace,
	}
	return sa
}

func clusterRoleBindingForNginxIngressController(name string) *rbacv1.ClusterRoleBinding {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return crb
}
