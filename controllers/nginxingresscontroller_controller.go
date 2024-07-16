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
	"maps"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubegems.io/ingress-nginx-operator/api/v1beta1"
	networkingv1beta1 "kubegems.io/ingress-nginx-operator/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	log := log.FromContext(ctx).WithValues("nginxingresscontroller", req.NamespacedName)

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
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, serviceMutateFn(svc, instance, r.Scheme)); err != nil {
		log.Error(err, "Failed to create or update Service")
		return ctrl.Result{}, err
	}
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, configMapMutateFn(cm, instance, r.Scheme)); err != nil {
		log.Error(err, "Failed to create or update ConfigMap")
		return ctrl.Result{}, err
	}

	if !instance.Status.Deployed {
		instance.Status.Deployed = true
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}
	log.Info("Reconciliation finished")
	return ctrl.Result{}, nil
}

func configMapMutateFn(cm *v1.ConfigMap, instance *v1beta1.NginxIngressController, scheme *runtime.Scheme) controllerutil.MutateFn {
	return func() error {
		cm.Data = instance.Spec.ConfigMapData
		return ctrl.SetControllerReference(instance, cm, scheme)
	}
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

	ic := ingressClassForNginxIngressController(instance)
	if err := r.Delete(context.TODO(), ic); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
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

func serviceMutateFn(svc *corev1.Service, instance *v1beta1.NginxIngressController, scheme *runtime.Scheme) controllerutil.MutateFn {
	service := instance.Spec.Service
	if service == nil {
		service = &v1beta1.Service{}
	}
	labels := instance.Labels
	annotations := instance.Annotations
	maps.Copy(labels, service.ExtraLabels)
	maps.Copy(annotations, service.ExtraAnnotations)
	selector := map[string]string{"app": instance.Name}
	return func() error {
		svc.Labels = labels
		svc.Annotations = annotations
		svc.Spec.Selector = selector
		svc.Spec.Type = corev1.ServiceType(service.Type)
		svc.Spec.Ports = mergePorts(svc.Spec.Ports, service.Ports)
		return ctrl.SetControllerReference(instance, svc, scheme)
	}
}

func mergePorts(cur, desired []corev1.ServicePort) []corev1.ServicePort {
	ports := map[string]corev1.ServicePort{}
	for _, port := range cur {
		ports[port.Name] = port
	}
	// Ensure that the default ports are present
	if httpport := ports["http"]; httpport.Name == "" {
		ports["http"] = corev1.ServicePort{
			Name: "http", Port: 80, TargetPort: intstr.IntOrString{IntVal: 80},
		}
	}
	if httpsport := ports["https"]; httpsport.Name == "" {
		ports["https"] = corev1.ServicePort{
			Name: "https", Port: 443, TargetPort: intstr.IntOrString{IntVal: 443},
		}
	}
	for _, desired := range desired {
		if port, ok := ports[desired.Name]; ok {
			ports[desired.Name] = mergePort(port, desired)
		} else {
			ports[desired.Name] = desired
		}
	}
	return nil
}

func mergePort(dest, src corev1.ServicePort) corev1.ServicePort {
	if src.Name != "" {
		dest.Name = src.Name
	}
	if src.Protocol != "" {
		dest.Protocol = src.Protocol
	}
	if src.Port != 0 {
		dest.Port = src.Port
	}
	if src.TargetPort.Type != 0 {
		dest.TargetPort = src.TargetPort
	}
	if src.NodePort != 0 {
		dest.NodePort = src.NodePort
	}
	return dest
}
