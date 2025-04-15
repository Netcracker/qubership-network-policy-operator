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
	crtv1 "k8s.io/api/certificates/v1"
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
			s2NameC		= "remote-cluster2-cert"
			csrName		= "remote-cluster2-cert"
			cmName		= "kube-root-ca.crt"
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
		typeNamespacedNameCRT := types.NamespacedName{
			Name:      csrName,
		}
		typeNamespacedNameSC := types.NamespacedName{
			Name:      s2NameC,
			Namespace: rclNS,
		}
		typeNamespacedNameCM := types.NamespacedName{
			Name:      cmName,
			Namespace: rclNS,
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
		csr := &crtv1.CertificateSigningRequest{}
		sc := &corev1.Secret{}
		req := `-----BEGIN CERTIFICATE REQUEST-----
MIICZjCCAU4CAQAwITEOMAwGA1UEAwwFdXNlcjExDzANBgNVBAoMBmdyb3VwMTCC
ASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMmcQmlufnKxtsEfWwMcqmHI
X8h6NAozC/XyrD/F49aX3dzkxMYz4EpxXnxfOYyBb6Z8bEdDG1tCCBR4ohdzAjQA
2eozP6deTwkj7950zubTMnhkdJhFw9lls9/Ls0llZn/c6vrDEOUl7gKHkXWqJ4QF
RguvTubmdcxgtoAiBmLdEoH6EvuWr8C1Ckv/lTrncMJA0GQFx8rEWiHnGNKfIowC
sGwDf29+ItZiTXxzcIXzi2tbkql+dAoCF5BNsiDO7osmqZ3ViBZL7CH2U0Abq8UY
NGjD+fKn8uTzW8GBGs6VMBpDx4tItOGdokA2EDWcl0XmrGWmA7pNo05G3vpfrxMC
AwEAAaAAMA0GCSqGSIb3DQEBCwUAA4IBAQC2lw/7vVpl472DH8c80j9uvbOTkxQq
OQnS+u0bxC2O+v40nM+8hFqmIV49u9n5iKLEXJFhvJmI5KgupyAc9BXhRpEVe2xx
f7PYv2ZmSemb5dEKvGokmVATNyGpGgwW1FyKoA31EjYM2MGrJmWMUaDbStC3dibR
aSDK11I2oil/y5B68hvquUr09lLPs5TpT9Pkch6AQl8t0wNnhIOUnXOMyY1luv41
PKignC4tNqN4VvGjry3GDBOOqWzRsVTz3/YORT6Y2HYbUhtOTwsSToKVK2etJcoU
SdkWhTI7Js+NZDPy4UjB+sQw1wkuGMNFSHhhkUa8HEg/yK1FHuvjeXON
-----END CERTIFICATE REQUEST-----`
		usage := []crtv1.KeyUsage{crtv1.UsageDigitalSignature,
					crtv1.UsageKeyEncipherment,
					crtv1.UsageClientAuth}
		data := make(map[string][]byte)
		data["tls.key"] = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAyZxCaW5+crG2wR9bAxyqYchfyHo0CjML9fKsP8Xj1pfd3OTE
xjPgSnFefF85jIFvpnxsR0MbW0IIFHiiF3MCNADZ6jM/p15PCSPv3nTO5tMyeGR0
mEXD2WWz38uzSWVmf9zq+sMQ5SXuAoeRdaonhAVGC69O5uZ1zGC2gCIGYt0SgfoS
+5avwLUKS/+VOudwwkDQZAXHysRaIecY0p8ijAKwbAN/b34i1mJNfHNwhfOLa1uS
qX50CgIXkE2yIM7uiyapndWIFkvsIfZTQBurxRg0aMP58qfy5PNbwYEazpUwGkPH
i0i04Z2iQDYQNZyXReasZaYDuk2jTkbe+l+vEwIDAQABAoIBAQCW700o8XfpFfLk
Ej1ozwUfv4s144ciO6UwtEUQWW6BMaD/xqUqXZwFH2rFYeULs7uKwIuvriXE5GJ4
MPfo5kk39YsvMr7s5kX1adoiiQB8/pBCYbKIbXJ6RxGq8i1/PVLHVvT0VyVG6GaP
rj7QAFslE22lQdGoO9Dk+WKn+tEdIV70KfZDgh7r0uGqnJKKQMm7ljucehBdnB+r
TUXCuEJ6Sw9A+ezaSAKMvqIfe30hzlKarnopN6liIxiYinYbjpUEL/LZD5eLBUvc
bwXZxRU2e81Mu2r4PqqrDPvD9/lmJepwovfCstGNVshKqOwmJraCIX9BRQGixA/x
aXtbxxHBAoGBAPqZtsBzBanTZcgiFeSMaB0467a8q3NgVLhlT3Y0JiIV6dWZHWnp
vNcCsJ2nh6fqaE/5t3V+Fg43HLz4v9xsgC5jaPc/iY9fXVFWSfuZpn85VsDltIr3
2IH3pqGhwf0RiR+8XpFuYFK/w93HhACErQCJu4KG7hkIFo37jcS6vgz3AoGBAM30
Ukt+C7A+HI5FeKtYSLUwQackltRa19Ufk79rrwegpPj7VwwxNA9RnyjZkRjj8no3
vXeqoEaiM7re1RgqvsgSV++uYsJaiHgkJmHfa16+Acv1jZ6Luli1PipclgO+TY2Z
OTFoh5rGRK33H7jftou1/ah/vAi1m7UqBIvMqbPFAoGAe6phUgUw9UZ6AtI3RYq4
Ujtlrs7aONx/0mNEXEuNyeJQe3TM3BhZdS1BpPsoaYe2pEjR7OwkzC0MRIDohUkR
1FKAWQsEwjjIXzwa1EtVq5DLK/PvLWjzceeZVkfx6AXASmX1sQTsCcnSSV6M2Iig
qxRM8TPOyZGBN928bynMYa8CgYA2zNGw4KL/jCdQ+8812FHQVDzD2op/ioRvYu2P
O3nhD6+VyFrLFbSCdvdgiTOAafa06mvxkWcAJFybVntg6M7GsYrQbZg9mpVBUm0c
EbSiCvCzcmzySL+WpvMZlnUcHeDI0oJTUTrzxdm0aFL6gBw0ZIJP+Vzw/FGoDLwU
7qog+QKBgAw+nZOaPZp9kQKFR8x2+VhC8QrFyFumrW7nbJPM92LItvsvV5kddvcy
Oads3imNiBoo8gNKK9/YZtQnS8wdM66IawclDXJ0Yn5kkgJRUlX0aCrQ7gy2paqH
yTm84gb6A1e3oe72514r0bim5PnoKg1swg+4HxPirARvZvnmpRw+
-----END RSA PRIVATE KEY-----`)
		cm := &corev1.ConfigMap{}

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
							Secret: s2Name,
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
							Secret: s1Name,
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

			By("Creating the CertificateSigningRequest")
			err = k8sClient.Get(ctx, typeNamespacedNameCRT, csr)
			if err != nil && errors.IsNotFound(err) {
				csr = &crtv1.CertificateSigningRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name: csrName,
					},
					Spec: crtv1.CertificateSigningRequestSpec{
						Groups:     []string{"system:authenticated"},
						SignerName: "kubernetes.io/kube-apiserver-client",
						Username:   "user1",
						Usages:	    usage,
						Request:    []byte(req),
					},
				}
				err = k8sClient.Create(ctx, csr)
				Expect(err).NotTo(HaveOccurred())
				csr.Status.Conditions = append (csr.Status.Conditions,
								crtv1.CertificateSigningRequestCondition{
								Type: crtv1.CertificateApproved,
								Status: corev1.ConditionTrue,
								Reason: "KubectlApprove",
								Message: "This CSR was approved by kubectl certificate approve",
								LastTransitionTime: metav1.Now(),
							})
				_, err = k8sClientSet.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csrName, csr, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the CertificateSigningRequest:", err))
			}

			By("Getting the CA")
			err = k8sClient.Get(ctx, typeNamespacedNameCM, cm)
			Expect(err).NotTo(HaveOccurred())
			data["ca.crt"] = []byte(cm.Data["ca.crt"])

			By("Getting the certificate")
			time.Sleep(5*time.Second)
			err = k8sClient.Get(ctx, typeNamespacedNameCRT, csr)
			Expect(err).NotTo(HaveOccurred())
			data["tls.crt"] = csr.Status.Certificate

			By("Creating the Secret with certificate")
			err = k8sClient.Get(ctx, typeNamespacedNameSC, sc)
			if err != nil && errors.IsNotFound(err) {
				sc = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        s2NameC,
						Namespace:   rclNS,
					},
					Data: data,
					Type: "Opaque",
				}
				Eventually(func() error {
					return k8sClient.Create(ctx, sc)
				}, time.Minute, time.Second).Should(Succeed())
			} else if err != nil {
				Fail(fmt.Sprint("Failed to get the Secret:", err))
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

			err = k8sClient.Get(ctx, typeNamespacedNameCRT, csr)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the CertificateSigningRequest")
			Eventually(func() error {
				return k8sClient.Delete(ctx, csr)
			}, time.Minute, time.Second).Should(Succeed())

			err = k8sClient.Get(ctx, typeNamespacedNameSC, sc)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the Secret with certificate")
			Eventually(func() error {
				return k8sClient.Delete(ctx, sc)
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

			By("Getting the RemoteClusterConfig")
			err = k8sClient.Get(ctx, typeNamespacedName, rcl)
			Expect(err).NotTo(HaveOccurred())

			By("Updating Remote Secret in RemoteClusterConfig")
			rcl.Spec.RemoteCluster.Secret = s2NameC
			Eventually(func() error {
				return k8sClient.Update(ctx, rcl)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the RemoteClusterConfig")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
