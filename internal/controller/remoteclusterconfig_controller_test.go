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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	npcv1 "git.netcracker.com/PROD.Platform.HA/network-policy-operator/api/v1alpha1"
)

var _ = Describe("RemoteClusterConfig Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			rclName = "test-remote-cluster-config"
			rclCoupleName = "couple-remote-cluster-config"
			rclNS   = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      rclName,
			Namespace: rclNS,
		}
		typeNamespacedNameC := types.NamespacedName{
			Name:      rclCoupleName,
			Namespace: rclNS,
		}
		rcl := &npcv1.RemoteClusterConfig{}
		rclCouple := &npcv1.RemoteClusterConfig{}
		ensList := &npcv1.ExternalNamespaceList{}

		BeforeEach(func() {
			By("Creating the custom resource for the Kind RemoteClusterConfig")
			err := k8sClient.Get(ctx, typeNamespacedName, rcl)
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
			By("Creating the couple for RemoteClusterConfig")
			err = k8sClient.Get(ctx, typeNamespacedNameC, rclCouple)
			if err != nil && errors.IsNotFound(err) {
				rclCouple = &npcv1.RemoteClusterConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rclCoupleName,
						Namespace: rclNS,
					},
					Spec: npcv1.RemoteClusterConfigSpec{
						RemoteCluster: npcv1.RemoteCluster{
							Name:   "cluster-1",
							API:    "all.new.local:6443",
							Secret: "remote-cluster1",
						},
						LocalCluster: npcv1.LocalCluster{
							Name: "cluster-2",
							API:  "all.new.local:6443",
						},
					},
				}
				Expect(k8sClient.Create(ctx, rclCouple)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rcl)).To(Succeed())

			err = k8sClient.List(ctx, ensList)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedNameC, rclCouple)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the couple for RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rclCouple)).To(Succeed())

		})
		It("should successfully reconcile the resource", func() {
			controllerReconciler := &RemoteClusterConfigReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Let's reconcile the couple RemoteClusterConfig. It will be skipped basically,
			// since it hasn't had couple yet, but it's necessary for further comprehensive
			// local RemoteClusterConfig reconciliation
			By("Reconciling the RemoteClusterConfig couple")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameC,
			})

			By("Reconciling the RemoteClusterConfig")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
