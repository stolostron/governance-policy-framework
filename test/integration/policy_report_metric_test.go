// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test policyreport_info metric", Ordered, Label("BVT"), func() {
	const (
		ocmNS                        = "open-cluster-management"
		saName                       = "grc-framework-sa"
		roleBindingName              = "grc-framework-role-binding"
		saTokenName                  = "grc-framework-sa-token-manual"
		saTokenYaml                  = "../resources/policy_report_metric/metrics_token.yaml"
		insightsClientPodSelector    = "component=insights-client"
		insightsMetricsSelector      = "component=insights-metrics"
		insightsMetricName           = "policyreport_info"
		noncompliantPolicyYamlReport = "../resources/policy_report_metric/noncompliant.yaml"
		noncompliantPolicyNameReport = "policyreport-metric-noncompliant"
		compliantPolicyYamlReport    = "../resources/policy_report_metric/compliant.yaml"
		compliantPolicyNameReport    = "policyreport-metric-noncompliant"
	)

	var (
		insightsMetricsURL       string
		insightsToken            string
		insightsClientDeployment string
	)

	JustAfterEach(func() {
		if CurrentSpecReport().Failed() {
			By("*** Debugging policyreport_info metric failure ***")

			By("Getting current policies")
			_, err := common.OcHub("get", "policies.policy.open-cluster-management.io", "-A", "-o", "yaml")
			Expect(err).To(BeNil())

			By("Getting current configurationpolicies")
			_, err = common.OcHub("get", "configurationpolicies.policy.open-cluster-management.io", "-A", "-o", "yaml")
			Expect(err).To(BeNil())

			By("Getting current policyreports")
			_, err = common.OcHub("get", "policyreports.wgpolicyk8s.io", "-A", "-o", "yaml")
			Expect(err).To(BeNil())

			By("Getting yaml and logs for insights client pod(s)")
			_, err = common.OcHub("get", "pods", "-n", ocmNS, "-l", insightsClientPodSelector, "-o", "yaml")
			Expect(err).To(BeNil())

			clientPodList, err := clientHub.CoreV1().Pods(ocmNS).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: insightsClientPodSelector,
				},
			)
			Expect(err).To(BeNil())
			for _, pod := range clientPodList.Items {
				By("Logs for " + pod.GetName())
				_, err := common.OcHub("logs", "-n", ocmNS, pod.GetName())
				Expect(err).To(BeNil())
			}

			By("Getting yaml and logs for insights metric pod(s)")
			_, err = common.OcHub(
				"get",
				"pods",
				"-n",
				ocmNS,
				"-l",
				insightsMetricsSelector,
				"-o",
				"yaml",
			)
			Expect(err).To(BeNil())

			metricsPodList, err := clientHub.CoreV1().Pods(ocmNS).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: insightsMetricsSelector,
				},
			)
			Expect(err).To(BeNil())
			for _, pod := range metricsPodList.Items {
				By("Logs for " + pod.GetName())
				_, err := common.OcHub("logs", "-n", ocmNS, pod.GetName(), "-c", "metrics")
				Expect(err).To(BeNil())
			}
		}
	})

	It("Sets up the metrics service endpoint for tests", func() {
		By("Setting the insights client to poll every minute")
		var insightsClientPod string
		Eventually(func() interface{} {
			var err error

			insightsClientPod, err = common.OcHub(
				"get",
				"pods",
				"-n",
				ocmNS,
				"-l",
				insightsClientPodSelector,
				"-o",
				"name",
			)
			insightsClientPods := strings.Split(insightsClientPod, "pod/")
			if err != nil || len(insightsClientPods) < 2 {
				return errors.New("could not find insights client pod")
			}

			insightsClientPod = insightsClientPods[1]

			// The pod is formatted as "policyreport-0f89c-insights-client-5c7d5fd5b4-5fdbz". The deployment name
			// is the pod name without the "-5c7d5fd5b4-5fdbz" suffix.
			splitPodName := strings.SplitN(insightsClientPod, "-", 5)
			insightsClientDeployment = "deployment.apps/" + strings.Join(splitPodName[:len(splitPodName)-1], "-")

			return nil
		}, defaultTimeoutSeconds*10, 1).Should(BeNil())

		_, err := common.OcHub(
			"set",
			"env",
			"-n",
			ocmNS,
			insightsClientDeployment,
			"POLL_INTERVAL=1",
		)
		Expect(err).To(BeNil())
		// checking if new pod has spun up
		Eventually(func() interface{} {
			var err error
			pod, err := common.OcHub(
				"get",
				"pods",
				"-n",
				ocmNS,
				"-l",
				insightsClientPodSelector,
				"--field-selector=status.phase=Running,metadata.name!="+insightsClientPod,
			)
			if err != nil {
				return err
			}

			return pod
		}, defaultTimeoutSeconds*10, 1).ShouldNot(Equal(""))
		// checking if old pod with slow refresh has been taken down
		Eventually(func() interface{} {
			var err error
			pod, err := common.OcHub(
				"get",
				"pods",
				"-n",
				ocmNS,
				"-l",
				insightsClientPodSelector,
				"--field-selector=status.phase=Running,metadata.name="+insightsClientPod,
			)
			if err != nil {
				return err
			}

			return pod
		}, defaultTimeoutSeconds*10, 1).Should(Equal(""))

		By("Ensuring the metrics service exists")
		svcList, err := clientHub.CoreV1().Services(ocmNS).List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: insightsMetricsSelector,
			},
		)
		Expect(err).To(BeNil())
		Expect(len(svcList.Items)).To(Equal(1))
		metricsSvc := svcList.Items[0]

		By("Checking for an existing metrics route")
		var routeList *unstructured.UnstructuredList
		Eventually(func() interface{} {
			var err error
			routeList, err = clientHubDynamic.Resource(common.GvrRoute).Namespace(ocmNS).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: insightsMetricsSelector,
				},
			)
			if err != nil {
				return err
			}

			return len(routeList.Items)
		}, defaultTimeoutSeconds, 1).Should(Or(Equal(0), Equal(1)))

		if len(routeList.Items) == 0 {
			By("Exposing the insights metrics service as a route")
			_, err = common.OcHub(
				"expose",
				"service",
				metricsSvc.Name,
				"-n",
				ocmNS,
				`--overrides={"spec":{"tls":{"termination":"reencrypt"}}}`,
			)
			Expect(err).To(BeNil())

			Eventually(func() interface{} {
				var err error
				routeList, err = clientHubDynamic.Resource(common.GvrRoute).Namespace(ocmNS).List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: insightsMetricsSelector,
					},
				)
				if err != nil {
					return err
				}

				return len(routeList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(1))
		}

		routeHost := routeList.Items[0].Object["spec"].(map[string]interface{})["host"].(string)
		By("Got the metrics route url: " + routeHost)
		insightsMetricsURL = "https://" + routeHost + "/metrics"
	})
	It("Sets up a ServiceAccount with permissions for metrics", func() {
		_, err := common.OcHub("create", "serviceaccount", saName, "-n", userNamespace)
		Expect(err).To(BeNil())

		_, err = common.OcHub("create", "clusterrolebinding", roleBindingName, "--clusterrole=cluster-admin",
			fmt.Sprintf("--serviceaccount=%s:%s", userNamespace, saName))
		Expect(err).To(BeNil())

		_, err = common.OcHub("apply", "-f", saTokenYaml, "-n", userNamespace)
		Expect(err).To(BeNil())

		var encodedtoken string

		// The secret could take a moment to be populated with the token
		Eventually(func(g Gomega) {
			var err error
			encodedtoken, err = common.OcHub("get", "secret", saTokenName,
				"-n", userNamespace, "-o", "jsonpath={.data.token}")

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(len(encodedtoken)).To(BeNumerically(">", 0))
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		decodedToken, err := base64.StdEncoding.DecodeString(encodedtoken)
		Expect(err).To(BeNil())

		insightsToken = string(decodedToken)
	})
	It("Checks that the endpoint does not expose metrics without auth", func() {
		Eventually(func() interface{} {
			_, status, err := common.GetWithToken(insightsMetricsURL, "")
			if err != nil {
				return err
			}

			return status
		}, defaultTimeoutSeconds, 1).Should(ContainSubstring("Unauthorized"))
	})
	It("Checks that a noncompliant policy reports a metric", func() {
		By("Creating a noncompliant policy")
		_, err := common.OcHub("apply", "-f", noncompliantPolicyYamlReport, "-n", userNamespace)
		Expect(err).To(BeNil())
		Eventually(
			getComplianceState(noncompliantPolicyNameReport),
			defaultTimeoutSeconds*8,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Checking the policy metric")
		output, err := common.OcHub("set", "env", "-n", ocmNS, insightsClientDeployment, "--list")
		Expect(err).To(BeNil())
		klog.V(5).Infof("INSIGHTS CLIENT ENV VARIABLES:%s\n", output)

		policyLabel := `policy="` + userNamespace + "." + noncompliantPolicyNameReport + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(
				insightsMetricsURL,
				strings.TrimSpace(insightsToken),
			)
			if err != nil {
				fmt.Println("ERROR GETTING METRIC:")
				fmt.Println(err)

				return err
			}
			fmt.Println("metric response received:")
			fmt.Println(resp)

			return resp
		}, 10*time.Minute, 1).Should(common.MatchMetricValue(insightsMetricName, policyLabel, "1"))
	})
	It("Checks that changing the policy to compliant removes the metric", func() {
		By("Creating a compliant policy")
		_, err := common.OcHub("apply", "-f", compliantPolicyYamlReport, "-n", userNamespace)
		Expect(err).To(BeNil())
		Eventually(
			getComplianceState(compliantPolicyNameReport),
			defaultTimeoutSeconds*8,
			1,
		).Should(Equal(policiesv1.Compliant))

		By("Checking the policy metric displays nothing")
		output, err := common.OcHub("set", "env", "-n", ocmNS, insightsClientDeployment, "--list")
		Expect(err).To(BeNil())
		klog.V(5).Infof("INSIGHTS CLIENT ENV VARIABLES:%s\n", output)

		policyLabel := `policy="` + userNamespace + "." + noncompliantPolicyNameReport + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(
				insightsMetricsURL,
				strings.TrimSpace(insightsToken),
			)
			if err != nil {
				fmt.Println("ERROR GETTING METRIC:")
				fmt.Println(err)

				return err
			}
			fmt.Println("metric response received:")
			fmt.Println(resp)

			return resp
		}, 10*time.Minute, 1).ShouldNot(common.MatchMetricValue(insightsMetricName, policyLabel, "1"))
	})
	AfterAll(func() {
		// unset poll interval
		_, err := common.OcHub("set", "env", "-n", ocmNS, insightsClientDeployment, "POLL_INTERVAL-")
		Expect(err).To(BeNil())
		_, err = common.OcHub(
			"delete", "-f", compliantPolicyYamlReport,
			"-n", userNamespace, "--ignore-not-found",
		)
		Expect(err).To(BeNil())
		_, err = common.OcHub(
			"delete", "route", "-n", ocmNS, "-l",
			insightsMetricsSelector, "--ignore-not-found",
		)
		Expect(err).To(BeNil())
		_, err = common.OcHub(
			"delete", "clusterrolebinding",
			roleBindingName, "--ignore-not-found",
		)
		Expect(err).To(BeNil())
		_, err = common.OcHub(
			"delete", "serviceaccount", saName, "-n",
			userNamespace, "--ignore-not-found",
		)
		Expect(err).To(BeNil())
		_, err = common.OcHub(
			"delete", "namespace",
			"policy-metric-test-compliant", "--ignore-not-found",
		)
		Expect(err).To(BeNil())
	})
})
