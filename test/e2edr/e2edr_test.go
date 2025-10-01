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

package e2edr

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Netcracker/qubership-network-policy-operator/test/utils"
)

const namespace = "npcv1-system"

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		By("installing prometheus operator")
		Expect(utils.InstallPrometheusOperator()).To(Succeed())

		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	AfterAll(func() {
		By("uninstalling the Prometheus manager bundle")
		utils.UninstallPrometheusOperator()

		By("removing manager namespace")
		cmd := exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	Context("Operator", func() {
		It("should run successfully", func() {
			var controllerPodName string
			var err error

			// TODO:
			// environment variables:
			// KUBECONFIG_1, KUBECONFIG_2, OP_IMG, JOB_IMG
			// ====Left side====
			// Install CRDs
			// Install operator with Helm. NetworkPolicyConfig disabled

			By("validating that the RemoteClusterConfig has `Coupled` status")
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
			By("validating that the ExternalNamespaces have been created")
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())

			// ====Right side====
			// Install CRDs
			// Install operator with Helm. NetworkPolicyConfig disabled

			By("validating that the RemoteClusterConfig has `Coupled` status")
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
			By("validating that the ExternalNamespaces have been created")
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
			// TODO:
			// Check pod-to-pod connectivity between namespaces (`test`, `test1`)
			// Check pod-to-svc connectivity between namespaces (`test`, `test1`)
			// Enable NetworkPolicyConfig
			// Check pod-to-pod connectivity between namespaces (`test`, `test`)
			// Check pod-to-svc connectivity between namespaces (`test`, `test`)
			// Check pod-to-pod connectivity between namespaces (`test`, `test1`)
			// Check pod-to-svc connectivity between namespaces (`test`, `test1`)
			// Add pod to `test` namespace
			// Check pod-to-pod connectivity between namespaces (`test`, `test`)
			// Check pod-to-svc connectivity between namespaces (`test`, `test`)

		})
	})
})
