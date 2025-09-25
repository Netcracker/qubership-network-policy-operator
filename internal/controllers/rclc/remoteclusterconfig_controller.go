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

package rclc

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
	"github.com/Netcracker/qubership-network-policy-operator/monitoring"
)

const (
	ExcludingAnnotation        = "network-policy.qubership.org/exclude-from-external-namespace"
	operatorNamespaceEnvVar    = "NAMESPACE"
	NodeAddressAnnotation      = "projectcalico.org/IPv4Address"
	TunnelAddressAnnotation    = "projectcalico.org/IPv4IPIPTunnelAddr"
	OCPConfigMap               = "cluster-config-v1"
	KubernetesConfigMap        = "kubeadm-config"
	ConfigMapNamespace         = "kube-system"
	OCPCoreDNSName             = "dns-default"
	OCPCoreDNSNamespace        = "openshift-dns"
	KubernetesCoreDNSName      = "kube-dns"
	KubernetesCoreDNSNamespace = "kube-system"
	KubeAPIService             = "kubernetes"
	KubeAPIServiceNamespace    = "default"
	FinalizerName              = "network-policy.qubership.org/finalizer"
)

type RemoteClusterConfigType string

const (
	TypeInitialized  RemoteClusterConfigType = "Initialized"
	TypeCoupled      RemoteClusterConfigType = "Coupled"
	TypeLoose        RemoteClusterConfigType = "Loose"
	TypeSynchronized RemoteClusterConfigType = "Synchronized"
)

var oprNamespace string

var remoteCluster = schema.GroupVersionResource{Group: "qubership.org",
	Version: "v1alpha1", Resource: "remoteclusterconfigs"}

// Structure to store Service configuration
type ServiceConfig struct {
	Subnet  string
	KubeAPI string
	CoreDNS string
}

// RemoteClusterConfigReconciler reconciles a RemoteClusterConfig object
type RemoteClusterConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=qubership.org,resources=remoteclusterconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=qubership.org,resources=remoteclusterconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=qubership.org,resources=remoteclusterconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *RemoteClusterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var extNamespace = schema.GroupVersionResource{Group: "qubership.org",
		Version: "v1alpha1", Resource: "externalnamespaces"}
	var accessor metav1.Object
	var svcCFG ServiceConfig
	var errSt error
	var ensName string

	nsAnn := make(map[string]string)
	unstr := &unstructured.Unstructured{}

	cni, err := r.CheckCNI(r.Client)
	if err != nil {
		log.Error(err, "The CNI cant't be identified")
		return ctrl.Result{}, err
	}
	if cni == "eks" {
		log.Info("The operator is running on the EKS. The DR part is not applicable in that case.")
		return ctrl.Result{}, nil
	}
	oprNamespace = r.GetOperatorNamespace()
	// Checking the RemoteClusterConfig has been created in a valid namespace
	if req.NamespacedName.Namespace != oprNamespace {
		log.Info(fmt.Sprintf("ExternalNamespce operator watches RemoteClusterConfigs resources only in '%s' Namespace", oprNamespace))
		return ctrl.Result{}, nil
	}

	rcl := &npcv1.RemoteClusterConfig{}
	err = r.Get(ctx, req.NamespacedName, rcl)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, fmt.Sprintf("Failed to get '%s' RemoteClusterConfig", req.NamespacedName.Name))
			return ctrl.Result{}, err
		} else {
			log.Info("RemoteClusterConfig Reconciler loop has been finished")
			return ctrl.Result{}, nil
		}
	}
	deletion := false
	// Checking the deletion
	if rcl.GetDeletionTimestamp() != nil {
		deletion = true
	} else {
		if !controllerutil.ContainsFinalizer(rcl, FinalizerName) {
			// Adding finalizer
			log.Info(fmt.Sprintf("Adding finilizer to '%s' RemoteClusterConfig", req.NamespacedName.Name))
			controllerutil.AddFinalizer(rcl, FinalizerName)
			err = r.Update(ctx, rcl)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to add '%s' RemoteClusterConfig finalizer", req.NamespacedName.Name))
				return ctrl.Result{}, err
			}
		}
	}

	// Setting ServiceSubnet
	svcCFG, err = r.getServiceSubnet()
	if err != nil {
		log.Error(err, "Failed identify Service subnet. Egress NetworkPolicies won't be created")
		rcl.Status.ServiceSubnet = ""
		rcl.Status.KubeAPIService = ""
		rcl.Status.CoreDNSService = ""
	} else {
		rcl.Status.ServiceSubnet = svcCFG.Subnet
		rcl.Status.KubeAPIService = svcCFG.KubeAPI
		rcl.Status.CoreDNSService = svcCFG.CoreDNS
		log.Info(fmt.Sprintf("The '%s' RemoteClusterConfig has '%s' as the local Service Subnet, '%s' as the Kubernetes API Service, '%s' as the CoreDNS Service", rcl.GetName(), rcl.Status.ServiceSubnet, rcl.Status.KubeAPIService, rcl.Status.CoreDNSService))
	}
	// Updating RemoteClusterConfig status
	err = r.UpdateStatus(rcl, TypeInitialized, metav1.ConditionUnknown, "Initializing", "Services subnet has been identified", r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Getting credentials to connect to remote cluster
	secret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: rcl.Spec.RemoteCluster.Secret,
		Namespace: oprNamespace}, secret)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get Secret to connect to remote cluster for '%s' RemoteClusterConfig", rcl.GetName()))
		// Updating RemoteClusterConfig status
		errSt = r.UpdateStatus(rcl, TypeLoose, metav1.ConditionFalse, "Initializing", "Couple hasn't been identified", r.Client)
		if errSt != nil {
			return ctrl.Result{}, errSt
		}
		return ctrl.Result{}, err
	}
	// Creating client configuration to connect to remote cluster
	remoteConfig, err := r.GetRemoteConfig(rcl, secret)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to make config to connect to remote cluster"))
		// Updating RemoteClusterConfig status
		errSt = r.UpdateStatus(rcl, TypeLoose, metav1.ConditionFalse, "Initializing", "Couple hasn't been identified", r.Client)
		if errSt != nil {
			return ctrl.Result{}, errSt
		}
		return ctrl.Result{}, err
	}
	dynamicClient, err := dynamic.NewForConfig(remoteConfig)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to create remote client for '%s' RemoteClusterConfig", rcl.GetName()))
		// Updating RemoteClusterConfig status
		errSt = r.UpdateStatus(rcl, TypeLoose, metav1.ConditionFalse, "Initializing", "Couple hasn't been identified", r.Client)
		if errSt != nil {
			return ctrl.Result{}, errSt
		}
		return ctrl.Result{}, err
	}

	// Checking if the RemoteClusterConfig has the correct couple on remote cluster
	rclCouple, err := r.GetCouple(rcl, dynamicClient)
	if err != nil {
		log.Info(fmt.Sprintf("Failed to get couple for '%s' RemoteClusterConfig: %s", rcl.GetName(), err))
		// Updating RemoteClusterConfig status
		errSt = r.UpdateStatus(rcl, TypeLoose, metav1.ConditionFalse, "Initializing", "Couple hasn't been identified", r.Client)
		if errSt != nil {
			return ctrl.Result{}, errSt
		}
		errSt = r.UpdateMetrics(r.Client)
		if errSt != nil {
			return ctrl.Result{}, errSt
		}
		log.Info("RemoteClusterConfig Reconciler loop has been finished")
		return ctrl.Result{}, fmt.Errorf("identification error")
	}
	log.Info(fmt.Sprintf("The couple for '%s' RemoteClusterConfig is '%s'", rcl.GetName(), rclCouple.GetName()))
	// Updating RemoteClusterConfig status
	// Setting couple Services configuration
	rcl.Status.CoupleServiceSubnet = rclCouple.Status.ServiceSubnet
	rcl.Status.CoupleKubeAPIService = rclCouple.Status.KubeAPIService
	rcl.Status.CoupleCoreDNSService = rclCouple.Status.CoreDNSService
	log.Info(fmt.Sprintf("The '%s' RemoteClusterConfig has '%s' as the remote Service Subnet", rcl.GetName(), rcl.Status.CoupleServiceSubnet))
	err = r.UpdateStatus(rcl, TypeCoupled, metav1.ConditionTrue, "Initializing", "Couple is identified", r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Getting the namespaces list
	nsList := &corev1.NamespaceList{}
	opts := []client.ListOption{}
	err = r.List(ctx, nsList, opts...)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get Namespaces for the '%s' RemoteClusterConfig", rcl.GetName()))
		return ctrl.Result{}, err
	}
	tunnelIPs, err := r.NodeToTunnelIPs(r.Client)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get tunnel interface IPs for '%s' RemoteClusterConfig", rcl.GetName()))
		return ctrl.Result{}, err
	}
	// Processing Namespace by Namespace in the following maner:
	// Checking if it's deletion, process it or
	// Getting Pods from Namespace
	// Creating ExternalNamespace description
	// Sending the ExternalNamespace to remote cluster
	labels := make(map[string]string)
	podList := &corev1.PodList{}
	svcList := &corev1.ServiceList{}
	ens := &npcv1.ExternalNamespace{}
	for _, nsItem := range nsList.Items {
		log.Info(fmt.Sprintf("'%s' Namespace will be processed", nsItem.GetName()))
		ensName = r.SetExternalNamespaceName(rcl, &nsItem)
		if deletion {
			// Deleting ExternalNamespace
			log.Info(fmt.Sprintf("'%s' ExternalNamespace deletion", ensName))
			err = dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
				Delete(ctx, ensName, metav1.DeleteOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					log.Error(err, fmt.Sprintf("Failed to delete '%s' ExternalNamespace", ensName))
					return ctrl.Result{}, err
				} else {
					log.Info(fmt.Sprintf("'%s' ExternalNamespace aleady has been deleted", ensName))
					continue
				}
			}
			log.Info(fmt.Sprintf("'%s' ExternalNamespace has been deleted sucessfully", ensName))
			continue
		}
		accessor, err = meta.Accessor(&nsItem)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to get '%s' Namespace annotaion", nsItem.GetName()))
			return ctrl.Result{}, err
		}
		nsAnn = accessor.GetAnnotations()
		_, ok := nsAnn[ExcludingAnnotation]
		if ok {
			log.Info(fmt.Sprintf("'%s' Namespace is skipped due to annotation", nsItem.GetName()))
			continue
		}
		labels = accessor.GetLabels()

		opts := &client.ListOptions{Namespace: nsItem.GetName()}
		err = r.List(ctx, podList, opts)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to list Pods in '%s' Namespace", nsItem.GetName()))
			return ctrl.Result{}, err
		}
		err = r.List(ctx, svcList, opts)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to list Services in '%s' Namespace", nsItem.GetName()))
			return ctrl.Result{}, err
		}
		ens, err = r.ExternalNamespaceForRemoteCluster(podList, svcList, rcl, rclCouple,
			nsItem.GetName(), ensName, labels, tunnelIPs, nsAnn)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to render ExternalNamespace for '%s' Namespace", nsItem.Name))
			return ctrl.Result{}, err
		}
		// Setting ExternalNamespace owner
		err = ctrl.SetControllerReference(rclCouple, ens, r.Scheme)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to set owner of '%s' ExternalNamespace", ens.GetName()))
			return ctrl.Result{}, err
		}
		log.Info(fmt.Sprintf("ExternalNamespace: %v", ens))
		// Converting ExtenalNamespace to Unstructured
		unstr.Object, err = runtime.DefaultUnstructuredConverter.
			ToUnstructured(ens)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to convert '%s' ExternalNamespace to Unstructured", ens.GetName()))
			return ctrl.Result{}, err
		}
		unstructured.SetNestedField(unstr.Object,
			"qubership.org/v1alpha1", "apiVersion")
		unstructured.SetNestedField(unstr.Object,
			"ExternalNamespace", "kind")
		log.Info(fmt.Sprintf("Unstructured: %v", unstr.Object))
		// Creating ExternalNamespace
		log.Info(fmt.Sprintf("'%s' ExternalNamespace creation", ens.GetName()))
		_, err = dynamicClient.Resource(extNamespace).Namespace(oprNamespace).
			Create(ctx, unstr, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, fmt.Sprintf("Failed to create '%s' ExternalNamespace", ens.GetName()))
			return ctrl.Result{}, err
		} else {
			log.Info(fmt.Sprintf("'%s' ExternalNamespace aleady exists, creation skipped", ens.GetName()))
		}
	}

	err = r.UpdateMetrics(r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	if deletion {
		// Removing finalizer
		log.Info(fmt.Sprintf("Removing finilizer from '%s' RemoteClusterConfig", req.NamespacedName.Name))
		controllerutil.RemoveFinalizer(rcl, FinalizerName)
		err = r.Update(ctx, rcl)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to remove '%s' RemoteClusterConfig finalizer", req.NamespacedName.Name))
			return ctrl.Result{}, err
		}
	}
	log.Info("RemoteClusterConfig Reconciler loop has been finished")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

	applicablePredicates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldRcl := e.ObjectOld.(*npcv1.RemoteClusterConfig)
			newRcl := e.ObjectNew.(*npcv1.RemoteClusterConfig)

			// Trigger reconciliation only if the `spec` has changed and `deletionTimestamp` is set
			if newRcl.GetDeletionTimestamp() != nil {
				return true
			}
			if newRcl.GetGeneration() != oldRcl.GetGeneration() {
				return true
			}
			return false
		},

		// Allow create events
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},

		// Reject delete events
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},

		// Reject generic events
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&npcv1.RemoteClusterConfig{}, builder.WithPredicates(applicablePredicates)).
		Complete(r)
}

// ExternalNamespaceForRemoteCluster combine Pods specs into ExternalNamespace
func (r *RemoteClusterConfigReconciler) ExternalNamespaceForRemoteCluster(podList *corev1.PodList,
	svcList *corev1.ServiceList, rcl *npcv1.RemoteClusterConfig, rclCouple *npcv1.RemoteClusterConfig,
	ns string, ensName string, labels map[string]string, tunnelIPs map[string]string,
	ann map[string]string) (*npcv1.ExternalNamespace, error) {

	var podIP string
	pod := &npcv1.Pod{}
	pods := []npcv1.Pod{}
	svc := &npcv1.Service{}
	svcs := []npcv1.Service{}
	for _, podItem := range podList.Items {
		if podItem.GetDeletionTimestamp() != nil {
			continue
		}
		// `hostNetwork` pods IPs must be replaced by tunnel IPs
		if podItem.Status.PodIP == podItem.Status.HostIP {
			_, ok := tunnelIPs[podItem.Status.HostIP]
			if ok {
				podIP = tunnelIPs[podItem.Status.HostIP]
			} else {
				podIP = ""
			}
		} else {
			podIP = podItem.Status.PodIP
		}
		pod.Labels = podItem.GetLabels()
		pod.Name = podItem.GetName()
		pod.NodeName = podItem.Spec.NodeName
		pod.HostIP = podItem.Status.HostIP
		pod.PodIP = podIP
		pods = append(pods, *pod)
	}
	for _, svcItem := range svcList.Items {
		if svcItem.GetDeletionTimestamp() != nil {
			continue
		}
		// Only Services with `clusterIP` are needed
		if len(svcItem.Spec.ClusterIP) > 0 && svcItem.Spec.ClusterIP != "None" {
			svc.Selector = svcItem.Spec.Selector
			svc.Labels = svcItem.GetLabels()
			svc.Name = svcItem.GetName()
			svc.ClusterIP = svcItem.Spec.ClusterIP
			svcs = append(svcs, *svc)
		}
	}
	resourceList := &npcv1.Resources{
		Pods:     pods,
		Services: svcs,
	}
	ens := &npcv1.ExternalNamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ensName,
			Namespace:   rclCouple.GetNamespace(),
			Labels:      labels,
			Annotations: ann,
		},
		Spec: npcv1.ExternalNamespaceSpec{
			RemoteCluster:   rcl.GetName(),
			RemoteNamespace: ns,
			Resources:       *resourceList,
		},
	}
	return ens, nil
}

// GetOperatorNamespace returns a Namespace where the operator is running
func (r *RemoteClusterConfigReconciler) GetOperatorNamespace() string {
	log := log.FromContext(context.TODO())
	var ns string
	ns, found := os.LookupEnv(operatorNamespaceEnvVar)
	if !found {
		log.Info(fmt.Sprintf("Failed to get %s variable, let's set it to \"default\"", operatorNamespaceEnvVar))
		ns = "default"
	}
	return ns
}

// GetCouple returns a RemoteClusterConfig couple from remote cluster
func (r *RemoteClusterConfigReconciler) GetCouple(rcl *npcv1.RemoteClusterConfig,
	dynamicClient *dynamic.DynamicClient) (*npcv1.RemoteClusterConfig, error) {
	rclGet := npcv1.RemoteClusterConfig{}
	rclCouple := npcv1.RemoteClusterConfig{}
	log := log.FromContext(context.TODO())
	// Getting list of RemoteClusterConfig from remote cluster
	resp, err := dynamicClient.Resource(remoteCluster).Namespace(oprNamespace).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Failed to get list of RemoteClusterConfigs")
		return nil, err
	}
	num := 0
	for _, unstrItem := range resp.Items {
		// Converting unstructured to RemoteClusterConfig
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(unstrItem.UnstructuredContent(), &rclGet)
		if err != nil {
			log.Error(err, "Failed to convert Unstructured to RemoteClusterConfigs")
			return nil, err
		}
		// Matching name of remote cluster and name of local cluster
		if rclGet.Spec.RemoteCluster.Name == rcl.Spec.LocalCluster.Name &&
			rclGet.Spec.LocalCluster.Name == rcl.Spec.RemoteCluster.Name {
			num++
			rclCouple = rclGet
		}
	}
	if num == 0 {
		return nil, fmt.Errorf("Failed to get correct couple")
	}
	if num > 1 {
		return nil, fmt.Errorf("There are more than one RemoteClusterConfigs on remote cluster. Can't identify couple")
	}

	return &rclCouple, nil
}

// Getting tunnel interface IPs matched by Nodes IPs
func (r *RemoteClusterConfigReconciler) NodeToTunnelIPs(c client.Client) (map[string]string, error) {
	log := log.FromContext(context.TODO())
	var accessor metav1.Object
	var nodeIP string
	tunnelIPs := make(map[string]string)
	nodeAnn := make(map[string]string)
	// Getting Nodes list
	nodesList := &corev1.NodeList{}
	opts := &client.ListOptions{}
	err := c.List(context.TODO(), nodesList, opts)
	for _, nodeItem := range nodesList.Items {
		// Getting Node annotations
		accessor, err = meta.Accessor(&nodeItem)
		if err != nil {
			return nil, err
		}
		nodeAnn = accessor.GetAnnotations()
		_, ok := nodeAnn[TunnelAddressAnnotation]
		if !ok {
			log.Info(fmt.Sprintf("The '%s' Node hasn't had tunnel interface IP address yet", nodeItem.GetName()))
			continue
		}
		_, ok = nodeAnn[NodeAddressAnnotation]
		if !ok {
			log.Info(fmt.Sprintf("The '%s' Node hasn't had node IP address annotation yet", nodeItem.GetName()))
			continue
		}
		// Adding IPs from annotation into list
		nodeIP = strings.Split(nodeAnn[NodeAddressAnnotation], "/")[0]
		log.Info(fmt.Sprintf("The '%s' Node has host IP address '%s'", nodeItem.GetName(), nodeIP))
		log.Info(fmt.Sprintf("The '%s' Node has tunnel IP interface address '%s'", nodeItem.GetName(), nodeAnn[TunnelAddressAnnotation]))
		tunnelIPs[nodeIP] = nodeAnn[TunnelAddressAnnotation]
	}

	return tunnelIPs, nil
}

// getServiceSubnet extracts the current cluster Service subnet
// Also, it gets KubeAPI and CoreDNS Services IPs
func (r *RemoteClusterConfigReconciler) getServiceSubnet() (ServiceConfig, error) {

	log := log.FromContext(context.TODO())
	var conf, serviceSubnet string
	var svcCFG ServiceConfig
	var data map[string]interface{}
	var netw *net.IPNet
	var ip net.IP

	// Checking OpenShift ConfigMap
	cm := &corev1.ConfigMap{}
	svc := &corev1.Service{}
	// Getting KubeAPI Services IPs
	err := r.Get(context.TODO(), types.NamespacedName{Name: KubeAPIService,
		Namespace: KubeAPIServiceNamespace}, svc)
	if err != nil {
		log.Error(err, "Failed to get Kubernetes API Service")
		return svcCFG, err
	}
	svcCFG.KubeAPI = svc.Spec.ClusterIP
	isFound := true
	err = r.Get(context.TODO(), types.NamespacedName{Name: OCPConfigMap, Namespace: ConfigMapNamespace}, cm)
	if err != nil {
		log.Error(err, "The OCP ConfigMap has not been found")
		isFound = false
	} else {
		conf = cm.Data["install-config"]
		err = yaml.Unmarshal([]byte(conf), &data)
		if err != nil {
			log.Error(err, "Failed to parse YALM in OCP ConfigMap")
		} else {
			_, ok := data["networking"]
			if ok {
				_, ok = data["networking"].(map[string]interface{})["serviceNetwork"]
				if ok {
					serviceSubnet = data["networking"].(map[string]interface{})["serviceNetwork"].([]interface{})[0].(string)
				}
			}
		}
		// Checking if the ServiceSubnet is actual network: kube-api service must be from that subnet
		_, netw, err = net.ParseCIDR(serviceSubnet)
		ip = net.ParseIP(svcCFG.KubeAPI)
		if netw.Contains(ip) {
			svcCFG.Subnet = serviceSubnet
		} else {
			return svcCFG, fmt.Errorf("Service Subnet is defined incorrectly")
		}
		// Getting CoreDNS Services IPs
		err = r.Get(context.TODO(), types.NamespacedName{Name: OCPCoreDNSName,
			Namespace: OCPCoreDNSNamespace}, svc)
		if err != nil {
			log.Error(err, "Failed to get CoreDNS Service")
		}
		svcCFG.CoreDNS = svc.Spec.ClusterIP
		return svcCFG, nil
	}
	// Checking Kubernetes ConfigMap then
	if !isFound {
		err = r.Get(context.TODO(), types.NamespacedName{Name: KubernetesConfigMap, Namespace: ConfigMapNamespace}, cm)
		if err != nil {
			log.Error(err, "The Kubernetes ConfigMap has not been found")
		} else {
			conf = cm.Data["ClusterConfiguration"]
			err = yaml.Unmarshal([]byte(conf), &data)
			if err != nil {
				log.Error(err, "Failed to parse YALM in Kubenretes ConfigMap")
			} else {
				_, ok := data["networking"]
				if ok {
					_, ok = data["networking"].(map[string]interface{})["serviceSubnet"]
					if ok {
						serviceSubnet = data["networking"].(map[string]interface{})["serviceSubnet"].(string)
					}
				}
			}
			_, netw, err = net.ParseCIDR(serviceSubnet)
			ip = net.ParseIP(svcCFG.KubeAPI)
			if netw.Contains(ip) {
				svcCFG.Subnet = serviceSubnet
			} else {
				return svcCFG, fmt.Errorf("Service Subnet is defined incorrectly")
			}
			// Getting CoreDNS Services IPs
			err = r.Get(context.TODO(), types.NamespacedName{Name: KubernetesCoreDNSName,
				Namespace: KubernetesCoreDNSNamespace}, svc)
			if err != nil {
				log.Error(err, "Failed to get CoreDNS Service")
			}
			svcCFG.CoreDNS = svc.Spec.ClusterIP
			return svcCFG, nil
		}
	}
	return svcCFG, err
}

// GetRemoteConfig makes configuration to connect to remote cluster
func (r *RemoteClusterConfigReconciler) GetRemoteConfig(rcl *npcv1.RemoteClusterConfig, secret *corev1.Secret) (*rest.Config, error) {

	log := log.FromContext(context.TODO())
	apiWoPrefix, _ := strings.CutPrefix(rcl.Spec.RemoteCluster.API, "https://")
	// Checking the type of Secret
	remoteConfig := &rest.Config{}
	switch sType := secret.Type; sType {
	// Authenticating by Token
	case "kubernetes.io/service-account-token":
		log.Info(fmt.Sprint("The Sercret has 'kubernetes.io/service-account-token' type. Let's authenticate on remote cluster by token"))
		remoteConfig = &rest.Config{
			Host:        rcl.Spec.RemoteCluster.API,
			APIPath:     "/apis",
			BearerToken: string(secret.Data["token"]),
			TLSClientConfig: rest.TLSClientConfig{
				Insecure:   true,
				ServerName: strings.Split(apiWoPrefix, ":")[0],
			},
		}
	// Authenticating by Certificate
	case "Opaque":
		log.Info(fmt.Sprint("The Sercret has 'Opaque' type. Let's authenticate on remote cluster by certificate"))
		if len(secret.Data["tls.crt"]) == 0 || len(secret.Data["tls.key"]) == 0 || len(secret.Data["ca.crt"]) == 0 {
			err := fmt.Errorf("The Secret is incorrect")
			return nil, err
		}
		remoteConfig = &rest.Config{
			Host:    rcl.Spec.RemoteCluster.API,
			APIPath: "/apis",
			TLSClientConfig: rest.TLSClientConfig{
				Insecure:   false,
				ServerName: strings.Split(apiWoPrefix, ":")[0],
				CertData:   secret.Data["tls.crt"],
				KeyData:    secret.Data["tls.key"],
				CAData:     secret.Data["ca.crt"],
			},
		}
	// Unsupported Secret type
	default:
		err := fmt.Errorf("Unsupported Secret type: '%s'", sType)
		return nil, err
	}
	return remoteConfig, nil
}

// Count number of Loose, Coupled, and Synchronized RemoteClusterConfigs
func (r *RemoteClusterConfigReconciler) UpdateMetrics(c client.Client) error {

	var conditionType string
	log := log.FromContext(context.TODO())
	rclList := &npcv1.RemoteClusterConfigList{}
	opt := &client.ListOptions{Namespace: oprNamespace}
	err := c.List(context.TODO(), rclList, opt)
	if err != nil {
		log.Error(err, "Failed to list RemoteClusterConfigs")
		return err
	}
	rclsInit := 0
	rclsLoose := 0
	rclsCoupled := 0
	rclsSynch := 0
	latestTime := metav1.Unix(0, 0)
	for _, rclItem := range rclList.Items {
		// Checking the latest condition
		for _, conditionItem := range rclItem.Status.Conditions {
			if latestTime.Before(&conditionItem.LastTransitionTime) {
				latestTime = conditionItem.LastTransitionTime
			}
		}
		for _, conditionItem := range rclItem.Status.Conditions {
			if latestTime == conditionItem.LastTransitionTime {
				conditionType = conditionItem.Type
			}
		}
		switch conditionType {
		case string(TypeInitialized):
			rclsInit++
		case string(TypeLoose):
			rclsLoose++
		case string(TypeCoupled):
			rclsCoupled++
		case string(TypeSynchronized):
			rclsSynch++
		}
	}
	monitoring.RemoteClustersConfigInitialized.Set(float64(rclsInit))
	monitoring.RemoteClustersConfigLoose.Set(float64(rclsLoose))
	monitoring.RemoteClustersConfigCoupled.Set(float64(rclsCoupled))
	monitoring.RemoteClustersConfigSynchronized.Set(float64(rclsSynch))

	return nil
}

// Update RemoteClusterConfig status
func (r *RemoteClusterConfigReconciler) UpdateStatus(rcl *npcv1.RemoteClusterConfig, rclType RemoteClusterConfigType, status metav1.ConditionStatus, reason string, message string, c client.Client) error {
	log := log.FromContext(context.TODO())
	// Deleting previous Condition with the same Type
	meta.RemoveStatusCondition(&rcl.Status.Conditions, string(rclType))
	// Setting the new Condition
	meta.SetStatusCondition(&rcl.Status.Conditions, metav1.Condition{Type: string(rclType), Status: status, Reason: reason, Message: message})
	err := c.Status().Update(context.TODO(), rcl)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to update '%s' RemoteClusterConfig status", rcl.GetName()))
		return err
	}

	return nil
}

// Checking if the operator is running on limited Calico configuration by the existance of other CNI pods
func (r *RemoteClusterConfigReconciler) CheckCNI(c client.Client) (string, error) {
	podList := &corev1.PodList{}
	// EKS: finding pods labeled by `k8s-app=aws-node`
	labelSet := labels.Set{"k8s-app": "aws-node"}
	opts := &client.ListOptions{LabelSelector: labels.SelectorFromSet(labelSet)}
	err := c.List(context.TODO(), podList, opts)
	if err != nil {
		return "", err
	}
	if len(podList.Items) > 0 {
		return "eks", nil
	}
	return "", nil

func (r *RemoteClusterConfigReconciler) SetExternalNamespaceName(rcl *npcv1.RemoteClusterConfig, ns *corev1.Namespace) string {
	return fmt.Sprintf("ens-%s-%s", string(rcl.GetUID()), ns.GetUID())
}
