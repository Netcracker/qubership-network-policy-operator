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
	"slices"
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

var img, curlImg, chart, namespace, key, user string
var conf1, conf2, values1, values2 string
var rclcName1, rclcName2, secret1, secret2 string
var rclcName, conf, testns string
var num int

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		img = os.Getenv("OP_IMG")
		if len(img) == 0 {
			img = "ghcr.io/netcracker/qubership-network-policy-operator:main"
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

		By("\033[35mCreating manager namespace on cluster #1")
		cmd := exec.Command("kubectl", "create", "ns", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mCreating manager namespace on cluster #1")
		cmd = exec.Command("kubectl", "create", "ns", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mCreating test namespaces on cluster #1")
		cmd = exec.Command("kubectl", "create", "ns", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		cmd = exec.Command("kubectl", "create", "ns", testns2,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mCreating test namespaces on cluster #2")
		cmd = exec.Command("kubectl", "create", "ns", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By(fmt.Sprintf("\033[35mCreating echo pod in `%s` namespace on cluster #1", testns1))
		cmd = exec.Command("kubectl", "apply", "-f", "./test/yamls/echo.yaml", "-n", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		By(fmt.Sprintf("\033[35mCreating echo pod in `%s` namespace on cluster #1", testns2))
		cmd = exec.Command("kubectl", "apply", "-f", "./test/yamls/echo.yaml", "-n", testns2,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mInstalling CRDs on cluster #1")
		cmd = exec.Command("kubectl", "apply", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)

		By("\033[35mInstalling CRDs on cluster #2")
		cmd = exec.Command("kubectl", "apply", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)

		By("\033[35mInstalling operator with Helm on cluster #1")
		cmd = exec.Command("helm", "install", "network-policy-operator", chart, "-n", namespace,
			"--set", "networkPolicies.enabled=false",
			"--set", fmt.Sprintf("manager.image=%s", img),
			"--set", "args={\"--health-probe-bind-address=:8081\", \"--metrics-bind-address=:8443\", \"--metrics-secure=true\", \"--leader-elect\", \"--zap-log-level=info\"}",
			"--values", values1, fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mInstalling operator with Helm on cluster #2")
		cmd = exec.Command("helm", "install", "network-policy-operator", chart, "-n", namespace,
			"--set", "networkPolicies.enabled=false",
			"--set", fmt.Sprintf("manager.image=%s", img),
			"--set", "args={\"--health-probe-bind-address=:8081\", \"--metrics-bind-address=:8443\", \"--metrics-secure=true\", \"--leader-elect\", \"--zap-log-level=info\"}",
			"--values", values2, fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("\033[35mDeleting NetworkPolicyConfig from cluster #1")
		cmd := exec.Command("kubectl", "delete", "npc", "--all",
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		By(fmt.Sprintf("\033[33mWaiting for GC"))
		time.Sleep(15 * time.Second)
		By("\033[35mDeleting NetworkPolicies in case if GC is not working on cluster #1")
		cmd = exec.Command("kubectl", "delete", "cnp", "-A", "--all",
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mUninstalling operator with Helm cluster #1")
		cmd = exec.Command("helm", "uninstall", "network-policy-operator", "-n", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)

		By("\033[35mDeleting NetworkPolicyConfig from cluster #2")
		cmd = exec.Command("kubectl", "delete", "npc", "--all",
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		By(fmt.Sprintf("\033[33mWaiting for GC"))
		time.Sleep(15 * time.Second)
		By("\033[35mDeleting NetworkPolicies in case if GC is not working on cluster #2")
		cmd = exec.Command("kubectl", "delete", "cnp", "-A", "--all",
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mUninstalling operator with Helm cluster #2")
		cmd = exec.Command("helm", "uninstall", "network-policy-operator", "-n", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)

		By("\033[35mDeleting finalizer from RemoteClusterConfig on cluster #1")
		cmd = exec.Command("kubectl", "patch", "rclc", rclcName1, "--type", "json", "-p",
			`[{"op": "remove", "path": "/metadata/finalizers"}]`, "-n", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)

		By("\033[35mDeleting finalizer from RemoteClusterConfig on cluster #2")
		cmd = exec.Command("kubectl", "patch", "rclc", rclcName2, "--type", "json", "-p",
			`[{"op": "remove", "path": "/metadata/finalizers"}]`, "-n", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)

		By("\033[35mDeleting CRDs on cluster #1")
		cmd = exec.Command("kubectl", "delete", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mDeleting CRDs on cluster #2")
		cmd = exec.Command("kubectl", "delete", "-f", fmt.Sprintf("%s/crds/", chart),
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mRemoving manager namespace on cluster #1")
		cmd = exec.Command("kubectl", "delete", "ns", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, _ = utils.Run(cmd)

		By("\033[35mRemoving manager namespace on cluster #2")
		cmd = exec.Command("kubectl", "delete", "ns", namespace,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, _ = utils.Run(cmd)

		By("\033[35mRemoving test namespaces on cluster #1")
		cmd = exec.Command("kubectl", "delete", "ns", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		cmd = exec.Command("kubectl", "delete", "ns", testns2,
			fmt.Sprintf("--kubeconfig=%s", conf1))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("\033[35mRemoving test namespaces on cluster #2")
		cmd = exec.Command("kubectl", "delete", "ns", testns1,
			fmt.Sprintf("--kubeconfig=%s", conf2))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	Context("Operator", func() {
		It("Should run successfully", func() {
			var echoPodName1, echoPodName2, echoPodIP1, echoPodIP2 string
			var echoSvcName, echoSvcIP1, echoSvcIP2 string
			curlParam := make(map[string]string)

			// Validating installation
			By("\033[32mValidating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "app.kubernetes.io/name=network-policy-operator",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf))

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				ExpectWithOffset(2, podNames[0]).Should(ContainSubstring("network-policy-operator"))

				// Validating pod status
				cmd = exec.Command("kubectl", "get",
					"pods", podNames[0], "-o", "jsonpath={.status.phase}",
					"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf))
				statusOutput, err := utils.Run(cmd)
				status := utils.GetNonEmptyLines(string(statusOutput))[0]
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			conf = conf1
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
			conf = conf2
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())

			// Getting RCLC names
			By("\033[32mGetting RemoteClusterConfig names")
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

			// Getting Secrets names to connect to the neighbor
			By("\033[32mGetting Secrets names")
			cmd = exec.Command("kubectl", "get",
				"secret", "-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf1))

			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			secret1 = utils.GetNonEmptyLines(string(output))[0]
			By(fmt.Sprintf("\033[33mSecret name: '%s'", secret1))

			cmd = exec.Command("kubectl", "get",
				"secret", "-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf2))

			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			secret2 = utils.GetNonEmptyLines(string(output))[0]
			By(fmt.Sprintf("\033[33mSecret name: '%s'", secret2))

			// Copy Secret from cluster #1 to cluster #2")
			By("\033[32mCopy Secret from cluster #1 to cluster #2")
			cmd = exec.Command("kubectl", "get", "secret", secret1, "-o", "yaml",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf1))
			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			secret := utils.GetNonEmptyLines(string(output))
			err = os.WriteFile("/tmp/secret.yaml", []byte(strings.Join(secret, "\n")), 0666)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			cmd = exec.Command("kubectl", "apply", "-f", "/tmp/secret.yaml",
				fmt.Sprintf("--kubeconfig=%s", conf2))
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			// Copy Secret from cluster #2 to cluster #1")
			By("\033[32mCopy Secret from cluster #2 to cluster #1")
			cmd = exec.Command("kubectl", "get", "secret", secret2, "-o", "yaml",
				"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			secret = utils.GetNonEmptyLines(string(output))
			err = os.WriteFile("/tmp/secret.yaml", []byte(strings.Join(secret, "\n")), 0666)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			cmd = exec.Command("kubectl", "apply", "-f", "/tmp/secret.yaml",
				fmt.Sprintf("--kubeconfig=%s", conf1))
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			rclcName = rclcName1
			conf = conf1
			verifyClusterCoupled := func() error {
				cmd := exec.Command("kubectl", "get",
					"rclc", rclcName, "-o", "jsonpath={.status.conditions[*].type}",
					"-n", namespace, fmt.Sprintf("--kubeconfig=%s", conf))
				statusOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				status := utils.GetNonEmptyLines(string(statusOutput))
				if len(status) == 0 {
					return fmt.Errorf("'%s' hasn't coupled yet; Output: %v", rclcName, status)
				}
				if !slices.Contains(strings.Split(status[0], " "), "Coupled") {
					return fmt.Errorf("'%s' hasn't coupled yet; Output: %v", rclcName, status)
				}
				return nil
			}
			By("\033[32mVerifing the RemoteClusterConfig status on cluster #1")
			EventuallyWithOffset(1, verifyClusterCoupled, time.Minute*10, time.Second).Should(Succeed())
			By("\033[32mThe cluster #1 is coupled")

			rclcName = rclcName2
			conf = conf2
			By("\033[32mVerifing the RemoteClusterConfig status on cluster #2")
			EventuallyWithOffset(1, verifyClusterCoupled, time.Minute*10, time.Second).Should(Succeed())
			By("\033[32mThe cluster #2 is coupled")

			By("\033[32mValidating that the echo pod is running as expected")
			testns = testns1
			num := 1
			verifyEchoUp := func() error {
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "run=echoserver",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", testns, fmt.Sprintf("--kubeconfig=%s", conf1))

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				echoPodName := podNames[0]
				ExpectWithOffset(2, echoPodName).Should(ContainSubstring("echoserver"))

				// Validating pod status
				cmd = exec.Command("kubectl", "get",
					"pods", echoPodName, "-o", "jsonpath={.status.phase}",
					"-n", testns, fmt.Sprintf("--kubeconfig=%s", conf1))
				statusOutput, err := utils.Run(cmd)
				status := utils.GetNonEmptyLines(string(statusOutput))[0]
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("echo pod in %s status", status)
				}
				if num == 1 {
					echoPodName1 = echoPodName
				}
				if num == 2 {
					echoPodName2 = echoPodName
				}
				return nil
			}
			EventuallyWithOffset(1, verifyEchoUp, time.Minute, time.Second).Should(Succeed())

			testns = testns2
			num = 2
			EventuallyWithOffset(1, verifyEchoUp, time.Minute, time.Second).Should(Succeed())

			By("\033[32mGetting echo pods IPs")
			cmd = exec.Command("kubectl", "get",
				"pods", echoPodName1, "-o", "jsonpath={.status.podIP}{\"\\n\"}",
				"-n", testns1, fmt.Sprintf("--kubeconfig=%s", conf1))
			output, err = utils.Run(cmd)
			echoPodIP1 = utils.GetNonEmptyLines(string(output))[0]
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			By(fmt.Sprintf("\033[33mEcho pod IP: `%s`", echoPodIP1))
			By("\033[32mGetting echo pods IPs")
			cmd = exec.Command("kubectl", "get",
				"pods", echoPodName2, "-o", "jsonpath={.status.podIP}{\"\\n\"}",
				"-n", testns2, fmt.Sprintf("--kubeconfig=%s", conf1))
			output, err = utils.Run(cmd)
			echoPodIP2 = utils.GetNonEmptyLines(string(output))[0]
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			By(fmt.Sprintf("\033[33mEcho pod IP: `%s`", echoPodIP2))

			echoSvcName = "echoserver"
			By("\033[32mGetting echo service IPs")
			cmd = exec.Command("kubectl", "get",
				"service", echoSvcName, "-o", "jsonpath={.spec.clusterIP}{\"\\n\"}",
				"-n", testns1, fmt.Sprintf("--kubeconfig=%s", conf1))
			output, err = utils.Run(cmd)
			echoSvcIP1 = utils.GetNonEmptyLines(string(output))[0]
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			By(fmt.Sprintf("\033[33mEcho service IP: `%s`", echoSvcIP1))
			By("\033[32mGetting echo service IPs")
			cmd = exec.Command("kubectl", "get",
				"service", echoSvcName, "-o", "jsonpath={.spec.clusterIP}{\"\\n\"}",
				"-n", testns2, fmt.Sprintf("--kubeconfig=%s", conf1))
			output, err = utils.Run(cmd)
			echoSvcIP2 = utils.GetNonEmptyLines(string(output))[0]
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			By(fmt.Sprintf("\033[33mEcho service IP: `%s`", echoSvcIP2))

			By(fmt.Sprintf("\033[32mChecking connectivity between `%s` namespace on cluster #1 and `%s` namespace on cluster #2", testns1, testns1))
			By(fmt.Sprintf("\033[32mNetworkPolicyConfig is disabled, so all of the connections between `%s` and `%s` must be \033[31mALLOWED", testns1, testns1))
			// Check pod-to-pod connectivity between namespaces (`test1`, `test1`)
			curlParam["ns1"] = testns1
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl1"
			curlParam["ip"] = echoPodIP1
			curlParam["conf"] = conf2
			curlParam["port"] = "5678"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			testns = testns1
			conf = conf2
			verifyCurlUp := func() error {
				cmd := exec.Command("kubectl", "get",
					"pods", curlParam["name"], "-o", "jsonpath={.status.phase}",
					"-n", testns, fmt.Sprintf("--kubeconfig=%s", conf))
				statusOutput, err := utils.Run(cmd)
				status := utils.GetNonEmptyLines(string(statusOutput))[0]
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("curl pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp := utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("\033[33mPod-to-Pod is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			By(fmt.Sprintf("\033[32mChecking connectivity between `%s` namespace on cluster #1 and `%s` namespace on cluster #2", testns2, testns1))
			By(fmt.Sprintf("\033[32mNetworkPolicyConfig is disabled, so all of the connections between `%s` and `%s` must be \033[31mALLOWED", testns2, testns1))
			// Check pod-to-pod connectivity between namespaces (`test1`, `test2`)
			curlParam["ns1"] = testns2
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl2"
			curlParam["ip"] = echoPodIP2
			curlParam["conf"] = conf2
			curlParam["port"] = "5678"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("\033[33mPod-to-Pod is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

			// Check pod-to-svc connectivity between namespaces (`test1`, `test1`)
			curlParam["ns1"] = testns1
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl3"
			curlParam["ip"] = echoSvcIP1
			curlParam["conf"] = conf2
			curlParam["port"] = "8080"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("\033[33mPod-to-Service is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

			// Check pod-to-svc connectivity between namespaces (`test1`, `test2`)
			curlParam["ns1"] = testns2
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl4"
			curlParam["ip"] = echoSvcIP2
			curlParam["conf"] = conf2
			curlParam["port"] = "8080"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("\033[33mPod-to-Service is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

			By("\033[32mEnabling NetworkPolicyConfig on cluster #1")
			cmd = exec.Command("kubectl", "patch", "npc", "default", "--type", "json", "-p",
				`[{"op":"replace","path":"/spec/enabled","value":true}]`,
				fmt.Sprintf("--kubeconfig=%s", conf1))
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			By("\033[32mEnabling NetworkPolicyConfig on cluster #2")
			cmd = exec.Command("kubectl", "patch", "npc", "default", "--type", "json", "-p",
				`[{"op":"replace","path":"/spec/enabled","value":true}]`,
				fmt.Sprintf("--kubeconfig=%s", conf2))
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			// Waiting for reconciliation
			By(fmt.Sprintf("\033[33mWaiting for reconciliation"))
			time.Sleep(40 * time.Second)

			By(fmt.Sprintf("\033[32mChecking connectivity between `%s` and `%s` namespaces", testns1, testns1))
			By(fmt.Sprintf("\033[32mEven though NetworkPolicyConfig is enabled all of the connections between `%s` and `%s` must be \033[31mALLOWED\033[32m, because they have the same name", testns1, testns1))
			// Check pod-to-pod connectivity between namespaces (`test1`, `test1`)
			curlParam["ns1"] = testns1
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl5"
			curlParam["ip"] = echoPodIP1
			curlParam["conf"] = conf2
			curlParam["port"] = "5678"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("\033[33mPod-to-Pod is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

			// Check pod-to-svc connectivity between namespaces (`test1`, `test1`)
			curlParam["ns1"] = testns1
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl6"
			curlParam["ip"] = echoSvcIP1
			curlParam["conf"] = conf2
			curlParam["port"] = "8080"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("\033[33mPod-to-Service is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

			By(fmt.Sprintf("\033[32mChecking connectivity between `%s` and `%s` namespaces", testns2, testns1))
			By(fmt.Sprintf("\033[32mNetworkPolicyConfig is enabled, so all of the connections between `%s` and `%s` must be \033[31m REJECTED", testns2, testns1))
			// Check pod-to-pod connectivity between namespaces (`test1`, `test2`)
			curlParam["ns1"] = testns2
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl7"
			curlParam["ip"] = echoPodIP2
			curlParam["conf"] = conf2
			curlParam["port"] = "5678"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) != 0 {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			By(fmt.Sprintf("\033[33mPod-to-Pod is rejected; %s response: `%v`", curlParam["name"], resp))

			// Check pod-to-svc connectivity between namespaces (`test1`, `test2`)
			curlParam["ns1"] = testns2
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl8"
			curlParam["ip"] = echoSvcIP2
			curlParam["conf"] = conf2
			curlParam["port"] = "8080"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) != 0 {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			By(fmt.Sprintf("\033[33mPod-to-Service is rejected; %s response: `%v`", curlParam["name"], resp))

			// Add testns1 to implicit namespaces
			By("\033[32mChanging implicitNamespaces in NetworkPolicyConfig on cluster #1")
			cmd = exec.Command("kubectl", "patch", "npc", "default", "--type", "json", "-p",
				fmt.Sprintf(`[{"op":"replace","path":"/spec/implicitNamespaces","value":"%s"}]`, testns1),
				fmt.Sprintf("--kubeconfig=%s", conf1))
			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			By("\033[32mChanging implicitNamespaces in NetworkPolicyConfig on cluster #2")
			cmd = exec.Command("kubectl", "patch", "npc", "default", "--type", "json", "-p",
				fmt.Sprintf(`[{"op":"replace","path":"/spec/implicitNamespaces","value":"%s"}]`, testns1),
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			// Waiting for reconciliation
			By(fmt.Sprintf("\033[33mWaiting for reconciliation"))
			time.Sleep(30 * time.Second)

			// Check pod-to-pod connectivity between namespaces (`test1`, `test2`)
			By(fmt.Sprintf("\033[32mChecking connectivity between `%s` and `%s` namespaces", testns2, testns1))
			By(fmt.Sprintf("\033[32mEven though NetworkPolicyConfig is enabled all of the connections between `%s` and `%s` must be \033[31mALLOWED\033[32m, because the `%s` is an implicit namespace", testns1, testns2, testns1))
			curlParam["ns1"] = testns2
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl9"
			curlParam["ip"] = echoPodIP2
			curlParam["conf"] = conf2
			curlParam["port"] = "5678"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			conf = conf2
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("\033[33mPod-to-Pod is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

			// Check pod-to-svc connectivity between namespaces (`test1`, `test2`)
			By(fmt.Sprintf("\033[32mChecking connectivity between `%s` and `%s` namespaces", testns2, testns1))
			By(fmt.Sprintf("\033[32mEven though NetworkPolicyConfig is enabled all of the connections between `%s` and `%s` must be \033[31mALLOWED\033[32m, because the `%s` is an implicit namespace", testns1, testns2, testns1))
			curlParam["ns1"] = testns2
			curlParam["ns2"] = testns1
			curlParam["img"] = curlImg
			curlParam["name"] = "curl10"
			curlParam["ip"] = echoSvcIP2
			curlParam["conf"] = conf2
			curlParam["port"] = "8080"
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns1, "logs", curlParam["name"],
				fmt.Sprintf("--kubeconfig=%s", conf2))
			output, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(output))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("\033[33mPod-to-Service is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

		})
	})
})
