// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"
	"strings"
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-framework/test/common"
	policiesv1 "github.com/open-cluster-management/governance-policy-propagator/pkg/apis/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	insightsClientSelector = "component=insights-client"
	insightsMetricsSelector = "component=insights-metrics"
	insightsMetricName           = "policyreport_info"
	noncompliantPolicyYamlReport   	= "../resources/policy_report_metric/noncompliant.yaml"
	noncompliantPolicyNameReport    = "policyreport-metric-noncompliant"
	compliantPolicyYamlReport   	= "../resources/policy_report_metric/compliant.yaml"
	compliantPolicyNameReport    = "policyreport-metric-noncompliant"
)

var insightsMetricsURL string

var insightsToken string

var _ = Describe("Test policyreport_info metric", func() {
	It("Sets up the metrics service endpoint for tests", func() {
		By("Create Namespace if needed")
		_, err := clientHub.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: userNamespace,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			fmt.Println("---- create ns error ----")
			fmt.Println(err)
			Expect(errors.IsAlreadyExists(err)).Should(BeTrue())
		}
		Expect(clientHub.CoreV1().Namespaces().Get(context.TODO(), userNamespace, metav1.GetOptions{})).NotTo(BeNil())

		//set up insights-client to poll every minute
		insightsClient, err := common.OcHub("get", "deployments", "-n", ocmNS, "-l", insightsClientSelector, "-o", "name")
		Expect(err).To(BeNil())
		insightsClient = strings.TrimSpace(insightsClient)
		_, err = common.OcHub("set", "env", "-n", ocmNS, insightsClient, "POLL_INTERVAL=1")
		Expect(err).To(BeNil())

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

		//get auth token from service account
		By("Setting up ServiceAccount for authentication")
		_, err = common.OcHub("create", "serviceaccount", saName, "-n", userNamespace)
		Expect(err).To(BeNil())
		_, err = common.OcHub("create", "clusterrolebinding", roleBindingName, "--clusterrole=cluster-admin", fmt.Sprintf("--serviceaccount=%s:%s", userNamespace, saName))
		Expect(err).To(BeNil())
		tokenNames, err := common.OcHub("get", fmt.Sprintf("serviceaccount/%s", saName), "-n", userNamespace, "-o", "jsonpath={.secrets[*].name}")
		Expect(err).To(BeNil())
		tokenNameArr := strings.Split(tokenNames, " ")
		var tokenName string
		for _, name := range tokenNameArr {
			if strings.HasPrefix(name, saName+"-token-") {
				tokenName = name
			}
		}
		encodedtoken, err := common.OcHub("get", "secret", tokenName, "-n", userNamespace, "-o", "jsonpath={.data.token}")
		Expect(err).To(BeNil())
		decodedToken, err := base64.StdEncoding.DecodeString(encodedtoken)
		Expect(err).To(BeNil())
		insightsToken = string(decodedToken)
	})
	It("Checks that a noncompliant policy reports a metric", func() {
		By("Creating a noncompliant policy")
		common.OcHub("apply", "-f", noncompliantPolicyYamlReport, "-n", userNamespace)
		Eventually(
			getComplianceState(noncompliantPolicyNameReport),
			defaultTimeoutSeconds,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Checking the policy metric")
		policyLabel := `policy="` + userNamespace + "." + noncompliantPolicyNameReport + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(insightsMetricsURL, strings.TrimSpace(insightsToken))
			if err != nil {
				return err
			}
			return resp
		}, 120, 1).Should(common.MatchMetricValue(insightsMetricName, policyLabel, "1"))
	})
	It("Checks that changing the policy to compliant removes the metric", func() {
		By("Creating a compliant policy")
		common.OcHub("apply", "-f", compliantPolicyYamlReport, "-n", userNamespace)
		Eventually(
			getComplianceState(compliantPolicyNameReport),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.Compliant))

		By("Checking the policy metric displays nothing")
		policyLabel := `policy="` + userNamespace + "." + noncompliantPolicyNameReport + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(insightsMetricsURL, strings.TrimSpace(insightsToken))
			if err != nil {
				return err
			}
			return resp
		}, defaultTimeoutSeconds*2, 1).Should(common.MatchMetricValue(insightsMetricName, policyLabel, "1"))
	})
	It("Cleans up", func() {
		common.OcHub("delete", "-f", compliantPolicyYamlReport, "-n", userNamespace)
		common.OcHub("delete", "route", "-n", ocmNS, "-l", insightsMetricsSelector)
		common.OcHub("delete", "clusterrolebinding", roleBindingName)
		common.OcHub("delete", "serviceaccount", saName, "-n", userNamespace)
		common.OcHub("delete", "namespace", userNamespace)
		common.OcHub("delete", "namespace", "policy-metric-test-compliant")
	})
})
