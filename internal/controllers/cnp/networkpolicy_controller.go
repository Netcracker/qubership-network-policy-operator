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

package cnp

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
	"github.com/Netcracker/qubership-network-policy-operator/internal/controllers/npc"
	"github.com/Netcracker/qubership-network-policy-operator/internal/controllers/rclc"
	"github.com/Netcracker/qubership-network-policy-operator/monitoring"
	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	"github.com/projectcalico/libcalico-go/lib/selector"
)

const (
	NetworkPolicyOriginAnnotation = "network-policy.netcracker.com/origin-policy"
	NetworkPolicyTargetLabel      = "network-policy.netcracker.com/target-policy"
	TypeCoupled                   = rclc.TypeCoupled
	NetworkPolicyLabel            = npc.NetworkPolicyLabel
	EgressServiceLabel            = npc.EgressServiceLabel
)

var AllowAnnotation string = npc.AllowAnnotation
var ImplicitNamespaces string = npc.ImplicitNamespaces
var oprNamespace string

// NetworkPolicyReconciler reconciles a NetworkPolicy object
type NetworkPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=projectcalico.org,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projectcalico.org,resources=networkpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projectcalico.org,resources=networkpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=netcracker.com,resources=externalnamespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=netcracker.com,resources=externalnamespaces/status,verbs=get;update;patch

func (r *NetworkPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var accessor metav1.Object
	var svcCFG rclc.ServiceConfig
	var serviceConfigs []rclc.ServiceConfig

	rc := &rclc.RemoteClusterConfigReconciler{}
	oprNamespace := rc.GetOperatorNamespace()

	// Listing the RemoteClusterConfigs
	rclList := &npcv1.RemoteClusterConfigList{}
	opts := &client.ListOptions{Namespace: oprNamespace}
	err := r.List(ctx, rclList, opts)
	if err != nil {
		log.Error(err, "Failed to list RemoteClusterConfigs")
		return ctrl.Result{}, err
	}
	// Checking if there is any RemoteClusterConfig with coupled status
	isEnabled := false
	for _, rclItem := range rclList.Items {
		// Checking if RemoteClusterConfig has valid status
		if !meta.IsStatusConditionTrue(rclItem.Status.Conditions, string(TypeCoupled)) {
			log.Info(fmt.Sprintf("'%s' RemoteClusterConfig is not coupled, so it's skipped", rclItem.GetName()))
		} else {
			isEnabled = true
			svcCFG.Subnet = rclItem.Status.CoupleServiceSubnet
			svcCFG.KubeAPI = rclItem.Status.CoupleKubeAPIService
			svcCFG.CoreDNS = rclItem.Status.CoupleCoreDNSService
			serviceConfigs = append(serviceConfigs, svcCFG)
		}
	}
	// Checking if reconciliation is needed
	if !isEnabled {
		log.Info("NetworkPolicy Reconciler loop has been finished")
		return ctrl.Result{}, nil
	}

	// Getting NetworkPolicy
	policy := &calicov3.NetworkPolicy{}
	err = r.Get(ctx, req.NamespacedName, policy)
	if err != nil {
		log.Info(fmt.Sprintf("Failed to get NetworkPolicy with name: '%s' in '%s' namespace", req.NamespacedName.Name, req.NamespacedName.Namespace))
		return ctrl.Result{}, nil
	}
	// Checking if NetworkPolicy labels is not labeled as target
	accessor, err = meta.Accessor(policy)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get '%s' NetworkPolicy labels in '%s' namespace", policy.GetName(), policy.GetNamespace()))
		return ctrl.Result{}, err
	}
	policyLabels := accessor.GetLabels()
	_, ok := policyLabels[NetworkPolicyTargetLabel]
	if ok {
		log.Info(fmt.Sprintf("'%s' NetworkPolicy in '%s' namespace has target label, it's skipped", policy.GetName(), policy.GetNamespace()))
		return ctrl.Result{}, nil
	}
	// Checking "network-policy.netcracker.com/generated" label
	_, ok = policyLabels[NetworkPolicyLabel]
	if !ok {
		log.Info(fmt.Sprintf("'%s' NetworkPolicy in '%s' namespace doesn't have '%s' label, it's skipped", policy.GetName(), policy.GetNamespace(), NetworkPolicyLabel))
		return ctrl.Result{}, nil
	}
	// Checking "network-policy.netcracker.com/egress-svc" label
	isService := false
	_, ok = policyLabels[EgressServiceLabel]
	if ok {
		log.Info(fmt.Sprintf("'%s' NetworkPolicy in '%s' namespace has '%s' label, it's must be processed as egress NetworkPolicy", policy.GetName(), policy.GetNamespace(), EgressServiceLabel))
		isService = true
	} else {
		log.Info(fmt.Sprintf("'%s' NetworkPolicy in '%s' namespace will be processed as ingress NetworkPolicy", policy.GetName(), policy.GetNamespace()))
	}

	// Setting additional NetworkPolicy name
	extName := ""
	pref := strings.Split(policy.GetName(), ".")[0]
	if (pref != policy.GetName()) && (len(pref) > 0) {
		// Calico v3.29.4 and higher requires NetworkPolicy name preffix that equal to Tier name
		extName = fmt.Sprintf("%s.ens-%s-%s", pref, randomString(10), randomString(5))
	} else {
		extName = fmt.Sprintf("ens-netpol-%s-%s", randomString(10), randomString(5))
	}

	targetPolicy := &calicov3.NetworkPolicy{}
	policyAnn := make(map[string]string)
	labelSet := labels.Set{NetworkPolicyTargetLabel: "true"}
	policyList := &calicov3.NetworkPolicyList{}
	opts = &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSet),
		Namespace:     req.NamespacedName.Namespace,
	}
	err = r.List(ctx, policyList, opts)
	if err != nil {
		log.Error(err, "Faled to list NetworkPolicies")
		return ctrl.Result{}, err
	}
	// Checking if target NetworkPolicy exists
	creation := true
	if err == nil {
		// Searching target NetworkPolicy by origin annotation
		for _, policyItem := range policyList.Items {
			accessor, err = meta.Accessor(&policyItem)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to get '%s' NetworkPolicy annotations in '%s' namespace", policy.GetName(), policy.GetNamespace()))
				return ctrl.Result{}, err
			}
			policyAnn = accessor.GetAnnotations()
			// Checking if NetworkPolicy annotations
			if policyAnn[NetworkPolicyOriginAnnotation] == policy.GetName() {
				log.Info(fmt.Sprintf("'%s' NetworkPolicy has target with name: '%s'", policy.GetName(), policyItem.GetName()))
				creation = false
				targetPolicy = &policyItem
				// Setting target NetworkPolicy spec
				targetPolicy.Spec = policy.Spec
			}
		}
	}

	if creation {
		// The target NetworkPolicy hasn't been found
		// It must be created
		// Setting special label that identifies target NetworkPolicies
		targetPolicyLabels := make(map[string]string)
		targetPolicyLabels[NetworkPolicyTargetLabel] = "true"
		targetPolicy = &calicov3.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extName,
				Namespace: req.NamespacedName.Namespace,
				Labels:    targetPolicyLabels,
			},
			Spec: policy.Spec,
		}
	}
	// Processing Rules
	targetPolicy.Spec.Ingress, targetPolicy.Spec.Egress = r.processRules(policy)
	if isService {
		targetPolicy.Spec.Egress, err = r.processServiceRules(policy, serviceConfigs)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to set Services rules in the '%s' NetworkPolicy in the '%s' namespace", policy.GetName(), policy.GetNamespace()))
			return ctrl.Result{}, err
		}
		// Setting an order for NetworkPolicy
		*targetPolicy.Spec.Order = *policy.Spec.Order - 1.0

	}
	ingChange := len(targetPolicy.Spec.Ingress)
	egChange := len(targetPolicy.Spec.Egress)
	accessor, err = meta.Accessor(targetPolicy)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get '%s' NetworkPolicy annotations and labels in '%s' namespace", policy.GetName(), policy.GetNamespace()))
		return ctrl.Result{}, err
	}
	// Annotate NetworkPolicy with ExternalNamespace
	ensAnnotation := make(map[string]string)
	ensAnnotation[NetworkPolicyOriginAnnotation] = policy.GetName()
	accessor.SetAnnotations(ensAnnotation)

	if (ingChange == 0) && (egChange == 0) && !creation {
		// Delete target NetworkPolicy since origin NetworkPolicy is not working for
		// selectors any more
		log.Info(fmt.Sprintf("Deleting '%s' target NetworkPolicy", targetPolicy.GetName()))
		err = r.Delete(ctx, targetPolicy)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to delete '%s' NetworkPolicy", targetPolicy.GetName()))
			return ctrl.Result{}, err
		}
	}
	if creation && ((ingChange > 0) || (egChange > 0)) {
		// Creating target NetworkPolicy
		log.Info(fmt.Sprintf("Creating '%s' target NetworkPolicy", extName))
		// Set target NetworkPolicy owner
		err = ctrl.SetControllerReference(policy, targetPolicy, r.Scheme)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to set owner of '%s' target NetworkPolicy", extName))
			return ctrl.Result{}, err
		}
		err = r.Create(ctx, targetPolicy)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to create '%s' target NetworkPolicy", extName))
			return ctrl.Result{}, err
		}
	}
	if !creation && ((ingChange > 0) || (egChange > 0)) {
		// Update target NetworkPolicy
		log.Info(fmt.Sprintf("Updating '%s' target NetworkPolicy", targetPolicy.GetName()))
		err = r.Update(ctx, targetPolicy)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' target NetworkPolicy", targetPolicy.GetName()))
			return ctrl.Result{}, err
		}
	}

	log.Info("NetworkPolicy Reconciler loop has been finished")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// The controller must watch:
	// * the ExternalNamespaces to process changes in Pods and Services and Namespaces labels and annotations changes
	// * the NetworkPolicyConfig to process `implicitNamespaces` option changes
	return ctrl.NewControllerManagedBy(mgr).
		For(&calicov3.NetworkPolicy{}).
		Watches(&npcv1.ExternalNamespace{},
			handler.TypedEnqueueRequestsFromMapFunc(r.findNetworkPolicyByExternalNamespace),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&npcv1.NetworkPolicyConfig{},
			handler.TypedEnqueueRequestsFromMapFunc(r.findServiceNetworkPolicies),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// Find Networkpolicy by ExtrenalNamespace name for reconciliation
func (r *NetworkPolicyReconciler) findNetworkPolicyByExternalNamespace(ctx context.Context, ens client.Object) []reconcile.Request {
	log := log.FromContext(ctx)

	// Getting all of NetworkPolicies
	policyList := &calicov3.NetworkPolicyList{}
	opts := &client.ListOptions{}
	err := r.List(context.TODO(), policyList, opts)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(policyList.Items))
	i := 0
	for _, policyItem := range policyList.Items {
		log.Info(fmt.Sprintf("'%s' ExtendedNamespace triggers '%s' NetworkPolicy reconciliation", ens.GetName(), policyItem.GetName()))
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      policyItem.GetName(),
				Namespace: policyItem.GetNamespace(),
			},
		}
		i++
	}

	return requests
}

// Find Service Networkpolicies for reconciliation
func (r *NetworkPolicyReconciler) findServiceNetworkPolicies(ctx context.Context, ens client.Object) []reconcile.Request {
	log := log.FromContext(ctx)

	var accessor metav1.Object
	policyLabels := make(map[string]string)
	// Getting all of NetworkPolicies
	policyList := &calicov3.NetworkPolicyList{}
	opts := &client.ListOptions{}
	err := r.List(context.TODO(), policyList, opts)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(policyList.Items))
	i := 0
	for _, policyItem := range policyList.Items {
		accessor, err = meta.Accessor(&policyItem)
		policyLabels = accessor.GetLabels()
		_, ok := policyLabels[EgressServiceLabel]
		if ok {
			log.Info(fmt.Sprintf("'%s' NetworkPolicyConfig triggers '%s' NetworkPolicy reconciliation", ens.GetName(), policyItem.GetName()))
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      policyItem.GetName(),
					Namespace: policyItem.GetNamespace(),
				},
			}
			i++
		}
	}

	return requests[:i]
}

// processRules returns Calico Rules with IPs instead of selectors
func (r *NetworkPolicyReconciler) processRules(policy *calicov3.NetworkPolicy) ([]calicov3.Rule,
	[]calicov3.Rule) {
	log := log.FromContext(context.TODO())
	var accessor metav1.Object
	var nsSelector, podSelector selector.Selector
	var ingNet, egNet, IPs []string

	ingress := []calicov3.Rule{}
	egress := []calicov3.Rule{}
	nsLabels := make(map[string]string)
	ensList := &npcv1.ExternalNamespaceList{}
	opts := &client.ListOptions{Namespace: oprNamespace}
	err := r.List(context.TODO(), ensList, opts)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get ExternalNamespace for '%s' NetworkPolicy", policy.GetName()))
		return ingress, egress
	}
	monitoring.ExternalNamespacesTotal.Set(float64(len(ensList.Items)))

	// Ingress rules
	ingressRules := policy.Spec.Ingress
	pSource := calicov3.EntityRule{}
	for i := range ingressRules {
		// Checking Selector and NamespaceSelector
		if len(ingressRules[i].Source.Selector) == 0 &&
			len(ingressRules[i].Source.NamespaceSelector) == 0 {
			log.Info(fmt.Sprintf("The Rule %v from %s NetworkPolicy is skipped. There are neither selectors nor namespaceSelectors", i, policy.GetName()))
			continue
		}

		log.Info(fmt.Sprintf("'%s' namespace selector has been found in %v rule of '%s' NetworkPolicy", ingressRules[i].Source.NamespaceSelector, i, policy.GetName()))
		// Getting applicable ExternalNamespaces
		for _, ensItem := range ensList.Items {
			// Skipping deleted ExternalNamespaces
			if ensItem.GetDeletionTimestamp() != nil {
				continue
			}
			accessor, err = meta.Accessor(&ensItem)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to get '%s' ExternalNamespace labels",
					ensItem.GetName()))
				continue
			}
			nsLabels = accessor.GetLabels()
			log.Info(fmt.Sprintf("%s ExternalNamespace labels: %v", ensItem.GetName(), nsLabels))
			nsSelector, err = selector.Parse(ingressRules[i].Source.NamespaceSelector)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to parse '%s' namespace selector in %v rule of '%s' NetworkPolicy", ingressRules[i].Source.NamespaceSelector,
					i, policy.GetName()))
				continue
			}
			// Checking NamespeceSelector
			if len(ingressRules[i].Source.NamespaceSelector) > 0 {
				// The ExternalNamespace must be proccesed if:
				// ExternalNamespace labels and labels in NetworkPolicy namespaceSelector match
				if nsSelector.Evaluate(nsLabels) {
					// Getting applicable pods from ExternalNamespace
					log.Info(fmt.Sprintf("Matching '%s' pod selectors", ingressRules[i].Source.Selector))
					podSelector, err = selector.Parse(ingressRules[i].Source.Selector)
					if err != nil {
						log.Error(err, fmt.Sprintf("Failed to parse '%s' pod selector in %v rule of '%s' NetworkPolicy", ingressRules[i].Source.NamespaceSelector, i, policy.GetName()))
						continue
					}
					// Getting Pods IPs from particular ExternalNamespace
					IPs = r.getIPs(&ensItem.Spec.Resources, podSelector)
					if len(IPs) == 0 {
						continue
					}
					// Appending to common list
					log.Info(fmt.Sprintf("%s ExternalNamespace Pods IPs: %v", ensItem.GetName(), IPs))
					ingNet = append(ingNet, IPs...)
				}
			} else {
				// The ExternalNamespace must be proccesed if:
				// 'kubernetes.io/metadata.name' ExternalNamespace label and NetworkPolicy Namespace is equal
				if nsLabels["kubernetes.io/metadata.name"] == policy.GetNamespace() {
					// Getting applicable pods from ExternalNamespace
					log.Info(fmt.Sprintf("Matching '%s' pod selectors", ingressRules[i].Source.Selector))
					podSelector, err = selector.Parse(ingressRules[i].Source.Selector)
					if err != nil {
						log.Error(err, fmt.Sprintf("Failed to parse '%s' pod selector in %v rule of '%s' NetworkPolicy", ingressRules[i].Source.NamespaceSelector, i, policy.GetName()))
						continue
					}
					// Getting Pods IPs from particular ExternalNamespace
					IPs = r.getIPs(&ensItem.Spec.Resources, podSelector)
					if len(IPs) == 0 {
						continue
					}
					// Appending to common list
					log.Info(fmt.Sprintf("%s ExternalNamespace Pods IPs: %v", ensItem.GetName(), IPs))
					ingNet = append(ingNet, IPs...)
				}
			}
		}
		log.Info(fmt.Sprintf("Rule %v total IPs list: %v", i, ingNet))
		if len(ingNet) > 0 {
			pSource = calicov3.EntityRule{
				Nets: ingNet,
			}
			ingressRules[i].Source = pSource
			ingress = append(ingress, ingressRules[i])
		}
	}

	// Egress rules
	egressRules := policy.Spec.Egress
	pDestination := calicov3.EntityRule{}

	for i := range egressRules {
		// Checking Selector and NamespaceSelector
		if len(egressRules[i].Destination.Selector) == 0 &&
			len(egressRules[i].Destination.NamespaceSelector) == 0 {
			log.Info(fmt.Sprintf("The Rule %v from %s NetworkPolicy is skipped. There are neither selectors nor namespaceSelectors", i, policy.GetName()))
			continue
		}

		log.Info(fmt.Sprintf("'%s' namespace selector has been found in %v rule of '%s' NetworkPolicy", egressRules[i].Destination.NamespaceSelector, i, policy.GetName()))
		// Getting applicable ExternalNamespaces
		for _, ensItem := range ensList.Items {
			// Skipping deleted ExternalNamespaces
			if ensItem.GetDeletionTimestamp() != nil {
				continue
			}
			accessor, err = meta.Accessor(&ensItem)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to get '%s' ExternalNamespace labels",
					ensItem.GetName()))
				continue
			}
			nsLabels = accessor.GetLabels()
			log.Info(fmt.Sprintf("%s ExternalNamespace labels: %v", ensItem.GetName(), nsLabels))
			nsSelector, err = selector.Parse(egressRules[i].Destination.NamespaceSelector)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to parse '%s' namespace selector in %v rule of '%s' NetworkPolicy", egressRules[i].Destination.NamespaceSelector,
					i, policy.GetName()))
				continue
			}
			if nsSelector.Evaluate(nsLabels) {
				// Getting applicable pods from ExternalNamespace
				log.Info(fmt.Sprintf("Matching '%s' pod selectors", egressRules[i].Destination.Selector))
				podSelector, err = selector.Parse(egressRules[i].Destination.Selector)
				if err != nil {
					log.Error(err, fmt.Sprintf("Failed to parse '%s' pod selector in %v rule of '%s' NetworkPolicy", egressRules[i].Destination.NamespaceSelector, i, policy.GetName()))
					continue
				}
				// Getting Pods IPs from particular ExternalNamespace
				IPs = r.getIPs(&ensItem.Spec.Resources, podSelector)
				if len(IPs) == 0 {
					continue
				}
				// Appending to common list
				log.Info(fmt.Sprintf("%s ExternalNamespace Pods IPs: %v", ensItem.GetName(), IPs))
				egNet = append(egNet, IPs...)
			}
		}
		log.Info(fmt.Sprintf("Rule %v total IPs list: %v", i, egNet))
		if len(egNet) > 0 {
			pDestination = calicov3.EntityRule{
				Nets:     egNet,
				Ports:    egressRules[i].Destination.Ports,
				NotPorts: egressRules[i].Destination.NotPorts,
			}
			egressRules[i].Destination = pDestination
			egress = append(egress, egressRules[i])
		}
	}

	return ingress, egress
}

// processServiceRules returns Calico Egress Rules with Services IPs
func (r *NetworkPolicyReconciler) processServiceRules(policy *calicov3.NetworkPolicy,
	servicesConfigs []rclc.ServiceConfig) ([]calicov3.Rule, error) {

	log := log.FromContext(context.TODO())

	var accessor metav1.Object
	var svcIPs, implicitNS, allowedNS, servicesSubnets []string
	var allow string

	rules := []calicov3.Rule{}
	allowRule := calicov3.Rule{}
	denyRule := calicov3.Rule{}
	ensAnn := make(map[string]string)
	ensLabels := make(map[string]string)

	// Getting 'allow annotation' from NetworkPolicyConfig
	npcList := &npcv1.NetworkPolicyConfigList{}
	opts := &client.ListOptions{}
	err := r.List(context.TODO(), npcList, opts)
	if err != nil {
		return nil, err
	}

	// NetworkPolicyConfig must be only one
	if len(npcList.Items) == 0 || len(npcList.Items) > 1 {
		return nil, fmt.Errorf("NetworkPolicyConfig is not single")
	}
	if len(npcList.Items[0].Spec.AllowAnnotation) == 0 {
		allow = AllowAnnotation
	} else {
		allow = npcList.Items[0].Spec.AllowAnnotation
	}

	if len(npcList.Items[0].Spec.ImplicitNamespaces) == 0 {
		implicitNS = strings.Split(ImplicitNamespaces, ",")
	} else {
		implicitNS = strings.Split(npcList.Items[0].Spec.ImplicitNamespaces, ",")
	}

	ensList := &npcv1.ExternalNamespaceList{}
	opts = &client.ListOptions{Namespace: oprNamespace}
	err = r.List(context.TODO(), ensList, opts)
	if err != nil {
		return nil, err
	}
	for _, ensItem := range ensList.Items {
		// Getting ExternalNamespace 'allow annotation'
		accessor, err = meta.Accessor(&ensItem)
		ensAnn = accessor.GetAnnotations()
		ensLabels = accessor.GetLabels()
		_, ok := ensAnn[allow]
		// Getting allowed Namespaces as concatenation of list from annotation and `implicitNamespaces`
		if ok {
			allowedNS = append(implicitNS, strings.Split(ensAnn[allow], ",")...)
		} else {
			allowedNS = implicitNS
		}
		log.Info(fmt.Sprintf("'%s' ExternalNamespace has allowed Namespaces: %v", ensItem.GetName(), allowedNS))
		// Traffic to Services must be allowed if the Services are located in allowed Namespaces
		// or if Namespaces name are equal
		if slices.Contains(allowedNS, policy.GetNamespace()) || ensLabels["kubernetes.io/metadata.name"] == policy.GetNamespace() {
			// Getting Services IPs
			for _, svcSpec := range ensItem.Spec.Resources.Services {
				if len(svcSpec.ClusterIP) > 0 {
					svcIPs = append(svcIPs, fmt.Sprintf("%s/32", svcSpec.ClusterIP))
				}
			}
			log.Info(fmt.Sprintf("%s ExternalNamespace Services IPs: %v", ensItem.GetName(), svcIPs))
		}
	}
	// The rule includes all of the allowed Services
	// The CoreDNS and Kube-API Service are available always
	for _, svcCFG := range servicesConfigs {
		svcIPs = append(svcIPs, fmt.Sprintf("%s/32", svcCFG.KubeAPI), fmt.Sprintf("%s/32", svcCFG.CoreDNS))
		servicesSubnets = append(servicesSubnets, svcCFG.Subnet)
	}

	log.Info(fmt.Sprintf("Overall allowed Services IPs list: %v", svcIPs))
	allowDestination := calicov3.EntityRule{
		Nets: svcIPs,
	}
	log.Info(fmt.Sprintf("Overall denied Services subnets list: %v", servicesSubnets))
	denyDestination := calicov3.EntityRule{
		Nets: servicesSubnets,
	}
	allowRule.Destination = allowDestination
	allowRule.Action = calicov3.Allow
	denyRule.Destination = denyDestination
	denyRule.Action = calicov3.Deny
	rules = append(rules, allowRule, denyRule)

	return rules, nil
}

// Get pods IPs from ExternalNamespace
func (r *NetworkPolicyReconciler) getIPs(res *npcv1.Resources, podSelector selector.Selector) []string {
	log := log.FromContext(context.TODO())
	var net []string
	podSpec := npcv1.Pod{}
	// Get pods specification from ExternalNamespace
	for _, podSpec = range res.Pods {
		if podSelector.Evaluate(podSpec.Labels) {
			log.Info(fmt.Sprintf("The '%s' IP has been matched by pod labels: %v", podSpec.PodIP, podSpec.Labels))
			// Add IP to list
			if len(podSpec.PodIP) > 0 {
				ip := fmt.Sprintf("%s/32", podSpec.PodIP)
				// Only uniq IPs
				if !slices.Contains(net, ip) {
					net = append(net, ip)
				}
			}
		}
	}
	return net
}

func randomString(length int) string {
	var set = []byte("abcdefghijklmnopqrstuvwxyz0123456789")
	str := make([]byte, length)
	for i := range str {
		str[i] = set[rand.Intn(len(set))]
	}
	return string(str)
}
