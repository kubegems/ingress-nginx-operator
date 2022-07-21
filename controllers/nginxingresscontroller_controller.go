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

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"kubegems.io/ingress-nginx-operator/api/v1beta1"
	networkingv1beta1 "kubegems.io/ingress-nginx-operator/api/v1beta1"
)

// NginxIngressControllerReconciler reconciles a NginxIngressController object
type NginxIngressControllerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	clusterRoleName = "ingress-nginx-role"
	finalizer       = "nginxingresscontroller.networking.kubegems.io/finalizer"
)

//+kubebuilder:rbac:groups=networking.kubegems.io,resources=nginxingresscontrollers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.kubegems.io,resources=nginxingresscontrollers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.kubegems.io,resources=nginxingresscontrollers/finalizers,verbs=update

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingressclasses;ingresses;ingresses/status,verbs=get;create;delete;list;watch;update
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update
//+kubebuilder:rbac:groups="",resources=services;endpoints;pods;secrets;events;configmaps;serviceaccounts;namespaces,verbs=create;update;get;list;watch;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NginxIngressControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	instance := &networkingv1beta1.NginxIngressController{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil && errors.IsNotFound(err) {
		// Request object not found, could have been deleted after reconcile request.
		// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
		// Return and don't requeue
		log.Info("NginxIngressController resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	} else if err != nil {
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get NginxIngressController")
		return ctrl.Result{}, err
	}

	// Check if the NginxIngressController instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isNginxIngressControllerMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isNginxIngressControllerMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(instance, finalizer) {
			// Run finalization logic for nginxingresscontrollerFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeNginxIngressController(log, instance); err != nil {
				return ctrl.Result{}, err
			}

			// Remove nginxingresscontrollerFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(instance, finalizer)
			err := r.Update(ctx, instance)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(instance, finalizer) {
		controllerutil.AddFinalizer(instance, finalizer)
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Namespace could have been deleted in the middle of the reconcile
	ns := &v1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Namespace, Namespace: v1.NamespaceAll}, ns)
	if (err != nil && errors.IsNotFound(err)) || (ns.Status.Phase == "Terminating") {
		log.Info(fmt.Sprintf("The namespace '%v' does not exist or is in Terminating status, canceling Reconciling", instance.Namespace))
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to check if namespace exists")
		return ctrl.Result{}, err
	}

	if err := addDefaultFields(instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.createCommonResources(log); err != nil {
		return ctrl.Result{}, err
	}

	err = r.checkPrerequisites(log, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// deployment
	found := &appsv1.Deployment{}
	dep, err := deploymentForNginxIngressController(instance, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new Deployment for NGINX Ingress Controller", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)

		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	} else if hasDeploymentChanged(found, instance) {
		log.Info("NginxIngressController spec has changed, updating Deployment")
		updated := updateDeployment(found, instance)
		err = r.Update(ctx, updated)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	svc, err := serviceForNginxIngressController(instance, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}
	var extraLabels map[string]string
	var extraAnnotations map[string]string
	if instance.Spec.Service != nil {
		extraLabels = instance.Spec.Service.ExtraLabels
		extraAnnotations = instance.Spec.Service.ExtraAnnotations
	}
	res, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, serviceMutateFn(svc, instance.Spec.Service.Type, extraLabels, extraAnnotations))
	log.V(1).Info(fmt.Sprintf("Service %s %s", svc.Name, res))
	if err != nil {
		return ctrl.Result{}, err
	}

	cm, err := configMapForNginxIngressController(instance, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}
	res, err = controllerutil.CreateOrUpdate(ctx, r.Client, cm, configMapMutateFn(cm, instance.Spec.ConfigMapData))
	log.V(1).Info(fmt.Sprintf("ConfigMap %s %s", svc.Name, res))
	if err != nil {
		return ctrl.Result{}, err
	}

	if !instance.Status.Deployed {
		instance.Status.Deployed = true
		err := r.Status().Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("Finish reconcile for NginxIngressController")

	return ctrl.Result{}, nil

}

func addDefaultFields(in *v1beta1.NginxIngressController) error {
	if in.Spec.Image.Repository == "" {
		in.Spec.Image.Repository = "registry.k8s.io/ingress-nginx/controller"
	}
	if in.Spec.Image.Tag == "" {
		in.Spec.Image.Tag = "v1.3.0"
	}
	if in.Spec.Image.PullPolicy == "" {
		in.Spec.Image.PullPolicy = v1.PullIfNotPresent
	}
	if !containsStr([]string{"Always", "IfNotPresent", "Never"}, string(in.Spec.Image.PullPolicy)) {
		return fmt.Errorf("image pull policy %s not valid", in.Spec.Image.PullPolicy)
	}

	if in.Spec.Replicas == nil {
		var r int32 = 1
		in.Spec.Replicas = &r
	}

	if in.Spec.Service == nil {
		in.Spec.Service = &networkingv1beta1.Service{}
	}
	if in.Spec.Service.Type == "" {
		in.Spec.Service.Type = "NodePort"
	}
	if !containsStr([]string{"NodePort", "LoadBanlancer"}, in.Spec.Service.Type) {
		return fmt.Errorf("service type %s not valid", in.Spec.Service.Type)
	}

	if in.Spec.Workload == nil {
		in.Spec.Workload = &networkingv1beta1.Workload{}
	}

	if in.Spec.IngressClass == "" {
		in.Spec.IngressClass = "nginx"
	}
	return nil
}

// createIfNotExists creates a new object. If the object exists, does nothing. It returns whether the object existed before or not.
func (r *NginxIngressControllerReconciler) createIfNotExists(object client.Object) (bool, error) {
	err := r.Create(context.TODO(), object)
	if err != nil && errors.IsAlreadyExists(err) {
		return true, nil
	}

	return false, err
}

func (r *NginxIngressControllerReconciler) finalizeNginxIngressController(log logr.Logger, instance *v1beta1.NginxIngressController) error {
	crb := clusterRoleBindingForNginxIngressController(clusterRoleName)

	err := r.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName, Namespace: v1.NamespaceAll}, crb)
	if err != nil {
		return err
	}

	var subjects []rbacv1.Subject
	for _, s := range crb.Subjects {
		if s.Name != instance.Name || s.Namespace != instance.Namespace {
			subjects = append(subjects, s)
		}
	}

	crb.Subjects = subjects

	err = r.Update(context.TODO(), crb)
	if err != nil {
		return err
	}

	log.Info("Successfully finalized NginxIngressController")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NginxIngressControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1beta1.NginxIngressController{}).
		Complete(r)
}
