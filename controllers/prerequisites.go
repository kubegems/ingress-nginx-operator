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
	"context"
	"fmt"

	"kubegems.io/ingress-nginx-operator/api/v1beta1"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// checkPrerequisites creates all necessary objects before the deployment of a new Ingress Controller.
func (r *NginxIngressControllerReconciler) checkPrerequisites(log logr.Logger, instance *v1beta1.NginxIngressController) error {
	sa, err := serviceAccountForNginxIngressController(instance, r.Scheme)
	if err != nil {
		return err
	}
	existed, err := r.createIfNotExists(sa)
	if err != nil {
		return err
	}

	if !existed {
		log.Info("ServiceAccount created", "ServiceAccount.Namespace", sa.Namespace, "ServiceAccount.Name", sa.Name)
	}

	// Assign this new ServiceAccount to the ClusterRoleBinding (if is not present already)
	crb := clusterRoleBindingForNginxIngressController(clusterRoleName)

	err = r.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName, Namespace: v1.NamespaceAll}, crb)
	if err != nil {
		return err
	}

	subject := subjectForServiceAccount(sa.Namespace, sa.Name)
	found := false
	for _, s := range crb.Subjects {
		if s.Name == subject.Name && s.Namespace == subject.Namespace {
			found = true
			break
		}
	}

	if !found {
		crb.Subjects = append(crb.Subjects, subject)

		err = r.Update(context.TODO(), crb)
		if err != nil {
			return err
		}
	}

	// IngressClass is available from k8s 1.18+
	ic := ingressClassForNginxIngressController(instance)
	existed, err = r.createIfNotExists(ic)
	if err != nil {
		return err
	}

	if !existed {
		log.Info("IngressClass created", "IngressClass.Name", ic.Name)
	}

	return nil
}

// create common resources shared by all the Ingress Controllers
func (r *NginxIngressControllerReconciler) createCommonResources(log logr.Logger) error {
	// Create ClusterRole and ClusterRoleBinding for all the NginxIngressController resources.
	var err error

	cr := clusterRoleForNginxIngressController(clusterRoleName)

	err = r.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName, Namespace: v1.NamespaceAll}, cr)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("no previous ClusterRole found, creating a new one.")
			err = r.Create(context.TODO(), cr)
			if err != nil {
				return fmt.Errorf("error creating ClusterRole: %w", err)
			}
		} else {
			return fmt.Errorf("error getting ClusterRole: %w", err)
		}
	} else {
		// For updates in the ClusterRole permissions (eg new CRDs of the Ingress Controller).
		log.Info("previous ClusterRole found, updating.")
		cr := clusterRoleForNginxIngressController(clusterRoleName)
		err = r.Update(context.TODO(), cr)
		if err != nil {
			return fmt.Errorf("error updating ClusterRole: %w", err)
		}
	}

	crb := clusterRoleBindingForNginxIngressController(clusterRoleName)

	err = r.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName, Namespace: v1.NamespaceAll}, crb)
	if err != nil && errors.IsNotFound(err) {
		log.Info("no previous ClusterRoleBinding found, creating a new one.")
		err = r.Create(context.TODO(), crb)
	}

	if err != nil {
		return fmt.Errorf("error creating ClusterRoleBinding: %w", err)
	}

	return nil
}
