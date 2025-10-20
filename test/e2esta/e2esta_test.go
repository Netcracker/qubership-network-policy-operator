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

var _ = Describe("Controller", Ordered, func() {
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

		By(fmt.Sprintf("Creating echo pod in `%s` namespace", testns1))
		cmd = exec.Command("kubectl", "apply", "-f", "./test/yamls/echo.yaml", "-n", testns1)
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
		// Uninstallation could fail
		//ExpectWithOffset(1, err).NotTo(HaveOccurred())

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
			var controllerPodName, echoPodName, echoPodIP string
			curlParam := make(map[string]string)

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

				// Validating pod status
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

			By("Validating that the echo pod is running as expected")
			verifyEchoUp := func() error {
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "run=echoserver",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", testns1,
				)

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				echoPodName = podNames[0]
				ExpectWithOffset(2, echoPodName).Should(ContainSubstring("echoserver"))

				// Validating pod status
				cmd = exec.Command("kubectl", "get",
					"pods", echoPodName, "-o", "jsonpath={.status.phase}",
					"-n", testns1,
				)
				statusOutput, err := utils.Run(cmd)
				status := utils.GetNonEmptyLines(string(statusOutput))[0]
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("echo pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyEchoUp, time.Minute, time.Second).Should(Succeed())

			By("Getting echo pod IP")
			cmd := exec.Command("kubectl", "get",
				"pods", echoPodName, "-o", "jsonpath={.status.podIP}{\"\\n\"}",
				"-n", testns1,
			)
			ipOutput, err := utils.Run(cmd)
			echoPodIP = utils.GetNonEmptyLines(string(ipOutput))[0]
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			By(fmt.Sprintf("Echo pod IP: `%s`", echoPodIP))

			By(fmt.Sprintf("Checking connectivity between `%s` and `%s` namespaces", testns1, testns2))
			By(fmt.Sprintf("NetworkPolicyConfig is disabled, so all of the connections between `%s` and `%s` must be ALLOWED", testns1, testns2))
			curlParam["ns1"] = testns1
			curlParam["ns2"] = testns2
			curlParam["img"] = curlImg
			curlParam["name"] = "curl1"
			curlParam["ip"] = ""
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			verifyCurlUp := func() error {
				cmd = exec.Command("kubectl", "get",
					"pods", curlParam["name"], "-o", "jsonpath={.status.phase}",
					"-n", testns2,
				)
				statusOutput, err := utils.Run(cmd)
				status := utils.GetNonEmptyLines(string(statusOutput))[0]
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("curl pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns2, "logs", curlParam["name"])
			curlOutput, err := utils.Run(cmd)
			resp := utils.GetNonEmptyLines(string(curlOutput))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("Pod-to-Service is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			curlParam["name"] = "curl2"
			curlParam["ip"] = echoPodIP
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns2, "logs", curlParam["name"])
			curlOutput, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(curlOutput))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("Pod-to-Pod is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

			By("Enabling NetworkPolicyConfig")
			cmd = exec.Command("kubectl", "patch", "npc", "default", "--type", "json",
				"--patch-file", "./test/yamls/patch.json")
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())

			// Checking NPO logs
			verifyNPOLog := func() error {
				cmd := exec.Command("kubectl", "-n", namespace, "logs", controllerPodName,
					"-c", "manager", "--tail=10")

				logOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				lastMessages := utils.GetNonEmptyLines(string(logOutput))
				for _, message := range lastMessages {
					//By(fmt.Sprintf("%s", message))
					if strings.Contains(message, "NetworkPolicyConfig Reconciler loop has been finished") {
						return nil
					}
				}
				return fmt.Errorf("Reconciliation hasn't been finished")
			}
			EventuallyWithOffset(1, verifyNPOLog, 2*time.Minute, time.Second).Should(Succeed())

			By(fmt.Sprintf("Checking connectivity between `%s` and `%s` namespaces", testns1, testns2))
			By(fmt.Sprintf("NetworkPolicyConfig is enabled, so all of the connections between `%s` and `%s` must be REJECTED", testns1, testns2))
			curlParam["name"] = "curl3"
			curlParam["ip"] = ""
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns2, "logs", curlParam["name"])
			curlOutput, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(curlOutput))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) != 0 {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			By(fmt.Sprintf("Pod-to-Service is rejected; %s response: `%v`", curlParam["name"], resp))
			curlParam["name"] = "curl4"
			curlParam["ip"] = echoPodIP
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns2, "logs", curlParam["name"])
			curlOutput, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(curlOutput))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) != 0 {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			By(fmt.Sprintf("Pod-to-Pod is rejected; %s response: `%v`", curlParam["name"], resp))

			By(fmt.Sprintf("Changing `%s` namespace annotation", testns1))
			cmd = exec.Command("kubectl", "annotate", "ns", testns1,
				fmt.Sprintf("network-policy.qubership.org/allowed-from=%s", testns2))
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			EventuallyWithOffset(1, verifyNPOLog, 2*time.Minute, time.Second).Should(Succeed())
			By(fmt.Sprintf("Checking connectivity between `%s` and `%s` namespaces", testns1, testns2))
			By(fmt.Sprintf("NetworkPolicyConfig is enabled and `%s` namespace has been annotated to allow the traffic from `%s`", testns1, testns2))
			curlParam["name"] = "curl5"
			curlParam["ip"] = ""
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns2, "logs", curlParam["name"])
			curlOutput, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(curlOutput))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("Pod-to-Service is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			curlParam["name"] = "curl6"
			curlParam["ip"] = echoPodIP
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns2, "logs", curlParam["name"])
			curlOutput, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(curlOutput))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) > 0 && resp[0] == "200" {
				By(fmt.Sprintf("Pod-to-Pod is allowed; %s response: `%s`", curlParam["name"], resp[0]))
			} else {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}

			By("Enabling logs for rejected connection")
			cmd = exec.Command("kubectl", "patch", "npc", "default", "--type", "json",
				"--patch-file", "./test/yamls/log_patch.json")
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			EventuallyWithOffset(1, verifyNPOLog, 2*time.Minute, time.Second).Should(Succeed())

			By(fmt.Sprintf("Changing `%s` namespace annotation", testns1))
			cmd = exec.Command("kubectl", "annotate", "ns", testns1,
				"network-policy.qubership.org/allowed-from-")
			_, err = utils.Run(cmd)
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			EventuallyWithOffset(1, verifyNPOLog, 2*time.Minute, time.Second).Should(Succeed())
			By(fmt.Sprintf("Checking connectivity between `%s` and `%s` namespaces", testns1, testns2))
			curlParam["name"] = "curl7"
			curlParam["ip"] = ""
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns2, "logs", curlParam["name"])
			curlOutput, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(curlOutput))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) != 0 {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			By(fmt.Sprintf("Pod-to-Service is rejected; %s response: `%v`", curlParam["name"], resp))
			curlParam["name"] = "curl8"
			curlParam["ip"] = echoPodIP
			Expect(utils.CreateCurlPod(curlParam)).To(Succeed())
			EventuallyWithOffset(1, verifyCurlUp, time.Minute, time.Second).Should(Succeed())
			cmd = exec.Command("kubectl", "-n", testns2, "logs", curlParam["name"])
			curlOutput, err = utils.Run(cmd)
			resp = utils.GetNonEmptyLines(string(curlOutput))
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			if len(resp) != 0 {
				Fail(fmt.Sprintf("%s has invalid response: %v", curlParam["name"], resp))
			}
			By(fmt.Sprintf("Pod-to-Pod is rejected; %s response: `%v`", curlParam["name"], resp))

			By("Getting node IP where the curl pod is running")
			cmd = exec.Command("kubectl", "-n", testns1, "get", "pod", echoPodName, "-o",
				"jsonpath={.status.hostIP}")
			nodeOutput, err := utils.Run(cmd)
			nodeIP := utils.GetNonEmptyLines(string(nodeOutput))[0]
			ExpectWithOffset(2, err).NotTo(HaveOccurred())
			By(fmt.Sprintf("!!!!!!!! It's possible to check the logs by the following command:\n ssh -i %s %s@%s \"journalctl --all --grep 'calico-packet'\"", key, user, nodeIP))
		})
	})
})
