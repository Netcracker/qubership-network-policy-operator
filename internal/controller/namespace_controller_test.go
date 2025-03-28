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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
)

var _ = Describe("Namespace Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			rclName	      = "test-remote-cluster-config"
			rclCoupleName = "couple-remote-cluster-config"
			nsName	      = "test-ns-nsc"
			rclNS	      = "default"
			s1Name	      = "remote-cluster1"
			s2Name	      = "remote-cluster2"
			roleName      = "npo-role"
			rbName	      = "npo-rb"
		)


		ctx := context.Background()
		ann := make(map[string]string)

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
		typeNamespacedNameS1 := types.NamespacedName{
			Name:      s1Name,
			Namespace: rclNS,
		}
		typeNamespacedNameS2 := types.NamespacedName{
			Name:      s2Name,
			Namespace: rclNS,
		}
		typeNamespacedNameR := types.NamespacedName{
			Name:      roleName,
		}
		typeNamespacedNameRB := types.NamespacedName{
			Name:      rbName,
		}
		rcl := &npcv1.RemoteClusterConfig{}
		ensList := &npcv1.ExternalNamespaceList{}
		ns := &corev1.Namespace{}
		sa1 := &corev1.ServiceAccount{}
		sa2 := &corev1.ServiceAccount{}
		s1 := &corev1.Secret{}
		s2 := &corev1.Secret{}
		role := &rbac.ClusterRole{}
		rb := &rbac.ClusterRoleBinding{}
		sl :=[]string{"*"}

		BeforeEach(func() {
			By("Creating the RemoteClusterConfig")
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
					Status: npcv1.RemoteClusterConfigStatus{
						Conditions: []metav1.Condition{{
							LastTransitionTime: metav1.Now(),
							Message: "Couple is identified",
							Reason: "Initializing",
							Status: "True",
							Type: "Coupled",
						}},
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
			err = k8sClient.Get(ctx, typeNamespacedNameC, rcl)
			if err != nil && errors.IsNotFound(err) {
				rcl = &npcv1.RemoteClusterConfig{
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
				Expect(k8sClient.Create(ctx, rcl)).To(Succeed())
			}
			By("Creating the Namespace resource")
			ns = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nsName,
					},
				}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			By("Creating the ServiceAccounts and Secrets")
			err = k8sClient.Get(ctx, typeNamespacedNameS1, sa1)
			if err != nil && errors.IsNotFound(err) {
				sa1 = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:	   s1Name,
						Namespace: rclNS,
					},
					Secrets: []corev1.ObjectReference{{
						Name: s1Name,
					}},
				}
				Expect(k8sClient.Create(ctx, sa1)).To(Succeed())
			}
			err = k8sClient.Get(ctx, typeNamespacedNameS1, s1)
			if err != nil && errors.IsNotFound(err) {
				ann["kubernetes.io/service-account.name"] = s1Name
				s1 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        s1Name,
						Namespace:   rclNS,
						Annotations: ann,
					},
					Type: "kubernetes.io/service-account-token",
				}
				Expect(k8sClient.Create(ctx, s1)).To(Succeed())
			}
			err = k8sClient.Get(ctx, typeNamespacedNameS2, sa2)
			if err != nil && errors.IsNotFound(err) {
				sa2 = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      s2Name,
						Namespace: rclNS,
					},
					Secrets: []corev1.ObjectReference{{
						Name: s2Name,
					}},
				}
				Expect(k8sClient.Create(ctx, sa2)).To(Succeed())
			}
			err = k8sClient.Get(ctx, typeNamespacedNameS2, s2)
			if err != nil && errors.IsNotFound(err) {
				ann["kubernetes.io/service-account.name"] = s2Name
				s2 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        s2Name,
						Namespace:   rclNS,
						Annotations: ann,
					},
					Type: "kubernetes.io/service-account-token",
				}
				Expect(k8sClient.Create(ctx, s2)).To(Succeed())
			}

			By("Creating the ClusterRole and ClusterRoleBinding")
			err = k8sClient.Get(ctx, typeNamespacedNameR, role)
			if err != nil && errors.IsNotFound(err) {
				role = &rbac.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: roleName,
					},
					Rules: []rbac.PolicyRule{{
						Verbs:	   sl,
						APIGroups: sl,
						Resources: sl,
					}},
				}
				Expect(k8sClient.Create(ctx, role)).To(Succeed())
			}
			err = k8sClient.Get(ctx, typeNamespacedNameRB, rb)
			if err != nil && errors.IsNotFound(err) {
				rb := &rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: rbName,
					},
					Subjects: []rbac.Subject{{
						Kind:	   "ServiceAccount",
						Name:	   s1Name,
						Namespace: rclNS,
					}, {
						Kind:	   "ServiceAccount",
						Name:	   s2Name,
						Namespace: rclNS,
					}},
					RoleRef: rbac.RoleRef{
						Kind:	  "ClusterRole",
						APIGroup: "rbac.authorization.k8s.io",
						Name:	  roleName,
					},
				}
				Expect(k8sClient.Create(ctx, rb)).To(Succeed())
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rcl)).To(Succeed())

			err = k8sClient.List(ctx, ensList)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.List(ctx, ensList)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedNameC, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the couple for RemoteClusterConfig")
			Expect(k8sClient.Delete(ctx, rcl)).To(Succeed())

			By("Cleanup the Namespace")
			Expect(k8sClient.Delete(ctx, ns)).To(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameRB, rb)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedNameR, role)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the ClusterRole and ClusterRoleBinding")
			Expect(k8sClient.Delete(ctx, role)).To(Succeed())
			Expect(k8sClient.Delete(ctx, rb)).To(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameS1, sa1)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedNameS1, s1)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedNameS2, sa2)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedNameS2, s2)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the ServiceAccounts and Secrets")
			Expect(k8sClient.Delete(ctx, sa1)).To(Succeed())
			Expect(k8sClient.Delete(ctx, sa2)).To(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &NamespaceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the updated resource")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})

			ns := &corev1.Namespace{}
			err = k8sClient.Get(ctx, typeNamespacedNameN, ns)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the specific Namespace")
			Expect(k8sClient.Delete(ctx, ns)).To(Succeed())

			By("Reconciling the deleted resource")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
