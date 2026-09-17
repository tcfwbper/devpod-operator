//go:build e2e
// +build e2e

/*
Copyright 2026.

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

package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tcfwbper/devpod-operator/test/utils"
)

// namespace where the project is deployed in
const namespace = "devpod-operator-system"

// serviceAccountName created for the project
const serviceAccountName = "devpod-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "devpod-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "devpod-operator-metrics-binding"

// secretKeyPassword is the key in the DevPod-owned Secret that the controller
// reads the ssh password from (see internal/controller/helpers.go).
const secretKeyPassword = "ubuntu-password"

// testPassword is a password that satisfies validatePassword (>= 8 chars, no ":" or line breaks).
const testPassword = "testpassword123"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", managerImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				By("getting the name of the controller-manager pod")
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				By("validating the pod's status")
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=devpod-operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("ensuring the controller pod is ready")
			verifyControllerPodReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", controllerPodName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Controller pod not ready")
			}
			Eventually(verifyControllerPodReady, 3*time.Minute, time.Second).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted, 3*time.Minute, time.Second).Should(Succeed())

			// +kubebuilder:scaffold:e2e-metrics-webhooks-readiness

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": [
								"for i in $(seq 1 30); do curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics && exit 0 || sleep 2; done; exit 1"
							],
							"securityContext": {
								"readOnlyRootFilesystem": true,
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccountName": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			verifyMetricsAvailable := func(g Gomega) {
				metricsOutput, err := getMetricsOutput()
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
				g.Expect(metricsOutput).NotTo(BeEmpty())
				g.Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
			}
			Eventually(verifyMetricsAvailable, 2*time.Minute).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})

	Context("DevPod sub-resource creation", Ordered, func() {
		const (
			devpodName = "e2e-test-devpod"
			testNS     = "devpod-e2e-test"
		)

		BeforeAll(func() {
			By("creating a test namespace")
			cmd := exec.Command("kubectl", "create", "ns", testNS)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

			By("labeling namespace to allow privileged pods")
			cmd = exec.Command("kubectl", "label", "--overwrite", "ns", testNS,
				"pod-security.kubernetes.io/enforce=privileged")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("applying the DevPod CR")
			cr := fmt.Sprintf(`apiVersion: apps.devpod.com/v1
kind: DevPod
metadata:
  name: %s
  namespace: %s
spec:
  image: docker.io/tcfwbper/dev-env:1.0.0
  initWorkspaceImage: docker.io/tcfwbper/dev-env:1.0.0-init-workspace
  auth:
    username: testuser
  persistence:
    storageClass: standard
    size: 1Gi
    reclaimPolicy: Retain
  nodePorts:
    - src: 22
      dest: 30122
  docker:
    enabled: false`, devpodName, testNS)

			tmpFile, err := os.CreateTemp("", "devpod-e2e-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpFile.Name())
			_, err = tmpFile.WriteString(cr)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpFile.Close()).To(Succeed())

			cmd = exec.Command("kubectl", "apply", "-f", tmpFile.Name())
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create DevPod CR")
		})

		AfterAll(func() {
			By("deleting the DevPod CR")
			cmd := exec.Command("kubectl", "delete", "devpod", devpodName, "-n", testNS,
				"--ignore-not-found=true", "--timeout=60s")
			_, _ = utils.Run(cmd)

			By("deleting test namespace")
			cmd = exec.Command("kubectl", "delete", "ns", testNS, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		})

		It("should create a Secret with owner reference", func() {
			verifySecret := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "secret", devpodName,
					"-n", testNS, "-o", "json")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Secret not found")

				var secret map[string]interface{}
				g.Expect(json.Unmarshal([]byte(output), &secret)).To(Succeed())

				metadata := secret["metadata"].(map[string]interface{})
				ownerRefs, ok := metadata["ownerReferences"].([]interface{})
				g.Expect(ok).To(BeTrue(), "ownerReferences not found")
				g.Expect(ownerRefs).To(HaveLen(1))

				ref := ownerRefs[0].(map[string]interface{})
				g.Expect(ref["kind"]).To(Equal("DevPod"))
				g.Expect(ref["name"]).To(Equal(devpodName))
				g.Expect(ref["controller"]).To(BeTrue())
			}
			Eventually(verifySecret, 3*time.Minute, 2*time.Second).Should(Succeed())

			By("patching the Secret with a valid password to unblock reconciliation")
			patchSecretPassword(devpodName, testNS)
		})

		It("should create a ServiceAccount", func() {
			verifySA := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "serviceaccount", devpodName,
					"-n", testNS, "-o", "jsonpath={.metadata.labels.app\\.kubernetes\\.io/instance}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "ServiceAccount not found")
				g.Expect(output).To(Equal(devpodName))
			}
			Eventually(verifySA, 3*time.Minute, 2*time.Second).Should(Succeed())
		})

		It("should create a NodePort Service with correct ports", func() {
			verifyService := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", devpodName,
					"-n", testNS, "-o", "json")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Service not found")

				var svc map[string]interface{}
				g.Expect(json.Unmarshal([]byte(output), &svc)).To(Succeed())

				spec := svc["spec"].(map[string]interface{})
				g.Expect(spec["type"]).To(Equal("NodePort"))

				ports := spec["ports"].([]interface{})
				found := false
				for _, p := range ports {
					port := p.(map[string]interface{})
					if int(port["port"].(float64)) == 22 {
						g.Expect(int(port["nodePort"].(float64))).To(Equal(30122))
						found = true
					}
				}
				g.Expect(found).To(BeTrue(), "SSH port mapping not found")
			}
			Eventually(verifyService, 3*time.Minute, 2*time.Second).Should(Succeed())
		})

		It("should create a StatefulSet with correct spec", func() {
			verifySTS := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "statefulset", devpodName,
					"-n", testNS, "-o", "json")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "StatefulSet not found")

				var sts map[string]interface{}
				g.Expect(json.Unmarshal([]byte(output), &sts)).To(Succeed())

				spec := sts["spec"].(map[string]interface{})
				g.Expect(int(spec["replicas"].(float64))).To(Equal(1))

				template := spec["template"].(map[string]interface{})
				podSpec := template["spec"].(map[string]interface{})
				g.Expect(podSpec["serviceAccountName"]).To(Equal(devpodName))

				containers := podSpec["containers"].([]interface{})
				g.Expect(containers).NotTo(BeEmpty())
				mainContainer := containers[0].(map[string]interface{})
				g.Expect(mainContainer["image"]).To(Equal("docker.io/tcfwbper/dev-env:1.0.0"))

				vcts := spec["volumeClaimTemplates"].([]interface{})
				g.Expect(vcts).NotTo(BeEmpty())
			}
			Eventually(verifySTS, 3*time.Minute, 2*time.Second).Should(Succeed())
		})
	})

	Context("DevPod CRUD lifecycle", Ordered, func() {
		const (
			lifecycleNS = "devpod-e2e-lifecycle"
		)

		BeforeAll(func() {
			By("creating lifecycle test namespace")
			cmd := exec.Command("kubectl", "create", "ns", lifecycleNS)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("labeling namespace to allow privileged pods")
			cmd = exec.Command("kubectl", "label", "--overwrite", "ns", lifecycleNS,
				"pod-security.kubernetes.io/enforce=privileged")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			By("deleting lifecycle test namespace")
			cmd := exec.Command("kubectl", "delete", "ns", lifecycleNS, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
		})

		It("should update Service when DevPod spec changes", func() {
			const devpodName = "e2e-update-test"

			By("creating a DevPod CR")
			cr := fmt.Sprintf(`apiVersion: apps.devpod.com/v1
kind: DevPod
metadata:
  name: %s
  namespace: %s
spec:
  auth:
    username: updateuser
  persistence:
    storageClass: standard
    size: 1Gi
    reclaimPolicy: Delete
  nodePorts:
    - src: 22
      dest: 30222
  docker:
    enabled: false`, devpodName, lifecycleNS)

			tmpFile, err := os.CreateTemp("", "devpod-update-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpFile.Name())
			_, err = tmpFile.WriteString(cr)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpFile.Close()).To(Succeed())

			cmd := exec.Command("kubectl", "apply", "-f", tmpFile.Name())
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("patching the Secret with a valid password")
			waitForSecretAndPatch(devpodName, lifecycleNS)

			By("waiting for the Service to exist")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", devpodName, "-n", lifecycleNS)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}, 3*time.Minute, 2*time.Second).Should(Succeed())

			By("adding a new nodePort to the DevPod")
			patch := `{"spec":{"nodePorts":[{"src":22,"dest":30222},{"src":8080,"dest":30280}]}}`
			cmd = exec.Command("kubectl", "patch", "devpod", devpodName,
				"-n", lifecycleNS, "-p", patch, "--type=merge")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch DevPod with new nodePort")

			By("verifying the Service gains the new port")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "service", devpodName,
					"-n", lifecycleNS, "-o", "json")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				var svc map[string]interface{}
				g.Expect(json.Unmarshal([]byte(output), &svc)).To(Succeed())

				spec := svc["spec"].(map[string]interface{})
				ports := spec["ports"].([]interface{})
				var foundPorts []int
				for _, p := range ports {
					port := p.(map[string]interface{})
					foundPorts = append(foundPorts, int(port["nodePort"].(float64)))
				}
				g.Expect(foundPorts).To(ContainElement(30280), "New nodePort 30280 not found")
			}, 3*time.Minute, 2*time.Second).Should(Succeed())

			By("cleaning up the update-test DevPod")
			cmd = exec.Command("kubectl", "delete", "devpod", devpodName,
				"-n", lifecycleNS, "--timeout=60s")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete PVCs when reclaimPolicy is Delete", func() {
			const devpodName = "e2e-delete-pvc"

			By("creating a DevPod CR with reclaimPolicy: Delete")
			cr := fmt.Sprintf(`apiVersion: apps.devpod.com/v1
kind: DevPod
metadata:
  name: %s
  namespace: %s
spec:
  auth:
    username: deluser
  persistence:
    storageClass: standard
    size: 1Gi
    reclaimPolicy: Delete
  nodePorts:
    - src: 22
      dest: 30322
  docker:
    enabled: false`, devpodName, lifecycleNS)

			tmpFile, err := os.CreateTemp("", "devpod-delete-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpFile.Name())
			_, err = tmpFile.WriteString(cr)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpFile.Close()).To(Succeed())

			cmd := exec.Command("kubectl", "apply", "-f", tmpFile.Name())
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("patching the Secret with a valid password")
			waitForSecretAndPatch(devpodName, lifecycleNS)

			By("waiting for PVCs to be created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pvc",
					"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", devpodName),
					"-n", lifecycleNS, "-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).NotTo(BeEmpty(), "PVCs not yet created")
			}, 3*time.Minute, 2*time.Second).Should(Succeed())

			By("deleting the DevPod CR")
			cmd = exec.Command("kubectl", "delete", "devpod", devpodName,
				"-n", lifecycleNS, "--timeout=60s")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the DevPod is gone")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "devpod", devpodName,
					"-n", lifecycleNS, "--ignore-not-found=true")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).To(BeEmpty())
			}, 3*time.Minute, 2*time.Second).Should(Succeed())

			By("verifying PVCs are deleted")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pvc",
					"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", devpodName),
					"-n", lifecycleNS, "-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).To(BeEmpty(), "PVCs should be deleted")
			}, 3*time.Minute, 2*time.Second).Should(Succeed())
		})

		It("should retain PVCs when reclaimPolicy is Retain", func() {
			const devpodName = "e2e-retain-pvc"

			By("creating a DevPod CR with reclaimPolicy: Retain")
			cr := fmt.Sprintf(`apiVersion: apps.devpod.com/v1
kind: DevPod
metadata:
  name: %s
  namespace: %s
spec:
  auth:
    username: retainuser
  persistence:
    storageClass: standard
    size: 1Gi
    reclaimPolicy: Retain
  nodePorts:
    - src: 22
      dest: 30422
  docker:
    enabled: false`, devpodName, lifecycleNS)

			tmpFile, err := os.CreateTemp("", "devpod-retain-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpFile.Name())
			_, err = tmpFile.WriteString(cr)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpFile.Close()).To(Succeed())

			cmd := exec.Command("kubectl", "apply", "-f", tmpFile.Name())
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("patching the Secret with a valid password")
			waitForSecretAndPatch(devpodName, lifecycleNS)

			By("waiting for PVCs to be created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pvc",
					"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", devpodName),
					"-n", lifecycleNS, "-o", "jsonpath={.items[*].metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).NotTo(BeEmpty(), "PVCs not yet created")
			}, 3*time.Minute, 2*time.Second).Should(Succeed())

			By("deleting the DevPod CR")
			cmd = exec.Command("kubectl", "delete", "devpod", devpodName,
				"-n", lifecycleNS, "--timeout=60s")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying the DevPod is gone")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "devpod", devpodName,
					"-n", lifecycleNS, "--ignore-not-found=true")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.TrimSpace(output)).To(BeEmpty())
			}, 3*time.Minute, 2*time.Second).Should(Succeed())

			By("verifying PVCs are retained")
			cmd = exec.Command("kubectl", "get", "pvc",
				"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", devpodName),
				"-n", lifecycleNS, "-o", "jsonpath={.items[*].metadata.name}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(output)).NotTo(BeEmpty(), "PVCs should be retained")

			By("manually cleaning up retained PVCs")
			cmd = exec.Command("kubectl", "delete", "pvc",
				"-l", fmt.Sprintf("app.kubernetes.io/instance=%s", devpodName),
				"-n", lifecycleNS)
			_, _ = utils.Run(cmd)
		})
	})
})

// waitForSecretAndPatch waits for the controller to create the DevPod-owned
// Secret and then fills in a valid password so reconciliation can proceed.
func waitForSecretAndPatch(name, ns string) {
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "secret", name, "-n", ns)
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
	}, 3*time.Minute, 2*time.Second).Should(Succeed())

	patchSecretPassword(name, ns)
}

// patchSecretPassword writes testPassword into the secretKeyPassword key of the
// named Secret. The key name is fixed by the controller and does not depend on
// spec.auth.username.
func patchSecretPassword(name, ns string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(testPassword))
	patch := fmt.Sprintf(`{"data":{"%s":"%s"}}`, secretKeyPassword, encoded)
	cmd := exec.Command("kubectl", "patch", "secret", name,
		"-n", ns, "-p", patch, "--type=merge")
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to patch Secret with password")
}

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	By("creating temporary file to store the token request")
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		By("executing kubectl command to create the token")
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		By("parsing the JSON output to extract the token")
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	return utils.Run(cmd)
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
