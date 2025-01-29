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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"

	npcv1 "git.netcracker.com/PROD.Platform.HA/network-policy-operator/api/v1alpha1"
)

var _ = Describe("Service Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			rclName = "test-remote-cluster-config"
			rclCoupleName = "couple-remote-cluster-config"
			nsName	= "test-ns-svc"
			svcName = "test-svc"
			podName = "test-pod"
			rclNS   = "default"
		)


		ctx := context.Background()

		label := map[string]string{
			"remote": "true",
		}

		typeNamespacedName := types.NamespacedName{
			Name:      rclName,
			Namespace: rclNS,
		}
		typeNamespacedNameC := types.NamespacedName{
			Name:      rclCoupleName,
			Namespace: rclNS,
		}
		typeNamespacedNameN := types.NamespacedName{
			Name:      nsName,
			Namespace: nsName,
		}
		typeNamespacedNameS := types.NamespacedName{
			Name:      svcName,
			Namespace: nsName,
		}
		rcl := &npcv1.RemoteClusterConfig{}
		rclCouple := &npcv1.RemoteClusterConfig{}
		ens := &npcv1.ExternalNamespace{}
		ns := &corev1.Namespace{}
		svc := &corev1.Service{}

		BeforeEach(func() {
			By("Creating RemoteClusterConfig")
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
			By("Creating the Namespace resource")
			err = k8sClient.Get(ctx, typeNamespacedNameN, ns)
			if err != nil && errors.IsNotFound(err) {
				ns = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nsName,
					},
				}
				Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			}

			By("Getting the Namespace labels")
			labels := make(map[string]string)
			accessor, err := meta.Accessor(ns)
			labels = accessor.GetLabels()
			Expect(err).NotTo(HaveOccurred())

			By("Creating the Service resource")
			err = k8sClient.Get(ctx, typeNamespacedNameS, svc)
			if err != nil && errors.IsNotFound(err) {
				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        svcName,
						Namespace:   nsName,
						Labels:	     label,
					},
					Spec: corev1.ServiceSpec{
						Selector:  label,
						ClusterIP: "172.30.10.10",
						Ports:	   []corev1.ServicePort{{
							Port:	    5000,
							Name:	    "random",
							Protocol:   corev1.ProtocolTCP,
							TargetPort: intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 5000,
							},
						}},
					},
				}
				Expect(k8sClient.Create(ctx, svc)).To(Succeed())
			}

			By("Creating the ExternalNamespace")
			ensName := fmt.Sprintf("ens-%s-%s", string(rcl.GetUID()), string(ns.GetUID()))
			ens = &npcv1.ExternalNamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:	   ensName,
					Namespace: rclNS,
					Labels:	   labels,
				},
				Spec: npcv1.ExternalNamespaceSpec{
					RemoteCluster:	 rcl.GetName(),
					RemoteNamespace: ns.GetName(),
				},
			}
			Expect(k8sClient.Create(ctx, ens)).To(Succeed())
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rcl)).To(Succeed())
			
			err = k8sClient.Get(ctx, typeNamespacedNameC, rclCouple)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the couple for RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rclCouple)).To(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameN, ns)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Namespace")
			Expect(k8sClient.Delete(ctx, ns)).To(Succeed())

		})

		It("Should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ServiceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameS,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedNameS, svc)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the specific Service")
			Expect(k8sClient.Delete(ctx, svc)).To(Succeed())

			By("Reconciling the deleted resource")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameS,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
