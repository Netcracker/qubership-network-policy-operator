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
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Netcracker/qubership-network-policy-operator/test/utils"
)

const (
	testns1 = "testns1"
	testns2 = "testns2"
)

var img, rbacImg, curlImg, chart, namespace, key, user string
var conf1, conf2 string

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		img = os.Getenv("OP_IMG")
		if len(img) == 0 {
			img = "ghcr.io/netcracker/qubership-network-policy-operator:main"
		}
		rbacImg = os.Getenv("RP_IMG")
		if len(rbacImg) == 0 {
			rbacImg = "bitnami/kube-rbac-proxy"
		}
		curlImg = os.Getenv("CURL_IMG")
		if len(curlImg) == 0 {
			curlImg = "alpine/curl"
		}
		chart = os.Getenv("CHART")
		if len(chart) == 0 {
			chart = "./charts/network-policy-operator/"
		}
		namespace = os.Getenv("NS")
		if len(namespace) == 0 {
			namespace = "network-policy-operator"
		}
		key = os.Getenv("SSH_KEY")
		if len(namespace) == 0 {
			namespace = "./id_rsa"
		}
		user = os.Getenv("SSH_USER")
		if len(namespace) == 0 {
			namespace = "ubuntu"
		}

		conf1 = os.Getenv("KUBECONF1")
		if len(namespace) == 0 {
			namespace = "./kubeconfig1"
		}

		conf2 = os.Getenv("KUBECONF2")
		if len(namespace) == 0 {
			namespace = "./kubeconfig2"
		}

		By("Creating manager namespace on cluster #1")
		cmd := exec.Command("kubectl", "create", "ns", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Creating manager namespace on cluster #1")
		cmd = exec.Command("kubectl", "create", "ns", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Creating test namespaces on cluster #1")
		cmd = exec.Command("kubectl", "create", "ns", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		cmd = exec.Command("kubectl", "create", "ns", testns2,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Creating test namespaces on cluster #2")
		cmd = exec.Command("kubectl", "create", "ns", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Creating echo pod in `%s` namespace on cluster #1", testns1))
		cmd = exec.Command("kubectl", "apply", "-f", "./test/yamls/echo.yaml", "-n", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Creating echo pod in `%s` namespace on cluster #1", testns2))
		cmd = exec.Command("kubectl", "apply", "-f", "./test/yamls/echo.yaml", "-n", testns2,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Installing CRDs on cluster #1")
		cmd = exec.Command("kubectl", "apply", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)

		By("Installing CRDs on cluster #2")
		cmd = exec.Command("kubectl", "apply", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)

	})

	AfterAll(func() {

		// TODO:
		// Delete finalizer

		By("Deleting CRDs on cluster #1")
		cmd := exec.Command("kubectl", "delete", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Deleting CRDs on cluster #2")
		cmd = exec.Command("kubectl", "delete", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Removing manager namespace on cluster #1")
		cmd = exec.Command("kubectl", "delete", "ns", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, _ = utils.Run(cmd)

		By("Removing manager namespace on cluster #2")
		cmd = exec.Command("kubectl", "delete", "ns", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, _ = utils.Run(cmd)

		By("Removing test namespaces on cluster #1")
		cmd = exec.Command("kubectl", "delete", "ns", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		cmd = exec.Command("kubectl", "delete", "ns", testns2,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Removing test namespaces on cluster #2")
		cmd = exec.Command("kubectl", "delete", "ns", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	Context("Operator", func() {
		It("Should run successfully", func() {
			// TODO:
			// ====Left side====
			// Install operator with Helm. NetworkPolicyConfig disabled
			// ====Right side====
			// Install operator with Helm. NetworkPolicyConfig disabled

			// Check pod-to-pod connectivity between namespaces (`test1`, `test1`)
			// Check pod-to-pod connectivity between namespaces (`test1`, `test2`)
			// Check pod-to-svc connectivity between namespaces (`test1`, `test1`)
			// Check pod-to-svc connectivity between namespaces (`test1`, `test2`)
			// Enable NetworkPolicyConfig
			// Check pod-to-pod connectivity between namespaces (`test1`, `test1`)
			// Check pod-to-svc connectivity between namespaces (`test1`, `test2`)
			// Check pod-to-pod connectivity between namespaces (`test1`, `test1`)
			// Check pod-to-svc connectivity between namespaces (`test1`, `test2`)
			// Add pod to `test` namespace
			// Check pod-to-pod connectivity between namespaces (`test1`, `test1`)
			// Check pod-to-pod connectivity between namespaces (`test1`, `test2`)
			// Check pod-to-svc connectivity between namespaces (`test1`, `test1`)
			// Check pod-to-svc connectivity between namespaces (`test1`, `test2`)

		})
	})
})
