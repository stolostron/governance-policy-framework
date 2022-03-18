// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "github.com/stolostron/governance-policy-propagator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stolostron/governance-policy-framework/test/common"
)

const (
	insightsClientSelector       = "component=insights-client"
	insightsMetricsSelector      = "component=insights-metrics"
	insightsMetricName           = "policyreport_info"
	noncompliantPolicyYamlReport = "../resources/policy_report_metric/noncompliant.yaml"
	noncompliantPolicyNameReport = "policyreport-metric-noncompliant"
	compliantPolicyYamlReport    = "../resources/policy_report_metric/compliant.yaml"
	compliantPolicyNameReport    = "policyreport-metric-noncompliant"
)

var insightsMetricsURL string

var insightsToken string

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test policyreport_info metric", Label("BVT"), func() {
	It("Sets up the metrics service endpoint for tests", func() {
		By("Setting the insights client to poll every minute")
		insightsClient, err := common.OcHub("get", "deployments", "-n", ocmNS, "-l", insightsClientSelector, "-o", "name")
		Expect(err).To(BeNil())
		insightsClientPod, err := common.OcHub("get", "pods", "-n", ocmNS, "-l", insightsClientSelector, "-o", "name")
		Expect(err).To(BeNil())
		insightsClientPod = strings.Split(insightsClientPod, "pod/")[1]
		insightsClient = strings.TrimSpace(insightsClient)
		_, err = common.OcHub("set", "env", "-n", ocmNS, insightsClient, "POLL_INTERVAL=1")
		Expect(err).To(BeNil())
		// checking if new pod has spun up
		Eventually(func() interface{} {
			var err error
			pod, err := common.OcHub("get", "pods", "-n", ocmNS, "-l", insightsClientSelector, "--field-selector=status.phase=Running,metadata.name!="+insightsClientPod)
			if err != nil {
				return err
			}
			return pod
		}, defaultTimeoutSeconds*2, 1).ShouldNot(Equal(""))
		// checking if old pod with slow refresh has been taken down
		Eventually(func() interface{} {
			var err error
			pod, err := common.OcHub("get", "pods", "-n", ocmNS, "-l", insightsClientSelector, "--field-selector=status.phase=Running,metadata.name="+insightsClientPod)
			if err != nil {
				return err
			}
			return pod
		}, defaultTimeoutSeconds*10, 1).Should(Equal(""))

		By("Ensuring the metrics service exists")
		svcList, err := clientHub.CoreV1().Services(ocmNS).List(context.TODO(), metav1.ListOptions{LabelSelector: insightsMetricsSelector})
		Expect(err).To(BeNil())
		Expect(len(svcList.Items)).To(Equal(1))
		metricsSvc := svcList.Items[0]

		By("Checking for an existing metrics route")
		var routeList *unstructured.UnstructuredList
		Eventually(func() interface{} {
			var err error
			routeList, err = clientHubDynamic.Resource(common.GvrRoute).Namespace(ocmNS).List(context.TODO(), metav1.ListOptions{LabelSelector: insightsMetricsSelector})
			if err != nil {
				return err
			}
			return len(routeList.Items)
		}, defaultTimeoutSeconds, 1).Should(Or(Equal(0), Equal(1)))

		if len(routeList.Items) == 0 {
			By("Exposing the insights metrics service as a route")
			_, err = common.OcHub("expose", "service", metricsSvc.Name, "-n", ocmNS, `--overrides={"spec":{"tls":{"termination":"reencrypt"}}}`)
			Expect(err).To(BeNil())

			Eventually(func() interface{} {
				var err error
				routeList, err = clientHubDynamic.Resource(common.GvrRoute).Namespace(ocmNS).List(context.TODO(), metav1.ListOptions{LabelSelector: insightsMetricsSelector})
				if err != nil {
					return err
				}
				return len(routeList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(1))
		}

		routeHost := routeList.Items[0].Object["spec"].(map[string]interface{})["host"].(string)
		By("Got the metrics route url: " + routeHost)
		insightsMetricsURL = "https://" + routeHost + "/metrics"

		// get auth token from service account
		By("Setting up ServiceAccount for authentication")
		_, err = common.OcHub("create", "serviceaccount", saName, "-n", userNamespace)
		Expect(err).To(BeNil())
		_, err = common.OcHub("create", "clusterrolebinding", roleBindingName, "--clusterrole=cluster-admin", fmt.Sprintf("--serviceaccount=%s:%s", userNamespace, saName))
		Expect(err).To(BeNil())

		// The secret can take a moment to be created, retry until it is in the cluster.
		var tokenName string
		Eventually(func() interface{} {
			tokenNames, err := common.OcHub("get", fmt.Sprintf("serviceaccount/%s", saName), "-n", userNamespace, "-o", "jsonpath={.secrets[*].name}")
			if err != nil {
				return err
			}
			tokenNameArr := strings.Split(tokenNames, " ")
			for _, name := range tokenNameArr {
				if strings.HasPrefix(name, saName+"-token-") {
					tokenName = name
				}
			}
			return tokenName
		}, defaultTimeoutSeconds, 1).Should(ContainSubstring(saName + "-token-"))

		encodedtoken, err := common.OcHub("get", "secret", tokenName, "-n", userNamespace, "-o", "jsonpath={.data.token}")
		Expect(err).To(BeNil())
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
		common.OcHub("apply", "-f", noncompliantPolicyYamlReport, "-n", userNamespace)
		Eventually(
			getComplianceState(noncompliantPolicyNameReport),
			defaultTimeoutSeconds*8,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Checking the policy metric")
		insightsClient, err := common.OcHub("get", "deployments", "-n", ocmNS, "-l", insightsClientSelector, "-o", "name")
		Expect(err).To(BeNil())
		insightsClient = strings.TrimSpace(insightsClient)
		output, err := common.OcHub("set", "env", "-n", ocmNS, insightsClient, "--list")
		Expect(err).To(BeNil())
		fmt.Println("INSIGHTS CLIENT ENV VARIABLES:")
		fmt.Println(output)

		policyLabel := `policy="` + userNamespace + "." + noncompliantPolicyNameReport + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(insightsMetricsURL, strings.TrimSpace(insightsToken))
			if err != nil {
				fmt.Println("ERROR GETTING METRIC:")
				fmt.Println(err)
				return err
			}
			fmt.Println("metric response received:")
			fmt.Println(resp)
			return resp
		}, defaultTimeoutSeconds*8, 1).Should(common.MatchMetricValue(insightsMetricName, policyLabel, "1"))
	})
	It("Checks that changing the policy to compliant removes the metric", func() {
		By("Creating a compliant policy")
		common.OcHub("apply", "-f", compliantPolicyYamlReport, "-n", userNamespace)
		Eventually(
			getComplianceState(compliantPolicyNameReport),
			defaultTimeoutSeconds*8,
			1,
		).Should(Equal(policiesv1.Compliant))

		By("Checking the policy metric displays nothing")
		insightsClient, err := common.OcHub("get", "deployments", "-n", ocmNS, "-l", insightsClientSelector, "-o", "name")
		Expect(err).To(BeNil())
		insightsClient = strings.TrimSpace(insightsClient)
		output, err := common.OcHub("set", "env", "-n", ocmNS, insightsClient, "--list")
		Expect(err).To(BeNil())
		fmt.Println("INSIGHTS CLIENT ENV VARIABLES:")
		fmt.Println(output)

		policyLabel := `policy="` + userNamespace + "." + noncompliantPolicyNameReport + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(insightsMetricsURL, strings.TrimSpace(insightsToken))
			if err != nil {
				fmt.Println("ERROR GETTING METRIC:")
				fmt.Println(err)
				return err
			}
			fmt.Println("metric response received:")
			fmt.Println(resp)
			return resp
		}, defaultTimeoutSeconds*8, 1).ShouldNot(common.MatchMetricValue(insightsMetricName, policyLabel, "1"))
	})
	It("Cleans up", func() {
		// unset poll interval
		insightsClient, err := common.OcHub("get", "deployments", "-n", ocmNS, "-l", insightsClientSelector, "-o", "name")
		Expect(err).To(BeNil())
		insightsClient = strings.TrimSpace(insightsClient)
		_, err = common.OcHub("set", "env", "-n", ocmNS, insightsClient, "POLL_INTERVAL-")
		Expect(err).To(BeNil())
		common.OcHub("delete", "-f", compliantPolicyYamlReport, "-n", userNamespace)
		common.OcHub("delete", "route", "-n", ocmNS, "-l", insightsMetricsSelector)
		common.OcHub("delete", "clusterrolebinding", roleBindingName)
		common.OcHub("delete", "serviceaccount", saName, "-n", userNamespace)
		common.OcHub("delete", "namespace", "policy-metric-test-compliant")
	})
})
