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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
)

var _ = Describe("NetworkPolicyConfig Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			npcName    = "default"
			npcNameAdd = "default2"
			rclName    = "test-remote-cluster-config"
			rclNS      = "default"
		)

		var fullNPList = []string{"allow-from-ns", "allow-within-ns", "default-deny", "restrict-service"}
		var logNP = []string{"default-log"}

		ctx := context.Background()

		typeNamespacedName1 := types.NamespacedName{
			Name:      npcName,
			Namespace: "",
		}
		typeNamespacedNameAdd := types.NamespacedName{
			Name:      npcNameAdd,
			Namespace: "",
		}
		typeNamespacedName2 := types.NamespacedName{
			Name:      npcName,
			Namespace: "test3",
		}
		typeNamespacedNameR := types.NamespacedName{
			Name:      rclName,
			Namespace: rclNS,
		}
		npc := &npcv1.NetworkPolicyConfig{}
		npc2 := &npcv1.NetworkPolicyConfig{}
		rcl := &npcv1.RemoteClusterConfig{}
		npList := &calicov3.NetworkPolicyList{}
		opts := &client.ListOptions{}
		en := true
		// To test particular namespace set the following:
		// scope := map[string]string{"kubernetes.io/metadata.name": "<NAMESPACE>"}
		// To test whole cluster:
		// scope := map[string]string{}
		scope := map[string]string{"npc": "true"}
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
		CST := `
    ---
    apiVersion: projectcalico.org/v3
    kind: NetworkPolicy
    metadata:
      name: custom-1
      namespace: test
      labels:
        test1: "true"
    spec:
      selector: all()
      egress:
        - action: Allow
          destination:
            services:
              name: kubernetes
              namespace: default
    ---
    apiVersion: projectcalico.org/v3
    kind: NetworkPolicy
    metadata:
      name: custom-2
      namespace: test
      labels:
        test2: "true"
    spec:
      selector: all()
      egress:
        - action: Allow
          destination:
            services:
              name: kube-dns
              namespace: kube-system`
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
		policies["CustomPolicies"]	 = CST
		policies["DefaultLog"]		 = DL

		BeforeEach(func() {
			By("creating the custom resource for the Kind NetworkPolicyConfig")
			err := k8sClient.Get(ctx, typeNamespacedName1, npc)
			if err != nil && errors.IsNotFound(err) {
				npc = &npcv1.NetworkPolicyConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      npcName,
					},
					Spec: npcv1.NetworkPolicyConfigSpec{
						Enabled:  en,
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
			By("Creating RemoteClusterConfig")
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
			}
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

		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName1, npc)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NetworkPolicyConfig")
			Expect(k8sClient.Delete(ctx, npc)).To(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameAdd, npc2)
			if err == nil {
				By("Deleting the second NetworkPolicyConfig")
				Expect(k8sClient.Delete(ctx, npc2)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedNameR, rcl)
			By("Cleanup the RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rcl)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the NetworkPolicyConfig")
			controllerReconciler := &NetworkPolicyConfigReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName1,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the NetworkPolicy was successfully created in regular Namespace")
			opts = &client.ListOptions{Namespace: "default",}
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			var npNamesList []string
			for _, npItem := range npList.Items {
				npNamesList = append(npNamesList, npItem.GetName())
			}
			By(fmt.Sprintf("NetwokPolicies list in regular Namespace: %v", npNamesList))
			if !slices.Equal(fullNPList, npNamesList) {
				err = fmt.Errorf("Wrong NetworkPolicies list in regular Namespace. Expected: '%v' Actual: '%v'", fullNPList, npNamesList)
			} else {
				err = nil
			}
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the NetworkPolicy was successfully created in control-plane Namespace")
			opts = &client.ListOptions{Namespace: "kube-system",}
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			var logNamesList []string
			for _, npItem := range npList.Items {
				logNamesList = append(logNamesList, npItem.GetName())
			}
			By(fmt.Sprintf("NetwokPolicies list in control-plane Namespace: %v", logNamesList))
			if !slices.Equal(logNP, logNamesList) {
				err = fmt.Errorf("Wrong NetworkPolicies list in control-plane Namespace. Expected: '%v' Actual: '%v'", logNP, logNamesList)
			} else {
				err = nil
			}
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the Namespace")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName2,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Disabling default policies in NetworkPolicyConfig")
			npc.Spec.Enabled = false
			Expect(k8sClient.Update(ctx, npc)).To(Succeed())
			By("Reconciling the Deletion")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName1,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling several NetworkPolicyConfigs")
				npc2 = &npcv1.NetworkPolicyConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      npcNameAdd,
					},
					Spec: npcv1.NetworkPolicyConfigSpec{
						Enabled:  en,
						Scope:	  scope,
						Policies: policies,
					},
				}
			Expect(k8sClient.Create(ctx, npc2)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName1,
			})
			Expect(err).NotTo(HaveOccurred())

		})
	})
})
