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

package ns

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	npcv1 "github.com/Netcracker/qubership-network-policy-operator/api/v1alpha1"
)

var _ = Describe("Namespace Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			rclName       = "test-remote-cluster-config"
			rclCoupleName = "couple-remote-cluster-config"
			nsName        = "test-ns-npo"
			podName       = "test-pod"
			svcName       = "test-svc"
			rclNS         = "default"
			s1Name        = "remote-cluster1"
			s2Name        = "remote-cluster2"
			roleName      = "npo-role"
			rbName        = "npo-rb"
			cniPodName    = "aws-node-1"
			cniPodNS      = "kube-system"
		)

		var accessor metav1.Object
		var num int
		ctx := context.Background()
		ann := make(map[string]string)
		args := []string{"sleep", "infinity"}
		label := map[string]string{
			"remote": "true",
		}
		ensLabels := make(map[string]string)
		cniLabel := map[string]string{
			"k8s-app": "aws-node",
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
		typeNamespacedNameS1 := types.NamespacedName{
			Name:      s1Name,
			Namespace: rclNS,
		}
		typeNamespacedNameS2 := types.NamespacedName{
			Name:      s2Name,
			Namespace: rclNS,
		}
		typeNamespacedNameR := types.NamespacedName{
			Name: roleName,
		}
		typeNamespacedNameRB := types.NamespacedName{
			Name: rbName,
		}
		typeNamespacedNameP := types.NamespacedName{
			Name:      podName,
			Namespace: nsName,
		}
		typeNamespacedNameS := types.NamespacedName{
			Name:      svcName,
			Namespace: nsName,
		}
		typeNamespacedNameCNIP := types.NamespacedName{
			Name:      cniPodName,
			Namespace: cniPodNS,
		}
		rcl := &npcv1.RemoteClusterConfig{}
		ensList := &npcv1.ExternalNamespaceList{}
		spec := &npcv1.ExternalNamespaceSpec{}
		ns := &corev1.Namespace{}
		sa1 := &corev1.ServiceAccount{}
		sa2 := &corev1.ServiceAccount{}
		s1 := &corev1.Secret{}
		s2 := &corev1.Secret{}
		role := &rbac.ClusterRole{}
		rb := &rbac.ClusterRoleBinding{}
		sl := []string{"*"}
		pod := &corev1.Pod{}
		cniPod := &corev1.Pod{}
		svc := &corev1.Service{}
		pssLabel := map[string]string{"pod-security.kubernetes.io/enforce": "privileged"}

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
							Message:            "Couple is identified",
							Reason:             "Initializing",
							Status:             "True",
							Type:               "Coupled",
						}},
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
				Eventually(func() error {
					return k8sClient.Create(ctx, rcl)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the couple for RemoteClusterConfig:", err))
			}

			By("Creating the Namespace")
			err = k8sClient.Get(ctx, typeNamespacedNameN, ns)
			if err != nil && errors.IsNotFound(err) {
				ns = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   nsName,
						Labels: pssLabel,
					},
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, ns)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the Namespace:", err))
			}

			err = k8sClient.Get(ctx, typeNamespacedNameP, pod)
			if err == nil {
				By("Deleting the Pod before the test")
				Eventually(func() error {
					return k8sClient.Delete(ctx, pod)
				}, time.Minute, time.Second).Should(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedNameS, svc)
			if err == nil {
				By("Deleting the Service before the test")
				Eventually(func() error {
					return k8sClient.Delete(ctx, svc)
				}, time.Minute, time.Second).Should(Succeed())
			}

			By("Creating the ServiceAccounts and Secrets")
			err = k8sClient.Get(ctx, typeNamespacedNameS1, sa1)
			if err != nil && errors.IsNotFound(err) {
				sa1 = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      s1Name,
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
						Verbs:     sl,
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
						Kind:      "ServiceAccount",
						Name:      s1Name,
						Namespace: rclNS,
					}, {
						Kind:      "ServiceAccount",
						Name:      s2Name,
						Namespace: rclNS,
					}},
					RoleRef: rbac.RoleRef{
						Kind:     "ClusterRole",
						APIGroup: "rbac.authorization.k8s.io",
						Name:     roleName,
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

			err = k8sClient.Get(ctx, typeNamespacedNameC, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the couple for RemoteClusterConfig")
			Eventually(func() error {
				return k8sClient.Delete(ctx, rcl)
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

			err = k8sClient.Get(ctx, typeNamespacedNameCNIP, cniPod)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the AWS-CNI Pod")
			Eventually(func() error {
				return k8sClient.Delete(ctx, cniPod)
			}, time.Minute, time.Second).Should(Succeed())
		})

		It("Should successfully reconcile the resource", func() {
			By("Reconciling the created Namespace")
			controllerReconciler := &NamespaceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Creating the Pod resource")
			err = k8sClient.Get(ctx, typeNamespacedNameP, pod)
			if err != nil && errors.IsNotFound(err) {
				pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: nsName,
						Labels:    label,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:            podName,
							Image:           "debian:bookworm-slim",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            args,
						}},
					},
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, pod)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the Pod:", err))
			}

			By("Reconciling the updated Namespace")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})

			By("Creating the Service resource")
			err = k8sClient.Get(ctx, typeNamespacedNameS, svc)
			if err != nil && errors.IsNotFound(err) {
				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      svcName,
						Namespace: nsName,
						Labels:    label,
					},
					Spec: corev1.ServiceSpec{
						Selector:  label,
						ClusterIP: "172.30.10.10",
						Ports: []corev1.ServicePort{{
							Port:     5000,
							Name:     "random",
							Protocol: corev1.ProtocolTCP,
							TargetPort: intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 5000,
							},
						}},
					},
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, svc)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the Pod:", err))
			}

			By("Reconciling the updated Namespace")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the ExternalNamespace spec after creation")
			err = k8sClient.List(ctx, ensList)
			Expect(err).NotTo(HaveOccurred())
			for _, ensItem := range ensList.Items {
				accessor, err = meta.Accessor(&ensItem)
				ensLabels = accessor.GetLabels()
				num = 0
				if ensLabels["kubernetes.io/metadata.name"] == nsName {
					num++
					spec = &ensItem.Spec
				}
			}
			if num > 1 {
				Fail("The ExternalNamespace must be only one")
			}
			if len(spec.Resources.Pods) > 1 {
				Fail("The Pod in the ExternalNamespace must be only one")
			}
			if len(spec.Resources.Services) > 1 {
				Fail("The Service in the ExternalNamespace must be only one")
			}

			err = k8sClient.Get(ctx, typeNamespacedNameP, pod)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the specific Pod")
			Eventually(func() error {
				return k8sClient.Delete(ctx, pod)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the deleted Pod")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedNameS, svc)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the specific Service")
			Eventually(func() error {
				return k8sClient.Delete(ctx, svc)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the deleted Service")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the ExternalNamespace spec after deletion")
			err = k8sClient.List(ctx, ensList)
			Expect(err).NotTo(HaveOccurred())
			for _, ensItem := range ensList.Items {
				accessor, err = meta.Accessor(&ensItem)
				ensLabels = accessor.GetLabels()
				num = 0
				if ensLabels["kubernetes.io/metadata.name"] == nsName {
					num++
					spec = &ensItem.Spec
				}
			}
			if num > 1 {
				Fail("The ExternalNamespace must be only one")
			}
			if len(spec.Resources.Pods) > 0 {
				Fail("The Pod in the ExternalNamespace has not been deleted")
			}
			if len(spec.Resources.Services) > 0 {
				Fail("The Service in the ExternalNamespace has not been deleted")
			}

			err = k8sClient.Get(ctx, typeNamespacedNameN, ns)
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the specific Namespace")
			Eventually(func() error {
				return k8sClient.Delete(ctx, ns)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the deleted Namespace")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling the EKS CNI")
			// Create Pod with `k8s-app=aws-node` label
			By("Creating the AWS-CNI Pod")
			err = k8sClient.Get(ctx, typeNamespacedNameCNIP, cniPod)
			if err != nil && errors.IsNotFound(err) {
				cniPod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cniPodName,
						Namespace: cniPodNS,
						Labels:    cniLabel,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:            podName,
							Image:           "debian:bookworm-slim",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            args,
						}},
					},
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, cniPod)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the Pod:", err))
			}
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedNameN,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
