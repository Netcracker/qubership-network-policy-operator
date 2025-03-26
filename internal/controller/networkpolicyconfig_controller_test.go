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

		defaultDeny := "default-deny"

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
		scope := map[string]string{}
		//scope := map[string]string{"npc": "true"}
		isTier := false
		dfTier := "automated"
		dfNewTier := "renewed"
		cmTier := "customized"
		dfOrder := float64(10000)
		dfNewOrder := float64(11000)
		cmOrder := float64(110000)
		tr1 := &calicov3.Tier{}
		tr2 := &calicov3.Tier{}
		tr3 := &calicov3.Tier{}
		typeNamespacedNameT1 := types.NamespacedName{
			Name:      dfTier,
			Namespace: "",
		}
		typeNamespacedNameT2 := types.NamespacedName{
			Name:      dfNewTier,
			Namespace: "",
		}
		typeNamespacedNameT3 := types.NamespacedName{
			Name:      cmTier,
			Namespace: "",
		}
		act :=	calicov3.Pass
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
			By("Creating the NetworkPolicyConfig")
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
							Name:      dfTier,
						},
						Spec: calicov3.TierSpec{
							Order:		&dfOrder,
							DefaultAction:	&act,
						},
					}
					Expect(k8sClient.Create(ctx, tr1)).To(Succeed())
				}

				By("Creating the new Default Tier")
				err = k8sClient.Get(ctx, typeNamespacedNameT2, tr2)
				if err != nil && errors.IsNotFound(err) {
					tr2 = &calicov3.Tier{
						ObjectMeta: metav1.ObjectMeta{
							Name:      dfNewTier,
						},
						Spec: calicov3.TierSpec{
							Order:		&dfNewOrder,
							DefaultAction:	&act,
						},
					}
					Expect(k8sClient.Create(ctx, tr2)).To(Succeed())
				}

				By("Creating the Custom Tier")
				err = k8sClient.Get(ctx, typeNamespacedNameT3, tr3)
				if err != nil && errors.IsNotFound(err) {
					tr3 = &calicov3.Tier{
						ObjectMeta: metav1.ObjectMeta{
							Name:      cmTier,
						},
						Spec: calicov3.TierSpec{
							Order:		&cmOrder,
							DefaultAction:	&act,
						},
					}
					Expect(k8sClient.Create(ctx, tr3)).To(Succeed())
				}
			} else {
				// Calico 3.28
				By("Current Calico version doesn't support Tiers, so it's 3.28 or lower")
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName1, npc)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NetworkPolicyConfig")
			Expect(k8sClient.Delete(ctx, npc)).To(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameAdd, npc2)
			if err == nil {
				By("Cleanup the second NetworkPolicyConfig")
				Expect(k8sClient.Delete(ctx, npc2)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedNameR, rcl)
			By("Cleanup the RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rcl)).To(Succeed())

			// Tiers availability check
			trList := &calicov3.TierList{}
			err = k8sClient.List(ctx, trList, &client.ListOptions{})
			if err == nil {
				err = k8sClient.Get(ctx, typeNamespacedNameT1, tr1)
				By("Cleanup the Default Tier")
				Expect(k8sClient.Delete(ctx, tr1)).To(Succeed())

				err = k8sClient.Get(ctx, typeNamespacedNameT2, tr2)
				By("Cleanup the new Default Tier")
				Expect(k8sClient.Delete(ctx, tr2)).To(Succeed())

				err = k8sClient.Get(ctx, typeNamespacedNameT3, tr3)
				By("Cleanup the Custom Tier")
				Expect(k8sClient.Delete(ctx, tr3)).To(Succeed())
			}
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

			By("Checking Tiers")
			if isTier {
				for i, item := range fullNPList {
					fullNPList[i] = fmt.Sprintf("%s.%s", dfTier, item)
				}
				for i, item := range logNP {
					logNP[i] = fmt.Sprintf("%s.%s", dfTier, item)
				}
				defaultDeny = fmt.Sprintf("%s.%s", dfTier, defaultDeny)
			}
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

			By("Changing NetworkPolicyConfig to collect only denied connections")
			npc.Spec.Logs.CollectingAll = false
			Expect(k8sClient.Update(ctx, npc)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName1,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the NetworkPolicy was successfully updated in regular Namespace")
			opts = &client.ListOptions{Namespace: "default",}
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			for _, npItem := range npList.Items {
				for _, rule := range npItem.Spec.Ingress {
					if (rule.Action == calicov3.Log) && (npItem.GetName() != defaultDeny) {
						err = fmt.Errorf("'%s' NetworkPolicy includes Log rule", npItem.GetName())
					}
				}
			}
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the NetworkPolicy was successfully deleted in control-plane Namespace")
			opts = &client.ListOptions{Namespace: "kube-system",}
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			logNamesList = []string{}
			for _, npItem := range npList.Items {
				logNamesList = append(logNamesList, npItem.GetName())
			}
			By(fmt.Sprintf("NetwokPolicies list in control-plane Namespace: %v", logNamesList))
			if len(npList.Items) > 0 {
				err = fmt.Errorf("Log NetworkPolicy hasn't been delete from control-plane Namespace")
			}
			Expect(err).NotTo(HaveOccurred())

			By("Disabling logs in NetworkPolicyConfig")
			npc.Spec.Logs.Enabled = false
			Expect(k8sClient.Update(ctx, npc)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName1,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the NetworkPolicy was successfully updated in regular Namespace")
			opts = &client.ListOptions{Namespace: "default",}
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			for _, npItem := range npList.Items {
				for _, rule := range npItem.Spec.Ingress {
					if (rule.Action == calicov3.Log) {
						err = fmt.Errorf("'%s' NetworkPolicy includes Log rule", npItem.GetName())
					}
				}
			}
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the NetworkPolicy was successfully deleted in control-plane Namespace")
			opts = &client.ListOptions{Namespace: "kube-system",}
			err = k8sClient.List(ctx, npList, opts)
			Expect(err).NotTo(HaveOccurred())
			logNamesList = []string{}
			for _, npItem := range npList.Items {
				logNamesList = append(logNamesList, npItem.GetName())
			}
			By(fmt.Sprintf("NetwokPolicies list in control-plane Namespace: %v", logNamesList))
			if len(npList.Items) > 0 {
				err = fmt.Errorf("Log NetworkPolicy hasn't been delete from control-plane Namespace")
			}
			Expect(err).NotTo(HaveOccurred())

			if isTier {
				By("Reconciling non-existent Default Tier")
				npc.Spec.DefaultTier = "nonexistent"
				Expect(k8sClient.Update(ctx, npc)).To(Succeed())
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName1,
				})
				Expect(err).To(MatchError("Default Tier doesn't exist"))

				By("Reconciling non-existent Custom Tier")
				npc.Spec.DefaultTier = dfTier
				npc.Spec.CustomTier = "nonexistent"
				Expect(k8sClient.Update(ctx, npc)).To(Succeed())
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName1,
				})
				Expect(err).To(MatchError("Custom Tier doesn't exist"))

				By("Reconciling wrong order of Custom Tier")
				npc.Spec.DefaultTier = dfTier
				npc.Spec.CustomTier = dfTier
				Expect(k8sClient.Update(ctx, npc)).To(Succeed())
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName1,
				})
				Expect(err).To(MatchError("Custom Tier Order must be higher than Default Tier Order"))

				By("Reconciling change of Default Tier")
				npc.Spec.DefaultTier = dfNewTier
				npc.Spec.CustomTier = "default"
				Expect(k8sClient.Update(ctx, npc)).To(Succeed())
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName1,
				})
				Expect(err).NotTo(HaveOccurred())
				opts = &client.ListOptions{}
				err = k8sClient.List(ctx, npList, opts)
				Expect(err).NotTo(HaveOccurred())
				for _, npItem := range npList.Items {
					if npItem.Spec.Tier == dfTier {
						err = fmt.Errorf("The Default NetworkPolicies that belong to the previous Tier have not been deleted")
					}
				}
				Expect(err).NotTo(HaveOccurred())

				By("Reconciling change of Custom Tier")
				npc.Spec.CustomTier = cmTier
				Expect(k8sClient.Update(ctx, npc)).To(Succeed())
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName1,
				})
				Expect(err).NotTo(HaveOccurred())
				opts = &client.ListOptions{}
				err = k8sClient.List(ctx, npList, opts)
				Expect(err).NotTo(HaveOccurred())
				for _, npItem := range npList.Items {
					if npItem.Spec.Tier == "default" {
						err = fmt.Errorf("The Custom NetworkPolicies that belong to the previous Tier have not been deleted")
					}
				}
				Expect(err).NotTo(HaveOccurred())
			}

			By("Disabling default policies in NetworkPolicyConfig")
			npc.Spec.Enabled = false
			if isTier {
				npc.Spec.CustomTier = "default"
			}
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
