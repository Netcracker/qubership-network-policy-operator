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
	"slices"
	"time"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/client"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
)

var _ = Describe("NetworkPolicy Controller", func() {
	Context("When reconciling a resource", func() {
/*
For comprehensive test running the NetworkPolicyConfig reconciliation must be disabled the `test` namespaces must be ceated the following policies must be applied in it:
apiVersion: projectcalico.org/v3
kind: NetworkPolicy
metadata:
  name: allow-from-ns
  namespace: test
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
  namespace: test
  labels:
    network-policy.qubership.org/generated: "true"
    network-policy.qubership.org/egress-svc: "true"
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
			rclName	      = "test-remote-cluster-config"
			rclCoupleName = "couple-remote-cluster-config"
			rclNS	     = "default"
			ensName	     = "ext-ns"
			podName      = "test-pod1"
			ns           = "test"
			nsOpr        = "default"
			policyName1  = "allow-from-ns"
			policyName2  = "restrict-service"
			npcName	     = "default"
		)

		var pod1IP, pod2IP string = "10.132.132.132", "10.132.132.133"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      ensName,
			Namespace: nsOpr,
		}

		typeNamespacedNameNP := types.NamespacedName{
			Name:      policyName1,
			Namespace: ns,
		}

		typeNamespacedNameNP2 := types.NamespacedName{
			Name:      policyName2,
			Namespace: ns,
		}

		typeNamespacedNameR := types.NamespacedName{
			Name:      rclName,
			Namespace: nsOpr,
		}

		typeNamespacedNameNPC := types.NamespacedName{
			Name:      npcName,
			Namespace: "",
		}

		ens := &npcv1.ExternalNamespace{}
		rcl := &npcv1.RemoteClusterConfig{}
		npc := &npcv1.NetworkPolicyConfig{}

		podLabels := map[string]string{
			"test-label": "test-value",
			"app":        "target",
		}

		svcSelector := map[string]string{
			"app":        "target",
		}

		nsLabels := map[string]string{
			"kubernetes.io/metadata.name": ns,
		}
		// It's needed to check Services NetworkPolicies
		nsAnnotations := map[string]string{
			"network-policy.qubership.org/allowed-from": "test2",
		}

		npList := &calicov3.NetworkPolicyList{}
		opts := &client.ListOptions{}
		var accessor metav1.Object
		nsAnn := make(map[string]string)
		scope := map[string]string{"kubernetes.io/metadata.name": "test"}

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
		policies["AllowFromNamespace"] 	 = ANS
		policies["DefaultDeny"]		 = DD
		policies["RestrictService"]	 = RS
		policies["CustomPolicies"]	 = ""
		policies["DefaultLog"]		 = DL

		BeforeEach(func() {
			By("Creating the ExternalNamespace")
			err := k8sClient.Get(ctx, typeNamespacedName, ens)
			if err != nil && errors.IsNotFound(err) {
				ens = &npcv1.ExternalNamespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:        ensName,
						Namespace:   nsOpr,
						Labels:	     nsLabels,
						Annotations: nsAnnotations,
					},
					Spec: npcv1.ExternalNamespaceSpec{
						RemoteCluster:   "cluster-2",
						RemoteNamespace: ns,
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
								Selector: svcSelector,
								Name:      "test-svc",
								ClusterIP: "172.30.10.1",
							}},
						},
					},
				}
				Expect(k8sClient.Create(ctx, ens)).To(Succeed())
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
				Expect(k8sClient.Create(ctx, rcl)).To(Succeed())
				meta.SetStatusCondition(&rcl.Status.Conditions,
							metav1.Condition{
								Type: TypeCoupled,
								Status: metav1.ConditionTrue,
								Reason: "Initializing",
								Message: "Couple is identified",
							})
				rcl.Status.ServiceSubnet  = "172.30.0.0/16"
				rcl.Status.KubeAPIService = "172.30.0.1"
				rcl.Status.CoreDNSService = "172.30.0.10"
				rcl.Status.CoupleServiceSubnet  = "172.30.0.0/16"
				rcl.Status.CoupleKubeAPIService = "172.30.0.1"
				rcl.Status.CoupleCoreDNSService = "172.30.0.10"
				Expect(k8sClient.Status().Update(ctx, rcl)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedNameNPC, npc)
			if err != nil && errors.IsNotFound(err) {
				npc = &npcv1.NetworkPolicyConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      npcName,
					},
					Spec: npcv1.NetworkPolicyConfigSpec{
						Enabled:  true,
						Scope:	  scope,
						Policies: policies,
						Logs:	  npcv1.LogsSpec{
							Enabled:       true,
							CollectingAll: true,
						},
					},
				}
				Expect(k8sClient.Create(ctx, npc)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, ens)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ExternalNamespace")
			Expect(k8sClient.Delete(ctx, ens)).To(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameR, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rcl)).To(Succeed())

			By("Cleanup NetworkPoliciyConfig")
			Expect(k8sClient.Delete(ctx, npc)).To(Succeed())

			By("Cleanup the target NetworkPolicies")
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			for _, npItem := range npList.Items {
				accessor, err = meta.Accessor(&npItem)
				Expect(err).NotTo(HaveOccurred())
				nsAnn = accessor.GetAnnotations()
				if nsAnn[NetworkPolicyOriginAnnotation] ==  policyName1 {
					Expect(k8sClient.Delete(ctx, &npItem)).To(Succeed())
				}
			}
		})

		It("should successfully reconcile the resource", func() {
			// Comment the NetworkPolicyConfig reconciliation to run the comprehensive NetworkPolicy processing test
			// Start comment
			By("Reconciling the NetworkPolicyConfig")
			controllerReconcilerNPC := &NetworkPolicyConfigReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconcilerNPC.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameNPC,
			})
			// End comment

			By("Reconciling the NetworkPolicy")
			controllerReconciler := &NetworkPolicyReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameNP,
			})

			By("Checking if the NetworkPolicy was successfully created")
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				for _, npItem := range npList.Items {
					accessor, err = meta.Accessor(&npItem)
					if err != nil {
						return err
					}
					nsAnn = accessor.GetAnnotations()
					if len(nsAnn) > 0 {
						if nsAnn[NetworkPolicyOriginAnnotation] ==  policyName1 {
							return nil
						}
					}
				}
				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking Ingress IPs in NetworkPolicy")
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				for _, npItem := range npList.Items {
					accessor, err = meta.Accessor(&npItem)
					if err != nil {
						return err
					}
					nsAnn = accessor.GetAnnotations()
					if nsAnn[NetworkPolicyOriginAnnotation] ==  policyName1 {
						if slices.Contains(npItem.Spec.Ingress[0].Source.Nets, 
								fmt.Sprintf("%s/32", pod1IP)) &&
							slices.Contains(npItem.Spec.Ingress[0].Source.Nets, 
									fmt.Sprintf("%s/32", pod2IP)) &&
							(len(npItem.Spec.Ingress[0].Source.Nets) == 2) {
								return nil
						}
					}
				}
				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking Egress IPs in NetworkPolicy")
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				for _, npItem := range npList.Items {
					accessor, err = meta.Accessor(&npItem)
					if err != nil {
						return err
					}
					nsAnn = accessor.GetAnnotations()
					if nsAnn[NetworkPolicyOriginAnnotation] ==  policyName1 {
						if len(npItem.Spec.Egress) > 0 {
							if slices.Contains(npItem.Spec.Egress[0].Destination.Nets, fmt.Sprintf("%s/32", pod1IP)) && slices.Contains(npItem.Spec.Egress[0].Destination.Nets, fmt.Sprintf("%s/32", pod2IP)) && (len(npItem.Spec.Egress[0].Destination.Nets) == 2) {
								return nil
							}
						}
					}
				}
				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("Update the ExternalNamespace")
			pod1IP = "10.132.132.128"
			pod2IP = "10.132.132.129"
			err = k8sClient.Get(ctx, typeNamespacedName, ens)
			ens.Spec.Resources.Pods[0].PodIP = pod1IP
			ens.Spec.Resources.Pods[1].PodIP = pod2IP
			err = k8sClient.Update(ctx, ens)

			By("Reconciling the ExternalNamespace update")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameNP,
			})

			By("Checking if the NetworkPolicy was successfully updated")
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				for _, npItem := range npList.Items {
					accessor, err = meta.Accessor(&npItem)
					if err != nil {
						return err
					}
					nsAnn = accessor.GetAnnotations()
					if nsAnn[NetworkPolicyOriginAnnotation] ==  policyName1 {
						return nil
					}
				}
				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking Ingress IPs in NetworkPolicy")
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				for _, npItem := range npList.Items {
					accessor, err = meta.Accessor(&npItem)
					if err != nil {
						return err
					}
					nsAnn = accessor.GetAnnotations()
					if nsAnn[NetworkPolicyOriginAnnotation] ==  policyName1 {
						if slices.Contains(npItem.Spec.Ingress[0].Source.Nets, 
								fmt.Sprintf("%s/32", pod1IP)) &&
							slices.Contains(npItem.Spec.Ingress[0].Source.Nets, 
									fmt.Sprintf("%s/32", pod2IP)) &&
							(len(npItem.Spec.Ingress[0].Source.Nets) == 2) {
								return nil
						}
					}
				}
				return err
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking Egress IPs in NetworkPolicy")
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				for _, npItem := range npList.Items {
					accessor, err = meta.Accessor(&npItem)
					if err != nil {
						return err
					}
					nsAnn = accessor.GetAnnotations()
					if nsAnn[NetworkPolicyOriginAnnotation] ==  policyName1 {
						if len(npItem.Spec.Egress) > 0 {
							if slices.Contains(npItem.Spec.Egress[0].Destination.Nets, fmt.Sprintf("%s/32", pod1IP)) && slices.Contains(npItem.Spec.Egress[0].Destination.Nets, fmt.Sprintf("%s/32", pod2IP)) && (len(npItem.Spec.Egress[0].Destination.Nets) == 2) {
								return nil
							}
						}
					}
				}
				return err
			}, time.Minute, time.Second).Should(Succeed())

			Expect(err).NotTo(HaveOccurred())			

			By("Reconciling Services NetworkPolicy")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameNP2,
			})

			By("Checking if the Services NetworkPolicy was successfully created")
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				for _, npItem := range npList.Items {
					accessor, err = meta.Accessor(&npItem)
					if err != nil {
						return err
					}
					nsAnn = accessor.GetAnnotations()
					if len(nsAnn) > 0 {
						if nsAnn[NetworkPolicyOriginAnnotation] ==  policyName2 {
							return nil
						}
					}
				}
				return err
			}, time.Minute, time.Second).Should(Succeed())

		})
	})
})
