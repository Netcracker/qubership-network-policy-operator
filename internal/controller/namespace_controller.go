/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces/status,verbs=get
//+kubebuilder:rbac:groups="",resources=namespaces/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var accessor metav1.Object

	nsAnn := make(map[string]string)

	rc := &RemoteClusterConfigReconciler{}
	oprNamespace = rc.getOperatorNamespace()

	ns := &corev1.Namespace{}
	err := r.Get(ctx, req.NamespacedName, ns)
	if err != nil {
		log.Info(fmt.Sprintf("There is no '%s' Namespace", req.NamespacedName.Namespace))
		return ctrl.Result{}, nil
	}

	// Checking Namespace annotation
	accessor, err = meta.Accessor(ns)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get '%s' Namespace annotaion", ns.GetName()))
		return ctrl.Result{}, nil
	}
	nsAnn = accessor.GetAnnotations()
	_, ok := nsAnn[ExcludingAnnotation]
	if ok {
		log.Info(fmt.Sprintf("'%s' Namespace is skipped due to annotation", ns.GetName()))
		return ctrl.Result{}, nil
	}
	labels := accessor.GetLabels()

	// Listing the RemoteClusterConfigs
	rclList := &npcv1.RemoteClusterConfigList{}
	opts := &client.ListOptions{Namespace: oprNamespace,}
	err = r.List(ctx, rclList, opts)
	if err != nil {
		log.Error(err, "Failed to list RemoteClusterConfigs")
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("RemoteClusterConfigs list is: %v", rclList))

	podList := &corev1.PodList{}
	svcList := &corev1.ServiceList{}
	opts = &client.ListOptions{Namespace: ns.GetName(),}
	err = r.List(ctx, podList, opts)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to list Pods in '%s' Namespace", ns.GetName()))
		return ctrl.Result{}, nil
	}
	log.Info(fmt.Sprintf("Pods list is: %v", len(podList.Items)))

	err = r.List(ctx, svcList, opts)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to list Services in '%s' Namespace", ns.GetName()))
		return ctrl.Result{}, err
	}
	tunnelIPs, err := rc.nodeToTunnelIPs(r.Client)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get tunnel interface IPs for reconciliation in '%s' Namespace", ns.GetName()))
		return ctrl.Result{}, err
	}
	// Processing RemoteCluster by RemoteCluster in the following maner:
	// Getting Pods and Services from Namespace
	// Applying ExtenalNamespace description to remote cluster
	cErr := make(chan error)
	defer close(cErr)
	for _, rclItem := range rclList.Items {
		go r.processRCL(&rclItem, podList, svcList, ns, labels, tunnelIPs, nsAnn, cErr)
		err, ok := <- cErr
		if ok && err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("Namespace Reconciler loop has been finished")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(&corev1.Pod{},
			handler.TypedEnqueueRequestsFromMapFunc(r.patchRequest),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.Service{},
			handler.TypedEnqueueRequestsFromMapFunc(r.patchRequest),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}

// It makes reconcilation unified for Namespace, Pod, and Service
func (r *NamespaceReconciler) patchRequest(ctx context.Context, res client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 1)
	requests[0] = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      res.GetNamespace(),
			Namespace: res.GetNamespace(),
		},
	}
	return requests
}

// processRCL creates, updates, or deletes ExternalNamespace
func (r *NamespaceReconciler) processRCL(rcl *npcv1.RemoteClusterConfig, podList *corev1.PodList, svcList *corev1.ServiceList, ns *corev1.Namespace, labels map[string]string, tunnelIPs map[string]string, nsAnn map[string]string, cErr chan error) {

	var extNamespace = schema.GroupVersionResource{Group: "qubership.org",
		Version: "v1alpha1", Resource: "externalnamespaces"}

	rc := &RemoteClusterConfigReconciler{}
	log := log.FromContext(context.TODO())
	remoteConfig := &rest.Config{}
	dynamicClient := &dynamic.DynamicClient{}
	secret := &corev1.Secret{}
	ens := &npcv1.ExternalNamespace{}
	ensGet := &npcv1.ExternalNamespace{}
	unstr := &unstructured.Unstructured{}

	// Checking if RemoteClusterConfig has valid status
	log.Info(fmt.Sprintln("RemoteClusterConfigStatus:", rcl.Status))
	if !meta.IsStatusConditionTrue(rcl.Status.Conditions, TypeCoupled) {
		log.Info(fmt.Sprintf("'%s' RemoteClusterConfig is not coupled, so it's skipped", rcl.GetName()))
		cErr <- nil
		return
	}
	// Getting credentials to connect to remote cluster
	err := r.Get(context.TODO(), types.NamespacedName{Name: rcl.Spec.RemoteCluster.Secret,
		Namespace: oprNamespace}, secret)
	if err != nil {
		log.Info(fmt.Sprintf("Secret to connect to '%s' remote cluster does not exist", rcl.Spec.RemoteCluster.Name))
		cErr <- err
		return
	}

	// Create configuration for client to connect to remote cluster
	remoteConfig = &rest.Config{
		Host:        rcl.Spec.RemoteCluster.API,
		APIPath:     "/apis",
		BearerToken: string(secret.Data["token"]),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   true,
			ServerName: strings.Split(rcl.Spec.RemoteCluster.API, ":")[0],
			//CAData:     []byte(secret.Data["ca.crt"]),
		},
	}
	dynamicClient, err = dynamic.NewForConfig(remoteConfig)
	if err != nil {
		log.Info(fmt.Sprintf("Failed to create remote client for '%s' remote cluster", rcl.Spec.RemoteCluster.Name))
		cErr <- err
		return
	}

	// Checking if the RemoteClusterConfig has the correct couple on remote cluster
	rclCouple, err := rc.getCouple(rcl, dynamicClient)
	if err != nil {
		log.Info(fmt.Sprintf("Failed to get couple for '%s' RemoteClusterConfig: %s", rcl.GetName(), err))
		cErr <- err
		return
	}
	log.Info(fmt.Sprintf("The couple for '%s' RemoteClusterConfig is '%s'", rcl.GetName(), rclCouple.GetName()))

	// Method from other controller
	ens, err = rc.ExternalNamespaceForRemoteCluster(podList, svcList, rcl, rclCouple,
			ns.GetName(), string(ns.GetUID()), labels, tunnelIPs, nsAnn)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to render ExternalNamespace for '%s' Namespace", ns.GetName()))
		cErr <- err
		return
	}
	// Setting ExternalNamespace owner
	err = ctrl.SetControllerReference(rclCouple, ens, r.Scheme)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to set owner of '%s' ExternalNamespace", ens.GetName()))
		cErr <- err
		return
	}
	// Setting ExternalNamespace owner
	err = ctrl.SetControllerReference(rclCouple, ens, r.Scheme)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to set owner of '%s' ExternalNamespace", ens.GetName()))
		cErr <- err
		return
	}
	log.Info(fmt.Sprintf("ExternalNamespace: %v", ens))
	// Convert ExtenalNamespace to Unstructured
	unstr.Object, err = runtime.DefaultUnstructuredConverter.
			ToUnstructured(ens)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to convert '%s' ExternalNamespace to Unstructured", ens.GetName()))
		cErr <- err
		return
	}
	unstructured.SetNestedField(unstr.Object, "qubership.org/v1alpha1", "apiVersion")
	unstructured.SetNestedField(unstr.Object, "ExternalNamespace", "kind")
	log.Info(fmt.Sprintf("Unstructured: %v", unstr.Object))
	if ns.GetDeletionTimestamp() == nil {
		// Checking ExternalNamespace
		resp, err := dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
			Get(context.TODO(), ens.GetName(), metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			log.Error(err, fmt.Sprintf("Failed to get '%s' ExternalNamespace", ens.GetName()))
			cErr <- err
			return
		}
		// Creating ExternalNamespace
		if errors.IsNotFound(err) {
			_, err = dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
				Create(context.TODO(), unstr, metav1.CreateOptions{})
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to create '%s' ExternalNamespace", ens.GetName()))
				cErr <- err
				return
			}
			log.Info(fmt.Sprintf("'%s' ExternalNamespace has been created", ens.GetName()))
		// Updating ExternalNamespace
		} else {
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(resp.UnstructuredContent(), &ensGet)
			if err != nil {
				log.Error(err, "Failed to convert Unstructured to ExternalNamespace")
				cErr <- err
				return
			}
			// Setting ResourceVersion
			unstructured.SetNestedField(unstr.Object, ensGet.GetResourceVersion(), "metadata", "resourceVersion")
			_, err = dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
				Update(context.TODO(), unstr, metav1.UpdateOptions{})
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to update '%s' ExternalNamespace", ens.GetName()))
				cErr <- err
				return
			}
			log.Info(fmt.Sprintf("'%s' ExternalNamespace has been updated", ens.GetName()))
		}
	} else {
		// Delete ExternalNamespace
		err = dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
			Delete(context.TODO(), ens.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to delete '%s' ExternalNamespace", ens.GetName()))
			cErr <- err
			return
		}
	}
	// Updating RemoteClusterConfig status
	meta.SetStatusCondition(&rcl.Status.Conditions, metav1.Condition{Type: TypeSynchronized, Status: metav1.ConditionTrue, Reason: "Synchronization", Message: "ExternalNamespaces have been updated"})
	err = r.Status().Update(context.TODO(), rcl)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to update '%s' RemoteClusterConfig status", rcl.GetName()))
		cErr <- err
		return
	}
	cErr <- nil
	return
}
