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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Netcracker/qubership-network-policy-operator/test/utils"
)

const (
	testns1 = "testns1"
	testns2 = "testns2"
)

var img, rbacImg, curlImg, chart, namespace, key, user string
var conf1, conf2, values1, values2 string
var rclcName1, rclcName2, secret1, secret2 string

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

		conf1 = os.Getenv("KUBECONF1")
		if len(namespace) == 0 {
			conf1 = "./kubeconfig1"
		}
		conf2 = os.Getenv("KUBECONF2")
		if len(namespace) == 0 {
			conf2 = "./kubeconfig2"
		}

		values1 = os.Getenv("VALUES1")
		if len(values1) == 0 {
			values1 = "./values1.yaml"
		}
		values2 = os.Getenv("VALUES2")
		if len(values2) == 0 {
			values2 = "./values2.yaml"
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

		By("Installing operator with Helm on cluster #1")
		cmd = exec.Command("helm", "install", "network-policy-operator", chart, "-n", namespace,
			"--set", "networkPolicies.enabled=false",
			"--values", values1, fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Installing operator with Helm on cluster #2")
		cmd = exec.Command("helm", "install", "network-policy-operator", chart, "-n", namespace,
			"--set", "networkPolicies.enabled=false",
			"--values", values2, fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("Uninstalling operator with Helm cluster #1")
		cmd := exec.Command("helm", "uninstall", "network-policy-operator", "-n", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err := utils.Run(cmd)

		By("Uninstalling operator with Helm cluster #2")
		cmd = exec.Command("helm", "uninstall", "network-policy-operator", "-n", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)

		By("Deleting finalizer from RemoteClusterConfig on cluster #1")
		cmd = exec.Command("kubectl", "patch", "rclc", rclcName1, "--type", "json",
			"--patch-file", "./test/yamls/rclc_patch.json", "-n", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)

		By("Deleting finalizer from RemoteClusterConfig on cluster #2")
		cmd = exec.Command("kubectl", "patch", "rclc", rclcName2, "--type", "json",
			"--patch-file", "./test/yamls/rclc_patch.json", "-n", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)

		By("Deleting CRDs on cluster #1")
		cmd = exec.Command("kubectl", "delete", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
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
			By("Getting RemoteClusterConfig names")
			cmd := exec.Command("kubectl", "get",
				"rclc", "-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf1))

			output, err := utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			rclcName1 = utils.GetNonEmptyLines(string(output))[0]

			cmd = exec.Command("kubectl", "get",
				"rclc", "-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf2))

			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			rclcName2 = utils.GetNonEmptyLines(string(output))[0]

			By("Getting Secrets names")
			cmd = exec.Command("kubectl", "get",
				"secret", "-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf1))

			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			secret1 = utils.GetNonEmptyLines(string(output))[0]
			By(fmt.Sprintf("Secret name: '%s'", secret1))

			cmd = exec.Command("kubectl", "get",
				"secret", "-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf2))

			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			secret2 = utils.GetNonEmptyLines(string(output))[0]
			By(fmt.Sprintf("Secret name: '%s'", secret2))

			By("Copy Secret from cluster #1 to cluster #2")
			cmd = exec.Command("kubectl", "get", "secret", secret1, "-o", "yaml",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf1))
			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			secret := utils.GetNonEmptyLines(string(output))
			err = os.WriteFile("/tmp/secret.yaml", []byte(strings.Join(secret, "\n")), 0666)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			cmd = exec.Command("kubectl", "apply", "-f", "/tmp/secret.yaml",
				fmt.Sprintf("--kubeconfig=%s", conf1))
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			By("Copy Secret from cluster #2 to cluster #1")
			cmd = exec.Command("kubectl", "get", "secret", secret1, "-o", "yaml",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			secret = utils.GetNonEmptyLines(string(output))
			err = os.WriteFile("/tmp/secret.yaml", []byte(strings.Join(secret, "\n")), 0666)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			cmd = exec.Command("kubectl", "apply", "-f", "/tmp/secret.yaml",
				fmt.Sprintf("--kubeconfig=%s", conf2))
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			verifyClusterCoupled := func(conf string) error {
				cmd = exec.Command("kubectl", "get",
					"rclc", rclcName1, "-o", "jsonpath={.status.conditions[*].type}",
					"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf))
				statusOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				status := utils.GetNonEmptyLines(string(statusOutput))[0]
				ExpectWithOffset(2, status).Should(ContainSubstring("Coupled"))
				return nil
			}
			By("Verifing the RemoteClusterConfig status on cluster #1")
			EventuallyWithOffset(1, verifyClusterCoupled(conf1), time.Minute, time.Second).Should(Succeed())
			By("The cluster #1 is coupled")

			By("Verifing the RemoteClusterConfig status on cluster #2")
			EventuallyWithOffset(1, verifyClusterCoupled(conf2), time.Minute, time.Second).Should(Succeed())
			By("The cluster #2 is coupled")
			// TODO:
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
