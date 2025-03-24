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

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods/status,verbs=get
//+kubebuilder:rbac:groups="",resources=pods/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var extNamespace = schema.GroupVersionResource{Group: "qubership.org",
		Version: "v1alpha1", Resource: "externalnamespaces"}
	var accessor metav1.Object

	nsAnn := make(map[string]string)
	unstr := &unstructured.Unstructured{}

	rc := &RemoteClusterConfigReconciler{}
	oprNamespace = rc.getOperatorNamespace()

	ns := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: req.NamespacedName.Namespace,
			Namespace: req.NamespacedName.Namespace}, ns)
	if err != nil {
		log.Info("There is no any Namespace")
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

	remoteConfig := &rest.Config{}
	dynamicClient := &dynamic.DynamicClient{}
	secret := &corev1.Secret{}
	podList := &corev1.PodList{}
	svcList := &corev1.ServiceList{}
	ens := &npcv1.ExternalNamespace{}
	ensGet := &npcv1.ExternalNamespace{}
	opts = &client.ListOptions{Namespace: ns.GetName(),}
	err = r.List(ctx, podList, opts)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to list Pods in '%s' Namespace", ns.GetName()))
		return ctrl.Result{}, nil
	}
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
	isSkiped := false
	// Processing RemoteCluster by RemoteCluster in the following maner:
	// Getting Pods from Namespace
	// Applying ExtenalNamespace description to remote cluster
	for _, rclItem := range rclList.Items {
		// Checking if RemoteClusterConfig has valid status
		if !meta.IsStatusConditionTrue(rclItem.Status.Conditions, TypeCoupled) {
			log.Info(fmt.Sprintf("'%s' RemoteClusterConfig is not coupled, so it's skipped", rclItem.GetName()))
			continue
		}
		// Getting credentials to connect to remote cluster
		err = r.Get(ctx, types.NamespacedName{Name: rclItem.Spec.RemoteCluster.Secret,
			Namespace: oprNamespace}, secret)
		if err != nil {
			log.Info(fmt.Sprintf("Secret to connect to '%s' remote cluster does not exist", rclItem.Spec.RemoteCluster.Name))
			isSkiped = true
			continue
		}

		// Create configuration for client to connect to remote cluster
		remoteConfig = &rest.Config{
			Host:        rclItem.Spec.RemoteCluster.API,
			APIPath:     "/apis",
			BearerToken: string(secret.Data["token"]),
			TLSClientConfig: rest.TLSClientConfig{
				Insecure:   true,
				ServerName: strings.Split(rclItem.Spec.RemoteCluster.API, ":")[0],
				//CAData:     []byte(secret.Data["ca.crt"]),
			},
		}
		dynamicClient, err = dynamic.NewForConfig(remoteConfig)
		if err != nil {
			log.Info(fmt.Sprintf("Failed to create remote client for '%s' remote cluster", rclItem.Spec.RemoteCluster.Name))
			isSkiped = true
			continue
		}

		// Checking if the RemoteClusterConfig has the correct couple on remote cluster
		rclCouple, err := rc.getCouple(&rclItem, dynamicClient)
		if err != nil {
			log.Info(fmt.Sprintf("Failed to get couple for '%s' RemoteClusterConfig: %s", rclItem.GetName(), err))
			isSkiped = true
			continue
		}
		log.Info(fmt.Sprintf("The couple for '%s' RemoteClusterConfig is '%s'", rclItem.GetName(), rclCouple.GetName()))

		// Method from other controller
		ens, err = rc.ExternalNamespaceForRemoteCluster(podList, svcList, &rclItem, rclCouple,
				ns.GetName(), string(ns.GetUID()), labels, tunnelIPs, nsAnn)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to render ExternalNamespace for '%s' Namespace", ns.GetName()))
			isSkiped = true
			continue
		}
		// Setting ExternalNamespace owner
		err = ctrl.SetControllerReference(rclCouple, ens, r.Scheme)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to set owner of '%s' ExternalNamespace", ens.GetName()))
			isSkiped = true
			continue
		}
		log.Info(fmt.Sprintf("ExternalNamespace: %v", ens))
		// Convert ExtenalNamespace to Unstructured
		unstr.Object, err = runtime.DefaultUnstructuredConverter.
				ToUnstructured(ens)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to convert '%s' ExternalNamespace to Unstructured", ens.GetName()))
			isSkiped = true
			continue
		}
		unstructured.SetNestedField(unstr.Object, "qubership.org/v1alpha1", "apiVersion")
		unstructured.SetNestedField(unstr.Object, "ExternalNamespace", "kind")
		log.Info(fmt.Sprintf("Unstructured: %v", unstr.Object))
		// Getting ExternalNamespace
		resp, err := dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
			Get(ctx, ens.GetName(), metav1.GetOptions{})
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to get '%s' ExternalNamespace", ens.GetName()))
			isSkiped = true
			continue
		}
		// Updating ExternalNamespace
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(resp.UnstructuredContent(), &ensGet)
		if err != nil {
			log.Error(err, "Failed to convert Unstructured to ExternalNamespace")
			isSkiped = true
			continue
		}
		// Setting ResourceVersion
		unstructured.SetNestedField(unstr.Object, ensGet.GetResourceVersion(), "metadata", "resourceVersion")
		_, err = dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
			Update(ctx, unstr, metav1.UpdateOptions{})
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' ExternalNamespace", ens.GetName()))
			isSkiped = true
			continue
		}
		log.Info(fmt.Sprintf("'%s' ExternalNamespace has been updated", ens.GetName()))

		// Updating RemoteClusterConfig status
		meta.SetStatusCondition(&rclItem.Status.Conditions, metav1.Condition{Type: TypeSynchronized, Status: metav1.ConditionTrue, Reason: "Synchronization", Message: "ExternalNamespaces have been updated"})
		err = r.Status().Update(ctx, &rclItem)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' RemoteClusterConfig status", rclItem.GetName()))
			isSkiped = true
			continue
		}
		// Updating ExternalNamespace
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(resp.UnstructuredContent(), &ensGet)
		if err != nil {
			log.Error(err, "Failed to convert Unstructured to ExternalNamespace")
			isSkiped = true
			continue
		}
		// Setting ResourceVersion
		unstructured.SetNestedField(unstr.Object, ensGet.GetResourceVersion(), "metadata", "resourceVersion")
		_, err = dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
			Update(ctx, unstr, metav1.UpdateOptions{})
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' ExternalNamespace", ens.GetName()))
			isSkiped = true
			continue
		}
		log.Info(fmt.Sprintf("'%s' ExternalNamespace has been updated", ens.GetName()))

		// Updating RemoteClusterConfig status
		meta.SetStatusCondition(&rclItem.Status.Conditions, metav1.Condition{Type: TypeSynchronized, Status: metav1.ConditionTrue, Reason: "Synchronization", Message: "ExternalNamespaces have been updated"})
		err = r.Status().Update(ctx, &rclItem)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' RemoteClusterConfig status", rclItem.GetName()))
			isSkiped = true
			continue
		}
	}

	if isSkiped {
		return ctrl.Result{}, err
	}
	log.Info("Pod Reconciler loop has been finished")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
