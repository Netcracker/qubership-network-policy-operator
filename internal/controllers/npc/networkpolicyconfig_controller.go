/*
Copyright NetCracker Technology Corporation

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

package npc

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
	"github.com/Netcracker/qubership-network-policy-operator/internal/controllers/rclc"
	"github.com/Netcracker/qubership-network-policy-operator/monitoring"
	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
)

const (
	NetworkPolicyLabel      = "network-policy.qubership.org/generated"
	EgressServiceLabel      = "network-policy.qubership.org/egress-svc"
	TunnelAddressAnnotation = rclc.TunnelAddressAnnotation
	TypeCoupled             = rclc.TypeCoupled
	TypeDefaultApplied      = "DefaultApplied"
	TypeCustomApplied       = "CustomApplied"
	TypeDuplicated          = "Duplicated"
	TypeStarted             = "Started"
	TypeNamespaceUpdated    = "NamespaceUpdated"
)

var KubeAPINamespaces = []string{"kube-system", "openshift-kube-apiserver"}

var ControlPlane string = "kube-,openshift,calico,tigera,ingress-nginx"
var ImplicitNamespaces string = "openshift-monitoring,platform-monitoring,ingress-nginx,openshift-ingress"
var AllowAnnotation string = "network-policy.qubership.org/allowed-from"
var NetworkPolicyLogLabel string = "network-policy.qubership.org/collecting-logs"
var DefaultTier string = "automated"
var CustomTier string = "default"

var LogsEnabled bool
var CollectAllLogs bool

// NetworkPolicyConfigReconciler reconciles a NetworkPolicyConfig object
type NetworkPolicyConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=qubership.org,resources=networkpolicyconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=qubership.org,resources=networkpolicyconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=qubership.org,resources=networkpolicyconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=projectcalico.org,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projectcalico.org,resources=networkpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projectcalico.org,resources=networkpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

func (r *NetworkPolicyConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var templateName string
	var cpNSList, cpPrfx, tunnelIPs []string
	var DefaultTierOrder, CustomTierOrder float64

	npc := &npcv1.NetworkPolicyConfig{}
	err := r.Get(ctx, req.NamespacedName, npc)
	if err != nil {
		log.Error(err, fmt.Sprintf("There is no '%s' NetworkPolicyConfig", req.NamespacedName.Name))
		return ctrl.Result{}, err
	}
	// Checking if NetworkPolicyConfig is deleting
	if npc.GetDeletionTimestamp() != nil {
		log.Info(fmt.Sprintf("NetworkPolicyConfig '%s' is deleting, there is nothing to do", npc.GetName()))
		log.Info("NetworkPolicyConfig Reconciler loop has been finished")
		return ctrl.Result{}, nil
	}
	npcGet := &npcv1.NetworkPolicyConfig{}
	// Removing all previous conditions if the loop is not trigged by the Namespace change
	if len(req.NamespacedName.Namespace) == 0 {
		meta.RemoveStatusCondition(&npc.Status.Conditions, TypeStarted)
		meta.RemoveStatusCondition(&npc.Status.Conditions, TypeDuplicated)
		meta.RemoveStatusCondition(&npc.Status.Conditions, TypeDefaultApplied)
		meta.RemoveStatusCondition(&npc.Status.Conditions, TypeCustomApplied)
		err = r.Get(ctx, req.NamespacedName, npcGet)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to get '%s' NetworkPolicyConfig", req.NamespacedName.Name))
			return ctrl.Result{}, err
		}
		npc.SetResourceVersion(npcGet.GetResourceVersion())
		err = r.Status().Update(ctx, npc)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' NetworkPolicyConfig status", npc.GetName()))
			return ctrl.Result{}, err
		}
		// Setting condition
		meta.SetStatusCondition(&npc.Status.Conditions, metav1.Condition{Type: TypeStarted, Status: metav1.ConditionUnknown, Reason: "Reconciliation", Message: "NetworkPolicyConfig reconciliation has been started"})
		err = r.Status().Update(ctx, npc)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' NetworkPolicyConfig status", npc.GetName()))
			return ctrl.Result{}, err
		}
	}

	// Couning NetworkPolicies
	labelSet := labels.Set{NetworkPolicyLabel: "true"}
	policyList := &calicov3.NetworkPolicyList{}
	npOpts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSet),
	}
	err = r.List(ctx, policyList, npOpts)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to list NetworkPolicies"))
		return ctrl.Result{}, err
	}
	monitoring.NetworkPoliciesTotal.Set(float64(len(policyList.Items)))

	// Checking if there is only one NetworkPolicyConfig in cluster
	npcList := &npcv1.NetworkPolicyConfigList{}
	_ = r.List(ctx, npcList, &client.ListOptions{})
	if len(npcList.Items) > 1 {
		log.Info("The NetworkPolicyConfig must be only one in the cluster")
		monitoring.NetworkPolicyConfigStatus.Set(float64(-1))
		// Setting condition
		meta.SetStatusCondition(&npc.Status.Conditions, metav1.Condition{Type: TypeDuplicated, Status: metav1.ConditionFalse, Reason: "Reconciliation", Message: "NetworkPoliciesConfig duplication"})
		err = r.Status().Update(ctx, npc)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' NetworkPolicyConfig status", npc.GetName()))
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	// Default setings
	for _, templateName := range getDefaultPolicies() {
		if len(npc.Spec.Policies[templateName]) != 0 {
			log.Info(fmt.Sprintf("'%s' NetworkPolicy template: %s", templateName, npc.Spec.Policies[templateName]))
		} else {
			log.Info(fmt.Sprintf("'%s' NetworkPolicy template is empty", templateName))
		}
	}
	if len(npc.Spec.AllowAnnotation) == 0 {
		log.Info(fmt.Sprintf("The '%s' NetworkPolicyConfig doesn't include 'allowAnnotation', let's set it to '%s'", npc.GetName(), AllowAnnotation))
	} else {
		AllowAnnotation = npc.Spec.AllowAnnotation
	}
	if len(npc.Spec.ControlPlane) == 0 {
		log.Info(fmt.Sprintf("The '%s' NetworkPolicyConfig doesn't include 'controlPlane', let's set it to '%s'", npc.GetName(), ControlPlane))
	} else {
		ControlPlane = npc.Spec.ControlPlane
	}
	if len(npc.Spec.ImplicitNamespaces) == 0 {
		log.Info(fmt.Sprintln(fmt.Sprintf("The '%s' NetworkPolicyConfig doesn't include 'implicitNamespaces', let's set it to", npc.GetName()), ImplicitNamespaces))
	} else {
		ImplicitNamespaces = npc.Spec.ImplicitNamespaces
	}
	// Checking logs settings
	if !npc.Spec.Logs.Enabled {
		monitoring.ConnectionsLogStatus.Set(float64(-1))
		LogsEnabled = false
		log.Info(fmt.Sprintf("Audit logs are disabled in the '%s' NetworkPolicyConfig", npc.GetName()))
	} else {
		LogsEnabled = true
		if len(npc.Spec.Logs.NamespaceLabel) > 0 {
			NetworkPolicyLogLabel = npc.Spec.Logs.NamespaceLabel
		} else {
			NetworkPolicyLogLabel = "network-policy.qubership.org/collecting-logs"
			log.Info(fmt.Sprintf("The '%s' NetworkPolicyConfig doesn't include 'namespaceLabel', let's set it to '%s'", npc.GetName(), NetworkPolicyLogLabel))
		}
		if !npc.Spec.Logs.CollectingAll {
			CollectAllLogs = false
			monitoring.ConnectionsLogStatus.Set(float64(1))
		} else {
			CollectAllLogs = true
			monitoring.ConnectionsLogStatus.Set(float64(0))
		}
	}
	// Checking the Tiers availability
	trList := &calicov3.TierList{}
	err = r.List(ctx, trList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "Faled to list Tiers. Let's work without NetworkPolicies Tiers")
		DefaultTier = ""
		CustomTier = ""
	} else {
		log.Info("Tiers has been listed")
		if len(npc.Spec.DefaultTier) > 0 {
			DefaultTier = npc.Spec.DefaultTier
		}
		if len(npc.Spec.CustomTier) > 0 {
			CustomTier = npc.Spec.CustomTier
		}
		log.Info(fmt.Sprintf("The DefaultTier is '%s'; The CustomTier is '%s'", DefaultTier, CustomTier))
		// Checking the default and custom Tiers
		for _, tierItem := range trList.Items {
			if tierItem.GetName() == DefaultTier {
				DefaultTierOrder = *tierItem.Spec.Order
			}
			if tierItem.GetName() == CustomTier {
				CustomTierOrder = *tierItem.Spec.Order
			}
		}
		if DefaultTierOrder == 0 {
			err = fmt.Errorf("Default Tier doesn't exist")
			log.Error(err, "Tiers check Failed")
			return ctrl.Result{}, err
		}
		if CustomTierOrder == 0 {
			err = fmt.Errorf("Custom Tier doesn't exist")
			log.Error(err, "Tiers check Failed")
			return ctrl.Result{}, err
		}
		if DefaultTierOrder >= CustomTierOrder {
			err = fmt.Errorf("Custom Tier Order must be higher than Default Tier Order")
			log.Error(err, "Tiers check Failed")
			return ctrl.Result{}, err
		}
	}

	// Getting slices from string
	for _, item := range strings.Split(ControlPlane, ",") {
		if len(item) > 0 {
			cpPrfx = append(cpPrfx, item)
		}
	}
	// Getting control-plane and operator namespaces
	nsLst := &corev1.NamespaceList{}
	err = r.List(ctx, nsLst, &client.ListOptions{})
	if err != nil {
		log.Error(err, "Faled to list Namespaces")
		return ctrl.Result{}, err
	}
	for _, ns := range nsLst.Items {
		for i := range cpPrfx {
			if strings.HasPrefix(ns.GetName(), cpPrfx[i]) {
				cpNSList = append(cpNSList, ns.GetName())
			}
		}
	}
	log.Info(fmt.Sprintln("Control plane Namespaces:", cpNSList))
	nsList := &corev1.NamespaceList{}
	log.Info(fmt.Sprintf("Requested in Namespace: '%s'", req.NamespacedName.Namespace))
	// Getting Namespaces by labels
	opts := []client.ListOption{
		client.MatchingLabels(npc.Spec.Scope),
	}
	err = r.List(ctx, nsList, opts...)
	if err != nil {
		log.Error(err, "Faled to list Namespaces")
		return ctrl.Result{}, err
	}
	monitoring.NamespacesTotal.Set(float64(len(nsList.Items)))
	if len(nsList.Items) == 0 {
		log.Info(fmt.Sprintf("There are no Namespaces for the '%s' NetworkPolicyConfig", npc.GetName()))
		log.Info("NetworkPolicyConfig Reconciler loop has been finished")
		return ctrl.Result{}, nil
	}
	// Getting tunnel interface IPs
	tunnelIPs, err = r.getTunnelIPs()
	if err != nil {
		log.Error(err, "Failed to get tunnel interfaces IPs")
		return ctrl.Result{}, err
	}
	for _, nsItem := range nsList.Items {
		if npc.Spec.Enabled {
			if len(npc.Spec.Scope) > 0 {
				monitoring.NetworkPolicyConfigStatus.Set(float64(2))
			} else {
				monitoring.NetworkPolicyConfigStatus.Set(float64(1))
			}
			if slices.Contains(cpNSList, nsItem.GetName()) {
				log.Info(fmt.Sprintf("'%s' Namespace is skipped, since it's a part of the control plane, checking log rules", nsItem.GetName()))
				// Processing only request namespace if it's applicable
				if len(req.NamespacedName.Namespace) > 0 &&
					nsItem.GetName() != req.NamespacedName.Namespace {
					continue
				}
				err = r.processNamespace(&nsItem, npc, cpNSList, ImplicitNamespaces, tunnelIPs, true)
				if err != nil {
					return ctrl.Result{}, err
				}
				continue
			}
			// Processing only requested namespace if it's applicable
			if len(req.NamespacedName.Namespace) > 0 &&
				nsItem.GetName() != req.NamespacedName.Namespace {
				continue
			}
			err = r.processNamespace(&nsItem, npc, cpNSList, ImplicitNamespaces, tunnelIPs, false)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			monitoring.NetworkPolicyConfigStatus.Set(float64(0))
			err = r.processDeletion(&nsItem, false)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Setting condition
	err = r.Get(ctx, req.NamespacedName, npcGet)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get '%s' NetworkPolicyConfig", req.NamespacedName.Name))
		return ctrl.Result{}, err
	}
	npc.SetResourceVersion(npcGet.GetResourceVersion())
	if len(req.NamespacedName.Namespace) == 0 {
		meta.SetStatusCondition(&npc.Status.Conditions, metav1.Condition{Type: TypeDefaultApplied, Status: metav1.ConditionTrue, Reason: "Reconciliation", Message: "Default NetworkPolicies have been applied"})
	} else {
		meta.SetStatusCondition(&npc.Status.Conditions, metav1.Condition{Type: TypeNamespaceUpdated, Status: metav1.ConditionTrue, Reason: "Reconciliation", Message: fmt.Sprintf("Default NetworkPolicies have been updated in '%s' Namespace", req.NamespacedName.Namespace)})
	}
	err = r.Status().Update(ctx, npc)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to update '%s' NetworkPolicyConfig status", npc.GetName()))
		return ctrl.Result{}, err
	}

	// Custom NetworkPolicies is not applyed is the loop is triggred by Namespace changes
	templateName = "CustomPolicies"
	if npc.Spec.Enabled && len(npc.Spec.Policies[templateName]) > 0 && len(req.NamespacedName.Namespace) == 0 {
		log.Info(fmt.Sprintf("'%s' NetworkPolicies: %s", templateName, npc.Spec.Policies[templateName]))
		customPolicyList := r.getCustomPolicies(npc.Spec.Policies[templateName])
		for _, policyItem := range customPolicyList.Items {
			err = r.processCustom(&policyItem, npc)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// Setting condition
		meta.SetStatusCondition(&npc.Status.Conditions, metav1.Condition{Type: TypeCustomApplied, Status: metav1.ConditionTrue, Reason: "Reconciliation", Message: "Custom NetworkPolicies have been applied"})
		err = r.Status().Update(ctx, npc)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' NetworkPolicyConfig status", npc.GetName()))
			return ctrl.Result{}, err
		}
	}

	log.Info("NetworkPolicyConfig Reconciler loop has been finished")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkPolicyConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&npcv1.NetworkPolicyConfig{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&calicov3.NetworkPolicy{}).
		Watches(&corev1.Namespace{},
			handler.TypedEnqueueRequestsFromMapFunc(r.patchRequestByNamespace),
			builder.WithPredicates(predicate.Or(predicate.AnnotationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Watches(&corev1.Node{},
			handler.TypedEnqueueRequestsFromMapFunc(r.triggerRequestByNode),
			builder.WithPredicates(predicate.AnnotationChangedPredicate{})).
		Complete(r)
}

func (r *NetworkPolicyConfigReconciler) patchRequestByNamespace(ctx context.Context, ns client.Object) []reconcile.Request {
	// Getting NetworkPolicyConfig
	npcList := &npcv1.NetworkPolicyConfigList{}
	opts := &client.ListOptions{}
	err := r.List(context.TODO(), npcList, opts)
	if err != nil {
		return []reconcile.Request{}
	}

	// NetworkPolicyConfig must be only one
	if len(npcList.Items) == 0 || len(npcList.Items) > 1 {
		return []reconcile.Request{}
	}

	// Putting the namespace name into request to identify it in Reconcile func
	requests := make([]reconcile.Request, 1)
	requests[0] = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      npcList.Items[0].GetName(),
			Namespace: ns.GetName(),
		},
	}

	return requests
}

func (r *NetworkPolicyConfigReconciler) triggerRequestByNode(ctx context.Context, ns client.Object) []reconcile.Request {
	// Getting NetworkPolicyConfig
	npcList := &npcv1.NetworkPolicyConfigList{}
	opts := &client.ListOptions{}
	err := r.List(context.TODO(), npcList, opts)
	if err != nil {
		return []reconcile.Request{}
	}

	// NetworkPolicyConfig must be only one
	if len(npcList.Items) == 0 || len(npcList.Items) > 1 {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 1)
	requests[0] = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      npcList.Items[0].GetName(),
			Namespace: "",
		},
	}

	return requests
}

// decodeNetworkPolicy parse NetworkPolicy
func (r *NetworkPolicyConfigReconciler) decodeNetworkPolicy(data []byte) (*calicov3.NetworkPolicy, error) {
	decode := serializer.NewCodecFactory(r.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	policy := obj.(*calicov3.NetworkPolicy)
	return policy, nil
}

// getCustomPolicies transform YAML notation into NetworkPolicyList
func (r *NetworkPolicyConfigReconciler) getCustomPolicies(rawYAML string) *calicov3.NetworkPolicyList {
	log := log.FromContext(context.TODO())

	// Splitting raw YAML
	list := strings.Split(rawYAML, "---\n")
	policyList := &calicov3.NetworkPolicyList{}
	policies := make([]calicov3.NetworkPolicy, len(list))

	// Decoding each part into NetworkPolicy
	i := 0
	for _, item := range list {
		policy, err := r.decodeNetworkPolicy([]byte(item))
		if err != nil {
			log.Info(fmt.Sprintf("The following couldn't be decoded into NetworkPolicy: '%v'", item))
			continue
		}
		policies[i] = *policy
		i++
	}
	policyList.Items = policies[:i]
	return policyList
}

// setSelector combine namespaceSelector for Networkpolicy
func setSelector(str string) string {
	list := strings.Split(str, ",")
	selector := make([]string, len(list))
	j := 0
	for i := range list {
		if len(list[i]) > 0 {
			selector[j] = fmt.Sprintf("'%s'", list[i])
			j++
		}
	}

	return fmt.Sprintf("kubernetes.io/metadata.name in { %s }", strings.Join(selector[:j], ", "))
}

// processNamespace applies NetworkPolicies in particular Namespace
func (r *NetworkPolicyConfigReconciler) processNamespace(ns *corev1.Namespace, npc *npcv1.NetworkPolicyConfig,
	cpNSList []string, impNS string, tunnelIPs []string, controlPlane bool) error {

	log := log.FromContext(context.TODO())

	var allowNS string
	var accessor metav1.Object
	var ok bool
	var defaultPolicies []string

	policy := &calicov3.NetworkPolicy{}
	getPolicy := &calicov3.NetworkPolicy{}
	getLabels := make(map[string]string)
	nsAnn := make(map[string]string)
	nsLabels := make(map[string]string)

	if ns.GetDeletionTimestamp() != nil {
		log.Info(fmt.Sprintf("'%s' Namespace is being deleting, stop processing it", ns.GetName()))
		return nil
	}
	log.Info(fmt.Sprintf("Creating NetworkPolicies in '%s' Namespace", ns.GetName()))
	// Getting Namespace annotations
	accessor, err := meta.Accessor(ns)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get '%s' Namespace metadata", ns.GetName()))
		return err
	}

	nsAnn = accessor.GetAnnotations()
	_, ok = nsAnn[AllowAnnotation]
	if !ok {
		log.Info(fmt.Sprintf("'%s' Namespace is not explicitly annotated to allow incoming traffic", ns.GetName()))
		// There is no annotation, so only traffic from namespaces that is set implicitly must be allowed
		allowNS = impNS
	} else {
		// Concatenation of namespaces from annotation and implicit namespaces
		allowNS = fmt.Sprintf("%s,%s", nsAnn[AllowAnnotation], impNS)
	}

	// Deleting log rule if logs disabled or it's needed to collect only denied connection
	deleteLog := false
	if !LogsEnabled {
		log.Info(fmt.Sprintf("The NetworkPolicies in '%s' Namespace must not inclde log rules, due to log disabling in NetworkPolicyConfig", ns.GetName()))
		deleteLog = true
	} else {
		// Checking Namespace labels
		// Deleting log rule due to particular Namespace label
		nsLabels = accessor.GetLabels()
		_, ok = nsLabels[NetworkPolicyLogLabel]
		if ok {
			if nsLabels[NetworkPolicyLogLabel] == "false" || nsLabels[NetworkPolicyLogLabel] == "none" {
				log.Info(fmt.Sprintf("The NetworkPolicies in '%s' Namespace must not inclde log rules, due to label", ns.GetName()))
				deleteLog = true
			}
		}
	}

	// Changing default policies list according to the type of Namespace
	if controlPlane {
		defaultPolicies = slices.DeleteFunc(getDefaultPolicies(), func(item string) bool {
			return item != "DefaultLog"
		})
	} else {
		defaultPolicies = slices.DeleteFunc(getDefaultPolicies(), func(item string) bool {
			return item == "DefaultLog"
		})
	}
	// Checking the annotated NetworkPolicies in the Namespace
	// That mesasure prevents creating duplication NetworkPolicies after DefaultTier change
	if len(DefaultTier) != 0 {
		r.processDeletion(ns, true)
	}
	// Appling Kubernetes Default NetworkPolicies
	for _, templateName := range defaultPolicies {
		labels := make(map[string]string)
		// Checking the NetworkPolicy template existence. It prevents crashing
		// if the NetworkPolicyConfig in code and in CR differ
		if len(npc.Spec.Policies[templateName]) == 0 {
			continue
		}
		policy, err = r.decodeNetworkPolicy([]byte(npc.Spec.Policies[templateName]))
		// Setting Namespace
		policy.SetNamespace(ns.GetName())
		// Processing Tiers and changing name
		if len(DefaultTier) != 0 {
			policy.Spec.Tier = DefaultTier
			policy.SetName(fmt.Sprintf("%s.%s", DefaultTier, policy.GetName()))
		}
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to decode '%s' NetworkPolicy template", templateName))
			return err
		}

		//Processing policies
		switch templateName {
		case "DefaultLog":
			// Processing log NetworkPolicy in control-plane Namespace
			if deleteLog || !CollectAllLogs {
				// Deleteting log NetworkPolicy if it exists
				err = r.Get(context.TODO(), types.NamespacedName{Name: policy.GetName(), Namespace: policy.GetNamespace()}, getPolicy)
				if err == nil {
					err = r.Delete(context.TODO(), policy)
					if err != nil {
						log.Error(err, fmt.Sprintf("Failed to delete '%s' NetworkPolicy in '%s' Namespace", templateName, ns.GetName()))
						return err
					}
					monitoring.NetworkPoliciesTotal.Dec()
					log.Info(fmt.Sprintf("'%s' NetworkPolicy has been deleted from '%s' Namespace", policy.GetName(), ns.GetName()))
				}
				continue
			}
		case "AllowFromNamespace":
			// Processing log rules
			if deleteLog || !CollectAllLogs {
				policy.Spec.Ingress = r.deleteRule(policy.Spec.Ingress, calicov3.Log)
				log.Info(fmt.Sprintf("Log rules are deleted from '%s' NetworkPolicy in '%s' Namespace", policy.GetName(), ns.GetName()))
			}
			// Setting Namespace selectors
			for i := range policy.Spec.Ingress {
				if policy.Spec.Ingress[i].Action != calicov3.Log {
					policy.Spec.Ingress[i].Source.NamespaceSelector = setSelector(allowNS)
				}
			}
			log.Info(fmt.Sprintln("Selector:", setSelector(allowNS)))
			// Adding tunnel interface IPs in additional rule
			if len(tunnelIPs) > 0 {
				ingress := []calicov3.Rule{}
				tunnelSource := calicov3.EntityRule{
					Nets: tunnelIPs,
				}
				ingressRule := calicov3.Rule{
					Action: "Allow",
					Source: tunnelSource,
				}
				ingress = policy.Spec.Ingress
				ingress = append(ingress, ingressRule)
				policy.Spec.Ingress = ingress
			}
			log.Info(fmt.Sprintln("NetworkPolicy:", *policy))
		case "DefaultDeny":
			if (templateName == "DefaultDeny") && (deleteLog || CollectAllLogs) {
				policy.Spec.Ingress = r.deleteRule(policy.Spec.Ingress, calicov3.Log)
				log.Info(fmt.Sprintf("Log rules are deleted from '%s' NetworkPolicy in '%s' Namespace", policy.GetName(), ns.GetName()))
			}
		case "AllowWithinNamespace":
		case "RestrictService":
			if deleteLog || !CollectAllLogs {
				policy.Spec.Ingress = r.deleteRule(policy.Spec.Ingress, calicov3.Log)
				log.Info(fmt.Sprintf("Log rules are deleted from '%s' NetworkPolicy in '%s' Namespace", policy.GetName(), ns.GetName()))
			}
			// Setting egress NetworkPolicy label
			labels[EgressServiceLabel] = "true"
		}

		// Getting NetworkPolicy labels
		accessor, err = meta.Accessor(policy)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to get '%s' NetworkPolicy labels", policy.GetName()))
			return err
		}
		getLabels = accessor.GetLabels()
		// Setting NetworkPolicy label
		labels[NetworkPolicyLabel] = "true"
		maps.Copy(labels, getLabels)
		accessor.SetLabels(labels)
		log.Info(fmt.Sprintln("NetworkPolicy:", *policy))
		// Setting NetworkPolicy owner
		err = ctrl.SetControllerReference(npc, policy, r.Scheme)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to set owner of '%s' NetworkPolicy in '%s' Namespace", templateName, ns.GetName()))
			return err
		}
		// Creating or updating NetworkPolicy in Namespace
		err := r.Get(context.TODO(), types.NamespacedName{Name: policy.GetName(), Namespace: policy.GetNamespace()}, getPolicy)
		if err != nil {
			err = r.Create(context.TODO(), policy)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to create '%s' NetworkPolicy in '%s' Namespace", templateName, ns.GetName()))
				return err
			}
			monitoring.NetworkPoliciesTotal.Inc()
			log.Info(fmt.Sprintf("'%s' NetworkPolicy has been created in '%s' Namespace", policy.GetName(), ns.GetName()))
		} else {
			policy.SetResourceVersion(getPolicy.GetResourceVersion())
			err = r.Update(context.TODO(), policy)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to update '%s' NetworkPolicy in '%s' Namespace", templateName, ns.GetName()))
				return err
			}
			log.Info(fmt.Sprintf("'%s' NetworkPolicy has been updated in '%s' Namespace", policy.GetName(), ns.GetName()))
		}
	}

	return nil
}

// processCustom applies Custom NetworkPolicy
func (r *NetworkPolicyConfigReconciler) processCustom(policy *calicov3.NetworkPolicy,
	npc *npcv1.NetworkPolicyConfig) error {

	log := log.FromContext(context.TODO())
	var accessor metav1.Object

	getPolicy := &calicov3.NetworkPolicy{}
	getLabels := make(map[string]string)
	labels := make(map[string]string)

	log.Info(fmt.Sprintf("Process Custom NetworkPolicy: %v", *policy))
	// Setting NetworkPolicy owner
	err := ctrl.SetControllerReference(npc, policy, r.Scheme)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to set owner of '%s' NetworkPolicy in '%s' Namespace", policy.GetName(), policy.GetNamespace()))
		return err
	}
	// Getting NetworkPolicy labels
	accessor, err = meta.Accessor(policy)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get '%s' NetworkPolicy labels", policy.GetName()))
		return err
	}
	getLabels = accessor.GetLabels()
	// Setting NetworkPolicy label
	labels[NetworkPolicyLabel] = "true"
	maps.Copy(labels, getLabels)
	accessor.SetLabels(labels)
	// Processing Tiers and changing name
	if len(CustomTier) != 0 {
		ns := &corev1.Namespace{}
		err = r.Get(context.TODO(), types.NamespacedName{Name: policy.GetNamespace(), Namespace: ""}, ns)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to get '%s' Namespace", policy.GetNamespace()))
			return err
		}
		r.processDeletion(ns, true)
		policy.Spec.Tier = CustomTier
		policy.SetName(fmt.Sprintf("%s.%s", CustomTier, policy.GetName()))
	}
	// Checking if NetworkPolicy must be created or updated
	err = r.Get(context.TODO(), types.NamespacedName{Name: policy.GetName(), Namespace: policy.GetNamespace()}, getPolicy)
	if err != nil {
		// Creating NetworkPolicy
		err = r.Create(context.TODO(), policy)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to create '%s' NetworkPolicy in '%s' Namespace", policy.GetName(), policy.GetNamespace()))
			return err
		}
		monitoring.NetworkPoliciesTotal.Inc()
		log.Info(fmt.Sprintf("'%s' NetworkPolicy in '%s' Namespace has been created", policy.GetName(), policy.GetNamespace()))
	} else {
		// Updating NetworkPolicy
		policy.SetResourceVersion(getPolicy.GetResourceVersion())
		err = r.Update(context.TODO(), policy)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' NetworkPolicy in '%s' Namespace", policy.GetName(), policy.GetName()))
			return err
		}
		log.Info(fmt.Sprintf("'%s' NetworkPolicy in '%s' Namespace has been updated", policy.GetName(), policy.GetNamespace()))
	}

	return nil
}

// processDeletion deletes NetworkPolicies in particular Namespace
func (r *NetworkPolicyConfigReconciler) processDeletion(ns *corev1.Namespace, tierCheck bool) error {

	log := log.FromContext(context.TODO())

	log.Info(fmt.Sprintf("Getting NetworkPolicies in '%s' Namespace for deletion", ns.GetName()))
	// Getting labeled NetworkPolicies in Namespace
	policyList := &calicov3.NetworkPolicyList{}
	opts := []client.ListOption{
		client.InNamespace(ns.GetName()),
		client.MatchingLabels{"network-policy.qubership.org/generated": "true"},
	}
	err := r.List(context.TODO(), policyList, opts...)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to list NetworkPolicy in '%s' Namespace", ns.GetName()))
		return err
	}
	// Deleting NetworkPolicies in Namespace
	for _, policyItem := range policyList.Items {
		// Do not delete the NetworkPolicy with the current DefaultTier or CustomTier if the Tier must be checked
		if tierCheck && (DefaultTier == policyItem.Spec.Tier || CustomTier == policyItem.Spec.Tier) {
			break
		}
		err = r.Delete(context.TODO(), &policyItem)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to delete '%s' NetworkPolicy in '%s' Namespace", policyItem.GetName(), policyItem.GetNamespace()))
			return err
		}
		monitoring.NetworkPoliciesTotal.Dec()
		log.Info(fmt.Sprintf("'%s' NetworkPolicy in '%s' Namespace has been deleted", policyItem.GetName(), policyItem.GetNamespace()))

	}
	return nil
}

// Getting tunnel interface IPs
func (r *NetworkPolicyConfigReconciler) getTunnelIPs() ([]string, error) {
	log := log.FromContext(context.TODO())
	var IPList []string
	var accessor metav1.Object
	nodeAnn := make(map[string]string)
	// Getting Nodes list
	nodesList := &corev1.NodeList{}
	opts := []client.ListOption{}
	err := r.List(context.TODO(), nodesList, opts...)
	if err != nil {
		return nil, err
	}
	for _, nodeItem := range nodesList.Items {
		// Getting Node annotations
		accessor, err = meta.Accessor(&nodeItem)
		if err != nil {
			return nil, err
		}
		nodeAnn = accessor.GetAnnotations()
		_, ok := nodeAnn[TunnelAddressAnnotation]
		if !ok {
			log.Info(fmt.Sprintf("The '%s' node hasn't had tunnel interface address yet", nodeItem.GetName()))
			continue
		}
		// Adding IP in annotation into list
		IPList = append(IPList, fmt.Sprintf("%s/32", nodeAnn[TunnelAddressAnnotation]))
	}

	return IPList, nil
}

// Deleting rules
func (r *NetworkPolicyConfigReconciler) deleteRule(rules []calicov3.Rule, ruleType calicov3.Action) []calicov3.Rule {
	for n, rule := range rules {
		if rule.Action == ruleType {
			rules = slices.Delete(rules, n, n+1)
		}
	}
	return rules
}

// Returns the default policies list
func getDefaultPolicies() []string {
	return []string{"DefaultLog", "AllowFromNamespace", "AllowWithinNamespace", "DefaultDeny", "RestrictService"}
}
