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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
)

var _ = Describe("RemoteClusterConfig Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			rclName 	= "test-remote-cluster-config"
			rclCoupleName 	= "couple-remote-cluster-config"
			rclNS   	= "default"
			s1Name		= "remote-cluster1"
			s2Name		= "remote-cluster2"
			roleName	= "npo-role"
			rbName		= "npo-rb"
		)

		var accessor metav1.Object
		var rclUID types.UID
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
		rclCouple := &npcv1.RemoteClusterConfig{}
		sa1 := &corev1.ServiceAccount{}
		sa2 := &corev1.ServiceAccount{}
		s1 := &corev1.Secret{}
		s2 := &corev1.Secret{}
		role := &rbac.ClusterRole{}
		rb := &rbac.ClusterRoleBinding{}
		sl :=[]string{"*"}
		ensList := &npcv1.ExternalNamespaceList{}
		owns := []metav1.OwnerReference{}

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
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, rcl)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the RemoteClusterConfig:", err))
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
				Eventually(func() error {
					return k8sClient.Create(ctx, rclCouple)
				}, time.Minute, time.Second).Should(Succeed())
				meta.SetStatusCondition(&rclCouple.Status.Conditions,
							metav1.Condition{
								Type: TypeCoupled,
								Status: metav1.ConditionTrue,
								Reason: "Initializing",
								Message: "Couple is identified",
							})
				rclCouple.Status.ServiceSubnet  = "172.31.0.0/16"
				rclCouple.Status.KubeAPIService = "172.31.0.1"
				rclCouple.Status.CoreDNSService = "172.31.0.10"
				Eventually(func() error {
					return k8sClient.Status().Update(ctx, rclCouple)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the couple for RemoteClusterConfig:", err))
			}

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
				Eventually(func() error {
					return k8sClient.Create(ctx, sa1)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the ServiceAccount:", err))
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
				Eventually(func() error {
					return k8sClient.Create(ctx, s1)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the Secret:", err))
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
				Eventually(func() error {
					return k8sClient.Create(ctx, sa2)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the ServiceAccount:", err))
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
				Eventually(func() error {
					return k8sClient.Create(ctx, s2)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the Secret:", err))
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
				Eventually(func() error {
					return k8sClient.Create(ctx, role)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the ClusterRole:", err))
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
				Eventually(func() error {
					return k8sClient.Create(ctx, rb)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the ClusterRoleBinding:", err))
			}
		})

		AfterEach(func() {
			err := k8sClient.Get(ctx, typeNamespacedName, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the RemoteClusterConfig")
			Eventually(func() error {
				return k8sClient.Delete(ctx, rcl)
			}, time.Minute, time.Second).Should(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameC, rclCouple)
			Expect(err).NotTo(HaveOccurred())
			accessor, err = meta.Accessor(rclCouple)
			rclUID = accessor.GetUID()

			By("Cleanup the couple for RemoteClusterConfig")
			Eventually(func() error {
				return k8sClient.Delete(ctx, rclCouple)
			}, time.Minute, time.Second).Should(Succeed())


			err = k8sClient.Get(ctx, typeNamespacedNameRB, rb)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedNameR, role)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the ClusterRole and ClusterRoleBinding")
			Eventually(func() error {
				return k8sClient.Delete(ctx, role)
			}, time.Minute, time.Second).Should(Succeed())
			Eventually(func() error {
				return k8sClient.Delete(ctx, rb)
			}, time.Minute, time.Second).Should(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameS1, sa1)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedNameS1, s1)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedNameS2, sa2)
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, typeNamespacedNameS2, s2)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the ServiceAccounts and Secrets")
			Eventually(func() error {
				return k8sClient.Delete(ctx, sa1)
			}, time.Minute, time.Second).Should(Succeed())
			Eventually(func() error {
				return k8sClient.Delete(ctx, sa2)
			}, time.Minute, time.Second).Should(Succeed())

			time.Sleep(15*time.Second)

			// It's a workaround to fix the cascading deletion
			err = k8sClient.List(ctx, ensList)
			Expect(err).NotTo(HaveOccurred())
			if len(ensList.Items) > 0 {
				By("Cleanup the ExternalNamespace")
				for _, ensItem := range ensList.Items {
					accessor, err = meta.Accessor(&ensItem)
					owns = accessor.GetOwnerReferences()
					for _, own := range owns{
						if own.UID == rclUID {
							Eventually(func() error {
								return k8sClient.Delete(ctx, &ensItem)
							}, time.Minute, time.Second).Should(Succeed())
						}
					}
				}
			}

			time.Sleep(30*time.Second)
		})
		It("should successfully reconcile the resource", func() {
			controllerReconciler := &RemoteClusterConfigReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling the RemoteClusterConfig")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
