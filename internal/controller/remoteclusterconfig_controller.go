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
	"os"
	"strings"
	"gopkg.in/yaml.v3"
	"net"

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

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
)

const (
	ExcludingAnnotation	   = "network-policy.qubership.org/exclude-from-external-namespace"
	operatorNamespaceEnvVar    = "NAMESPACE"
	TypeCoupled		   = "Coupled"
	TypeLoose		   = "Loose"
	TypeSynchronized	   = "Synchronized"
	NodeAddressAnnotation	   = "projectcalico.org/IPv4Address"
	OCPConfigMap		   = "cluster-config-v1"
	KubernetesConfigMap	   = "kubeadm-config"
	ConfigMapNamespace	   = "kube-system"
	OCPCoreDNSName		   = "dns-default"
	OCPCoreDNSNamespace	   = "openshift-dns"
	KubernetesCoreDNSName	   = "kube-dns"
	KubernetesCoreDNSNamespace = "kube-system"
	KubeAPIService		   = "kubernetes"
	KubeAPIServiceNamespace	   = "default"
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

	nsAnn := make(map[string]string)
	unstr := &unstructured.Unstructured{}

	oprNamespace = r.getOperatorNamespace()
	// Checking the RemoteClusterConfig has been created in a valid namespace
	if req.NamespacedName.Namespace != oprNamespace {
		log.Info(fmt.Sprintf("ExternalNamespce operator watches RemoteClusterConfigs resources only in '%s' Namespace", oprNamespace))
		return ctrl.Result{}, nil
	}

	rcl := &npcv1.RemoteClusterConfig{}
	err := r.Get(ctx, req.NamespacedName, rcl)
	if err != nil {
		log.Error(err, fmt.Sprintf("There is no '%s' RemoteClusterConfig", req.NamespacedName.Name))
		return ctrl.Result{}, err
	}

	// Getting credentials to connect to remote cluster
	secret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: rcl.Spec.RemoteCluster.Secret,
		Namespace: oprNamespace}, secret)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get Secret to connect to remote cluster for '%s' RemoteClusterConfig", rcl.GetName()))
		return ctrl.Result{}, err
	}
	// Create configuration for client to connect to remote cluster
	remoteConfig := &rest.Config{
		Host:        rcl.Spec.RemoteCluster.API,
		APIPath:     "/apis",
		BearerToken: string(secret.Data["token"]),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   true,
			ServerName: strings.Split(rcl.Spec.RemoteCluster.API, ":")[0],
			//CAData:     []byte(secret.Data["ca.crt"]),
		},
	}
	dynamicClient, err := dynamic.NewForConfig(remoteConfig)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to create remote client for '%s' RemoteClusterConfig", rcl.GetName()))
		return ctrl.Result{}, err
	}

	// Setting ServiceSubnet
	svcCFG, err = r.getServiceSubnet()
	if err != nil {
		log.Error(err, "Failed identify Service subnet. Egress NetworkPolicies won't be created")
		rcl.Status.ServiceSubnet  = ""
		rcl.Status.KubeAPIService = ""
		rcl.Status.CoreDNSService = ""
	} else {
		rcl.Status.ServiceSubnet  = svcCFG.Subnet
		rcl.Status.KubeAPIService = svcCFG.KubeAPI
		rcl.Status.CoreDNSService = svcCFG.CoreDNS
		log.Info(fmt.Sprintf("The '%s' RemoteClusterConfig has '%s' as the local Service Subnet, '%s' as the Kubernetes API Service, '%s' as the CoreDNS Service", rcl.GetName(), rcl.Status.ServiceSubnet, rcl.Status.KubeAPIService, rcl.Status.CoreDNSService))
	}

	// Checking if the RemoteClusterConfig has the correct couple on remote cluster
	rclCouple, err := r.getCouple(rcl, dynamicClient)
	if err != nil {
		log.Info(fmt.Sprintf("Failed to get couple for '%s' RemoteClusterConfig: %s", rcl.GetName(), err))
		// Updating RemoteClusterConfig status
		meta.SetStatusCondition(&rcl.Status.Conditions, metav1.Condition{Type: TypeLoose, Status: metav1.ConditionFalse, Reason: "Initializing", Message: "Couple hasn't been identified"})
		err = r.Status().Update(ctx, rcl)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update '%s' RemoteClusterConfig status", rcl.GetName()))
			return ctrl.Result{}, err
		}
		log.Info("RemoteClusterConfig Reconciler loop has been finished")
		return ctrl.Result{}, fmt.Errorf("identification error")
	}
	log.Info(fmt.Sprintf("The couple for '%s' RemoteClusterConfig is '%s'", rcl.GetName(), rclCouple.GetName()))
	// Updating RemoteClusterConfig status
	meta.SetStatusCondition(&rcl.Status.Conditions, metav1.Condition{Type: TypeCoupled, Status: metav1.ConditionTrue, Reason: "Initializing", Message: "Couple is identified"})
	// Setting couple Services configuration
	rcl.Status.CoupleServiceSubnet  = rclCouple.Status.ServiceSubnet
	rcl.Status.CoupleKubeAPIService = rclCouple.Status.KubeAPIService
	rcl.Status.CoupleCoreDNSService = rclCouple.Status.CoreDNSService
	log.Info(fmt.Sprintf("The '%s' RemoteClusterConfig has '%s' as the remote Service Subnet", rcl.GetName(), rcl.Status.CoupleServiceSubnet))
	err = r.Status().Update(ctx, rcl)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to update '%s' RemoteClusterConfig status", rcl.GetName()))
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
	tunnelIPs, err := r.nodeToTunnelIPs(r.Client)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed to get tunnel interface IPs for '%s' RemoteClusterConfig", rcl.GetName()))
		return ctrl.Result{}, err
	}
	// Processing Namespace by Namespace in the following maner:
	// Getting Pods from Namespace
	// Creating ExternalNamespace description
	// Sending the ExternalNamespace to remote cluster
	labels := make(map[string]string)
	podList := &corev1.PodList{}
	svcList := &corev1.ServiceList{}
	ens := &npcv1.ExternalNamespace{}
	for _, nsItem := range nsList.Items {
		log.Info(fmt.Sprintf("'%s' Namespace will be processed", nsItem.GetName()))
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

		opts := &client.ListOptions{Namespace: nsItem.GetName(),}
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
								nsItem.GetName(), string(nsItem.GetUID()),
								labels, tunnelIPs, nsAnn)
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
		// Create ExternalNamespace
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

	log.Info("RemoteClusterConfig Reconciler loop has been finished")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&npcv1.RemoteClusterConfig{}).
		Complete(r)
}

// ExternalNamespaceForRemoteCluster combine Pods specs into ExternalNamespace
func (r *RemoteClusterConfigReconciler) ExternalNamespaceForRemoteCluster(podList *corev1.PodList,
	svcList *corev1.ServiceList, rcl *npcv1.RemoteClusterConfig, rclCouple *npcv1.RemoteClusterConfig,
	ns string, uid string, labels map[string]string, tunnelIPs map[string]string,
	ann map[string]string) (*npcv1.ExternalNamespace, error) {

	ensName := fmt.Sprintf("ens-%s-%s", string(rcl.GetUID()), uid)
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
			podIP    = podItem.Status.PodIP
		}
		pod.Labels   = podItem.GetLabels()
		pod.Name     = podItem.GetName()
		pod.NodeName = podItem.Spec.NodeName
		pod.HostIP   = podItem.Status.HostIP
		pod.PodIP    = podIP
		pods	     = append(pods, *pod)
	}
	for _, svcItem := range svcList.Items {
		if svcItem.GetDeletionTimestamp() != nil {
			continue
		}
		// Only Services with `clusterIP` are needed
		if len(svcItem.Spec.ClusterIP) > 0 && svcItem.Spec.ClusterIP != "None" {
			svc.Selector  = svcItem.Spec.Selector
			svc.Labels    = svcItem.GetLabels()
			svc.Name      = svcItem.GetName()
			svc.ClusterIP = svcItem.Spec.ClusterIP
			svcs	      = append(svcs, *svc)
		}
	}
	resourceList := &npcv1.Resources{
		Pods:	 pods,
		Services: svcs,
	}
	ens := &npcv1.ExternalNamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:		 ensName,
			Namespace:	 oprNamespace,
			Labels:		 labels,
			Annotations:	 ann,
		},
		Spec: npcv1.ExternalNamespaceSpec{
			RemoteCluster:	 rcl.GetName(),
			RemoteNamespace: ns,
			Resources:	 *resourceList,
		},
	}
	return ens, nil
}

//
func (r *RemoteClusterConfigReconciler) getOperatorNamespace() string {
	log := log.FromContext(context.TODO())
	var ns string
	ns, found := os.LookupEnv(operatorNamespaceEnvVar)
	if !found {
		log.Info(fmt.Sprintf("Failed to get %s variable, let's set it to \"default\"", operatorNamespaceEnvVar))
		ns = "default"
	}
	return ns
}

// getCouple returns a RemoteClusterConfig couple from remote cluster
func (r *RemoteClusterConfigReconciler) getCouple(rcl *npcv1.RemoteClusterConfig, 
	dynamicClient *dynamic.DynamicClient) (*npcv1.RemoteClusterConfig, error) {
	rclGet := npcv1.RemoteClusterConfig{} 
	rclCouple := npcv1.RemoteClusterConfig{}
	// Getting list of RemoteClusterConfig from remote cluster
	resp, err := dynamicClient.Resource(remoteCluster).Namespace(oprNamespace).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to get list of RemoteClusterConfigs")
	}
	num := 0
	for _, unstrItem := range resp.Items {
		// Converting unstructured to RemoteClusterConfig
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(unstrItem.UnstructuredContent(), &rclGet)
		if err != nil {
			return nil, fmt.Errorf("Failed to convert Unstructured to RemoteClusterConfigs")
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
func (r *RemoteClusterConfigReconciler) nodeToTunnelIPs(c client.Client) (map[string]string, error) {
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
