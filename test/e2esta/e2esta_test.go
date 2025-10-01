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

package e2esta

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Netcracker/qubership-network-policy-operator/test/utils"
)

const (
	testns1 = "testns1"
	testns2 = "testns2"
)

var img, rbacImg, chart, namespace string

var _ = Describe("Controller", Ordered, func() {
	BeforeAll(func() {
		rbacImg = os.Getenv("RP_IMG")
		img = os.Getenv("OP_IMG")
		if len(img) == 0 {
			img = "ghcr.io/netcracker/qubership-network-policy-operator:main"
		}
		chart = os.Getenv("CHART")
		if len(chart) == 0 {
			chart = "./charts/network-policy-operator/"
		}
		namespace = os.Getenv("NS")
		if len(namespace) == 0 {
			namespace = "network-policy-operator"
		}

		By("Creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Creating test namespaces")
		cmd = exec.Command("kubectl", "create", "ns", testns1)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		cmd = exec.Command("kubectl", "create", "ns", testns2)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Installing CRDs")
		cmd = exec.Command("kubectl", "apply", "-f", fmt.Sprintf("%s/crds/", chart))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Installing operator with Helm")
		cmd = exec.Command("helm", "install", "network-policy-operator", chart, "-n", namespace, "--set", "networkPolicies.enabled=false", "--set", fmt.Sprintf("manager.image=%s", img), "--set", fmt.Sprintf("authProxy.image.repository=%s", rbacImg))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("Uninstalling operator with Helm")
		cmd := exec.Command("helm", "uninstall", "network-policy-operator", "-n", namespace)
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Deleting CRDs")
		cmd = exec.Command("kubectl", "delete", "-f", fmt.Sprintf("%s/crds/", chart))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Removing test namespaces")
		cmd = exec.Command("kubectl", "delete", "ns", testns1)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		cmd = exec.Command("kubectl", "delete", "ns", testns2)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	Context("Operator", func() {
		It("Should run successfully", func() {
			var controllerPodName string

			By("Validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name

				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "app.kubernetes.io/name=network-policy-operator",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("network-policy-operator"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				statusOutput, err := utils.Run(cmd)
				status := utils.GetNonEmptyLines(string(statusOutput))[0]
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())

			By(fmt.Sprintf("Checking connectivity between `%s` and `%s` namespaces", testns1, testns2))
			By(fmt.Sprintf("NetworkPolicyConfig is enabled, so all of the connections between `%s` and `%s` must be ALLOWED", testns1, testns2))

			By("Enabling NetworkPolicyConfig")
			By(fmt.Sprintf("Checking connectivity between `%s` and `%s` namespaces", testns1, testns2))
			By(fmt.Sprintf("NetworkPolicyConfig is enabled, so all of the connections between `%s` and `%s` must be REJECTED", testns1, testns2))

			By(fmt.Sprintf("Changing `%s` namespace annotation", testns1))
			By(fmt.Sprintf("Checking connectivity between `%s` and `%s` namespaces", testns1, testns2))
			By(fmt.Sprintf("NetworkPolicyConfig is enabled and `%s` namespace has been annotated to allow the traffic from `%s`", testns1, testns2))

			By("Enabling logs for rejected connection")
			By(fmt.Sprintf("Changing `%s` namespace annotation", testns1))
			By(fmt.Sprintf("Checking connectivity between `%s` and `%s` namespaces", testns1, testns2))
			By("Checking logs")

		})
	})
})
