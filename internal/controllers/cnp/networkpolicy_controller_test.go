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
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
	npcc "github.com/Netcracker/qubership-network-policy-operator/internal/controllers/npc"
	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
)

var _ = Describe("NetworkPolicy Controller", func() {
	Context("When reconciling a resource", func() {
		/*
		   For comprehensive test running the NetworkPolicyConfig reconciliation must be disabled and in the `test` namespaces must be ceated the following policies:
		   apiVersion: projectcalico.org/v3
		   kind: NetworkPolicy
		   metadata:
		     name: allow-from-ns
		     namespace: test1
		   spec:
		     egress:
		     - action: Allow
		       destination:
		         namespaceSelector: kubernetes.io/metadata.name in {'test-2','test-1'}
		         ports:
		         - 5555
		         selector: all()
		       protocol: TCP
		       source: {}
		     ingress:
		     - action: Log
		       destination: {}
		       source:
		         nets:
		         - 0.0.0.0/0
		     - action: Allow
		       destination:
		         ports:
		         - 5432
		       protocol: TCP
		       source:
		         namespaceSelector: kubernetes.io/metadata.name in {'test-2','test-1'}
		         selector: all()
		     - action: Allow
		       destination:
		         ports:
		         - 5000
		       protocol: TCP
		       source:
		         selector: all()
		     - action: Allow
		       destination:
		         ports:
		         - 5555
		       protocol: TCP
		       source:
		         nets:
		         - 10.0.0.0/16
		     selector: all()
		     types:
		     - Ingress
		     - Egress
		   ---
		   apiVersion: projectcalico.org/v3
		   kind: NetworkPolicy
		   metadata:
		     name: restrict-service
		     namespace: test1
		     labels:
		       network-policy.netcracker.com/generated: "true"
		       network-policy.netcracker.com/egress-svc: "true"
		   spec:
		     order: 10
		     egress:
		     - action: Allow
		       destination:
		         nets:
		           - 0.0.0.0/0
		     selector: all()
		     types:
		     - Egress
		*/

		const (
			rclName       = "test-remote-cluster-config"
			rclCoupleName = "couple-remote-cluster-config"
			rclNS         = "default"
			ensName       = "ext-ns"
			podName       = "test-pod1"
			nsName        = "test1"
			nsOpr         = "default"
			npcName       = "default1"
		)

		var pod1IP, pod2IP string = "10.132.132.132", "10.132.132.133"
		ctx := context.Background()
		policyName1 := "allow-within-ns"
		policyName2 := "restrict-service"

		typeNamespacedName := types.NamespacedName{
			Name:      ensName,
			Namespace: nsOpr,
		}

		typeNamespacedNameR := types.NamespacedName{
			Name:      rclName,
			Namespace: nsOpr,
		}

		typeNamespacedNameNPC := types.NamespacedName{
			Name:      npcName,
			Namespace: "",
		}

		typeNamespacedNameN := types.NamespacedName{
			Name:      nsName,
			Namespace: nsName,
		}

		ens := &npcv1.ExternalNamespace{}
		rcl := &npcv1.RemoteClusterConfig{}
		npc := &npcv1.NetworkPolicyConfig{}

		podLabels := map[string]string{
			"test-label": "test-value",
			"app":        "target",
		}

		svcSelector := map[string]string{
			"app": "target",
		}

		nsLabels := map[string]string{
			"kubernetes.io/metadata.name": nsName,
		}
		// It's needed to check Services NetworkPolicies
		nsAnnotations := map[string]string{
			"network-policy.netcracker.com/allowed-from": "test2",
		}

		npList := &calicov3.NetworkPolicyList{}
		labelSet := labels.Set{NetworkPolicyTargetLabel: "true"}
		opts := &client.ListOptions{}
		var accessor metav1.Object
		nsAnn := make(map[string]string)
		scope := map[string]string{"kubernetes.io/metadata.name": nsName}
		isTier := false
		dfTier := "automated"
		dfOrder := float64(10000)
		act := calicov3.Pass
		tr1 := &calicov3.Tier{}
		typeNamespacedNameT1 := types.NamespacedName{
			Name:      dfTier,
			Namespace: "",
		}
		ns := &corev1.Namespace{}
		npLabels := make(map[string]string)
		var found bool
		targetNP := &calicov3.NetworkPolicy{}

		WNS := `
    apiVersion: projectcalico.org/v3
    kind: NetworkPolicy
    metadata:
      name: allow-within-ns
      namespace: default
    spec:
      selector: all()
      types:
        - Ingress
      ingress:
        - action: Allow
          source:
            selector: all()`
		ANS := `
    apiVersion: projectcalico.org/v3
    kind: NetworkPolicy
    metadata:
      name: allow-from-ns
      namespace: default
    spec:
      selector: all()
      types:
        - Ingress
      ingress:
        - action: Log
          source:
            nets:
              - 0.0.0.0/0
        - action: Allow
          source:
            namespaceSelector: all()`
		DD := `
    apiVersion: projectcalico.org/v3
    kind: NetworkPolicy
    metadata:
      name: default-deny
      namespace: default
    spec:
      selector: all()
      ingress:
        - action: Log
          source:
            nets:
              - 0.0.0.0/0
      types:
        - Ingress`
		RS := `
    apiVersion: projectcalico.org/v3
    kind: NetworkPolicy
    metadata:
      name: restrict-service
    spec:
      order: 10
      egress:
      - action: Log
        destination:
          nets:
            - 0.0.0.0/0
      - action: Allow
        destination:
          nets:
            - 0.0.0.0/0
      selector: all()
      types:
      - Egress`
		DL := `
    apiVersion: projectcalico.org/v3
    kind: NetworkPolicy
    metadata:
      name: default-log
      namespace: default
    spec:
      selector: all()
      ingress:
        - action: Log
          source:
            nets:
              - 0.0.0.0/0
        - action: Allow
          source:
            nets:
              - 0.0.0.0/0
      types:
        - Ingress`

		policies := make(map[string]string)
		policies["AllowWithinNamespace"] = WNS
		policies["AllowFromNamespace"] = ANS
		policies["DefaultDeny"] = DD
		policies["RestrictService"] = RS
		policies["CustomPolicies"] = ""
		policies["DefaultLog"] = DL

		BeforeEach(func() {
			By("Creating the ExternalNamespace")
			err := k8sClient.Get(ctx, typeNamespacedName, ens)
			if err != nil && errors.IsNotFound(err) {
				ens = &npcv1.ExternalNamespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:        ensName,
						Namespace:   nsOpr,
						Labels:      nsLabels,
						Annotations: nsAnnotations,
					},
					Spec: npcv1.ExternalNamespaceSpec{
						RemoteCluster:   "cluster-2",
						RemoteNamespace: nsName,
						Resources: npcv1.Resources{
							Pods: []npcv1.Pod{{
								Labels:   podLabels,
								HostIP:   "192.168.56.106",
								Name:     podName,
								NodeName: "ubuntu",
								PodIP:    pod1IP,
							},
								{
									Labels:   podLabels,
									HostIP:   "192.168.56.106",
									Name:     podName,
									NodeName: "ubuntu",
									PodIP:    pod2IP,
								}},
							Services: []npcv1.Service{{
								Selector:  svcSelector,
								Name:      "test-svc",
								ClusterIP: "172.30.10.1",
							}},
						},
					},
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, ens)
				}, time.Minute, time.Second).Should(Succeed())

			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the ExternalNamespace:", err))
			}
			By("Creating the RemoteClusterConfig")
			err = k8sClient.Get(ctx, typeNamespacedNameR, rcl)
			if err != nil && errors.IsNotFound(err) {
				rcl = &npcv1.RemoteClusterConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rclName,
						Namespace: rclNS,
					},
					Spec: npcv1.RemoteClusterConfigSpec{
						RemoteCluster: npcv1.RemoteCluster{
							Name:   "cluster-2",
							API:    "all.new.local:6443",
							Secret: "remote-cluster2",
						},
						LocalCluster: npcv1.LocalCluster{
							Name: "cluster-1",
							API:  "all.new.local:6443",
						},
					},
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, rcl)
				}, time.Minute, time.Second).Should(Succeed())
				meta.SetStatusCondition(&rcl.Status.Conditions,
					metav1.Condition{
						Type:    string(TypeCoupled),
						Status:  metav1.ConditionTrue,
						Reason:  "Initializing",
						Message: "Couple is identified",
					})
				rcl.Status.ServiceSubnet = "172.30.0.0/16"
				rcl.Status.KubeAPIService = "172.30.0.1"
				rcl.Status.CoreDNSService = "172.30.0.10"
				rcl.Status.CoupleServiceSubnet = "172.30.0.0/16"
				rcl.Status.CoupleKubeAPIService = "172.30.0.1"
				rcl.Status.CoupleCoreDNSService = "172.30.0.10"
				Eventually(func() error {
					return k8sClient.Status().Update(ctx, rcl)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the RemoteClusterConfig:", err))
			}

			err = k8sClient.Get(ctx, typeNamespacedNameNPC, npc)
			if err != nil && errors.IsNotFound(err) {
				npc = &npcv1.NetworkPolicyConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: npcName,
					},
					Spec: npcv1.NetworkPolicyConfigSpec{
						Enabled:  true,
						Scope:    scope,
						Policies: policies,
						Logs: npcv1.LogsSpec{
							Enabled:       true,
							CollectingAll: true,
						},
					},
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, npc)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the NetworkPolicyConfig:", err))
			}

			By("Creating the Namespace")
			err = k8sClient.Get(ctx, typeNamespacedNameN, ns)
			if err != nil && errors.IsNotFound(err) {
				ns = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: nsName,
					},
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, ns)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the Namespace:", err))
			}

			// Tiers availability check
			trList := &calicov3.TierList{}
			err = k8sClient.List(ctx, trList, &client.ListOptions{})
			if err == nil {
				// Calico 3.29
				isTier = true
				By("Current Calico version supports Tiers, so it's 3.29 and higher")
				By("Creating the Default Tier")
				err = k8sClient.Get(ctx, typeNamespacedNameT1, tr1)
				if err != nil && errors.IsNotFound(err) {
					tr1 = &calicov3.Tier{
						ObjectMeta: metav1.ObjectMeta{
							Name: dfTier,
						},
						Spec: calicov3.TierSpec{
							Order:         &dfOrder,
							DefaultAction: &act,
						},
					}
					Eventually(func() error {
						return k8sClient.Create(ctx, tr1)
					}, time.Minute, time.Second).Should(Succeed())
				} else if err != nil {
					Fail(fmt.Sprint("Failed to get the Tier:", err))
				}
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, ens)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the ExternalNamespace")
			Eventually(func() error {
				return k8sClient.Delete(ctx, ens)
			}, time.Minute, time.Second).Should(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameR, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the RemoteClusterConfig")
			Eventually(func() error {
				return k8sClient.Delete(ctx, rcl)
			}, time.Minute, time.Second).Should(Succeed())

			By("Cleanup the NetworkPoliciyConfig")
			Eventually(func() error {
				return k8sClient.Delete(ctx, npc)
			}, time.Minute, time.Second).Should(Succeed())

			time.Sleep(15 * time.Second)

			// It's a workaround to fix the cascading deletion
			By("Cleanup the NetworkPolicies")
			opts = &client.ListOptions{}
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			for _, npItem := range npList.Items {
				accessor, err = meta.Accessor(&npItem)
				Expect(err).NotTo(HaveOccurred())
				npLabels = accessor.GetLabels()
				// Local NetworkPolicy Deleting
				_, ok := npLabels[NetworkPolicyLabel]
				if ok {
					By(fmt.Sprintf("Cleanup the '%s' NetworkPolicy", npItem.GetName()))
					Eventually(func() error {
						return k8sClient.Delete(ctx, &npItem)
					}, time.Minute, time.Second).Should(Succeed())
				}
				// DR NetworkPolicy Deleting
				_, ok = npLabels[NetworkPolicyTargetLabel]
				if ok {
					By(fmt.Sprintf("Cleanup the '%s' NetworkPolicy", npItem.GetName()))
					Eventually(func() error {
						return k8sClient.Delete(ctx, &npItem)
					}, time.Minute, time.Second).Should(Succeed())
				}
			}

			err = k8sClient.Get(ctx, typeNamespacedNameN, ns)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the Namespace")
			Eventually(func() error {
				return k8sClient.Delete(ctx, ns)
			}, time.Minute, time.Second).Should(Succeed())

			// Tiers availability check
			trList := &calicov3.TierList{}
			err = k8sClient.List(ctx, trList, &client.ListOptions{})
			if err == nil {
				err = k8sClient.Get(ctx, typeNamespacedNameT1, tr1)
				By("Cleanup the Default Tier")
				Eventually(func() error {
					return k8sClient.Delete(ctx, tr1)
				}, time.Minute, time.Second).Should(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			// Comment the NetworkPolicyConfig reconciliation to run the comprehensive NetworkPolicy processing test
			// Start comment
			By("Reconciling the NetworkPolicyConfig")
			controllerReconcilerNPC := &npcc.NetworkPolicyConfigReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconcilerNPC.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameNPC,
			})
			// End comment

			if isTier {
				policyName1 = fmt.Sprintf("%s.%s", dfTier, policyName1)
				policyName2 = fmt.Sprintf("%s.%s", dfTier, policyName2)
			}
			typeNamespacedNameNP := types.NamespacedName{
				Name:      policyName1,
				Namespace: nsName,
			}

			typeNamespacedNameNP2 := types.NamespacedName{
				Name:      policyName2,
				Namespace: nsName,
			}

			By("Reconciling the NetworkPolicy")
			controllerReconciler := &NetworkPolicyReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameNP,
			})

			By("Checking if the NetworkPolicy was successfully created")
			opts = &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelSet),
			}

			time.Sleep(45 * time.Second)
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			found = false
			for _, npItem := range npList.Items {
				accessor, err = meta.Accessor(&npItem)
				if err != nil {
					Fail(fmt.Sprint("Failed to get the Annotation:", err))
				}
				nsAnn = accessor.GetAnnotations()
				if len(nsAnn) > 0 {
					if nsAnn[NetworkPolicyOriginAnnotation] == policyName1 {
						found = true
						targetNP = &npItem
					}
				}
			}
			if !found {
				Fail("Failed to find the NetworkPolicy")
			}

			By("Checking Ingress IPs in NetworkPolicy")
			if slices.Contains(targetNP.Spec.Ingress[0].Source.Nets,
				fmt.Sprintf("%s/32", pod1IP)) &&
				slices.Contains(targetNP.Spec.Ingress[0].Source.Nets,
					fmt.Sprintf("%s/32", pod2IP)) &&
				(len(targetNP.Spec.Ingress[0].Source.Nets) == 2) {
				By("Ingress IPs are matched")
			} else {
				Fail("Ingress IPs are not mached")
				By(fmt.Sprintf("Actual IPs: %v", targetNP.Spec.Ingress[0].Source.Nets))
			}

			if len(targetNP.Spec.Egress) > 0 {
				By("Checking Egress IPs in NetworkPolicy")
				if slices.Contains(targetNP.Spec.Egress[0].Destination.Nets, fmt.Sprintf("%s/32", pod1IP)) && slices.Contains(targetNP.Spec.Egress[0].Destination.Nets, fmt.Sprintf("%s/32", pod2IP)) && (len(targetNP.Spec.Egress[0].Destination.Nets) == 2) {
					By("Egress IPs are matched")
				} else {
					Fail("Egress IPs are not mached")
				}
			}

			By("Update the ExternalNamespace")
			pod1IP = "10.132.132.128"
			pod2IP = "10.132.132.129"
			err = k8sClient.Get(ctx, typeNamespacedName, ens)
			ens.Spec.Resources.Pods[0].PodIP = pod1IP
			ens.Spec.Resources.Pods[1].PodIP = pod2IP
			err = k8sClient.Update(ctx, ens)
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the ExternalNamespace update")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameNP,
			})

			By("Checking if the NetworkPolicy was successfully updated")
			Eventually(func() error {
				return k8sClient.List(ctx, npList, opts)
			}, time.Minute, time.Second).Should(Succeed())
			Expect(err).NotTo(HaveOccurred())
			found = false
			for _, npItem := range npList.Items {
				accessor, err = meta.Accessor(&npItem)
				if err != nil {
					Fail(fmt.Sprint("Failed to get the Annotation:", err))
				}
				nsAnn = accessor.GetAnnotations()
				if len(nsAnn) > 0 {
					if nsAnn[NetworkPolicyOriginAnnotation] == policyName1 {
						found = true
						targetNP = &npItem
					}
				}
			}
			if !found {
				Fail("Failed to find the NetworkPolicy")
			}

			By("Checking Ingress IPs in NetworkPolicy")
			if slices.Contains(targetNP.Spec.Ingress[0].Source.Nets,
				fmt.Sprintf("%s/32", pod1IP)) &&
				slices.Contains(targetNP.Spec.Ingress[0].Source.Nets,
					fmt.Sprintf("%s/32", pod2IP)) &&
				(len(targetNP.Spec.Ingress[0].Source.Nets) == 2) {
				By("Ingress IPs are matched")
			} else {
				Fail("Ingress IPs are not mached")
			}

			if len(targetNP.Spec.Egress) > 0 {
				By("Checking Egress IPs in NetworkPolicy")
				if slices.Contains(targetNP.Spec.Egress[0].Destination.Nets, fmt.Sprintf("%s/32", pod1IP)) && slices.Contains(targetNP.Spec.Egress[0].Destination.Nets, fmt.Sprintf("%s/32", pod2IP)) && (len(targetNP.Spec.Egress[0].Destination.Nets) == 2) {
					By("Egress IPs are matched")
				} else {
					Fail("Egress IPs are not mached")
				}
			}

			By("Reconciling Services NetworkPolicy")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameNP2,
			})

			By("Checking if the Services NetworkPolicy was successfully created")
			Eventually(func() error {
				return k8sClient.List(ctx, npList, opts)
			}, time.Minute, time.Second).Should(Succeed())
			Expect(err).NotTo(HaveOccurred())
			found = false
			for _, npItem := range npList.Items {
				accessor, err = meta.Accessor(&npItem)
				if err != nil {
					Fail(fmt.Sprint("Failed to get the Annotation:", err))
				}
				nsAnn = accessor.GetAnnotations()
				if len(nsAnn) > 0 {
					if nsAnn[NetworkPolicyOriginAnnotation] == policyName2 {
						found = true
						targetNP = &npItem
					}
				}
			}
			if !found {
				Fail(fmt.Sprint("Failed to find the NetworkPolicy:", err))
			}

		})
	})
})
