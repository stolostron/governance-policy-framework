// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"encoding/base64"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test policy_governance_info metric", Ordered, Label("BVT"), func() {
	const (
		propagatorMetricsSelector = "component=ocm-policy-propagator"
		ocmNS                     = "open-cluster-management"
		metricName                = "policy_governance_info"
		metricsAccName            = "grc-framework-sa-metrics"
		metricsTokenYaml          = "../resources/policy_info_metric/metrics_token.yaml"
		metricsTokenName          = "grc-framework-sa-metrics-token-manual"
		metricsRoleName           = "grc-framework-metrics-reader"
		metricsRoleYaml           = "../resources/policy_info_metric/metrics_role.yaml"
		noMetricsAccName          = "grc-framework-sa-nometrics"
		noMetricsTokenYaml        = "../resources/policy_info_metric/nometrics_token.yaml"
		noMetricsTokenName        = "grc-framework-sa-nometrics-token-manual"
		roleBindingName           = "grc-framework-metrics-reader-binding"
		compliantPolicyYaml       = "../resources/policy_info_metric/compliant.yaml"
		compliantPolicyName       = "policy-metric-compliant"
		noncompliantPolicyYaml    = "../resources/policy_info_metric/noncompliant.yaml"
		noncompliantPolicyName    = "policy-metric-noncompliant"
	)

	var (
		metricsToken         string
		noMetricsToken       string
		propagatorMetricsURL string
	)

	It("Sets up the metrics service endpoint for tests", func() {
		By("Ensuring the metrics service exists")
		svcList, err := clientHub.CoreV1().Services(ocmNS).List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: propagatorMetricsSelector,
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(svcList.Items).To(HaveLen(1))
		metricsSvc := svcList.Items[0]

		By("Checking for an existing metrics route")
		var routeList *unstructured.UnstructuredList
		Eventually(func() interface{} {
			var err error
			routeList, err = clientHubDynamic.Resource(common.GvrRoute).Namespace(ocmNS).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: propagatorMetricsSelector,
				},
			)
			if err != nil {
				return err
			}

			return len(routeList.Items)
		}, defaultTimeoutSeconds, 1).Should(Or(Equal(0), Equal(1)))

		if len(routeList.Items) == 0 {
			By("Exposing the metrics service as a route")
			_, err = common.OcHub(
				"create",
				"route",
				"reencrypt",
				"--service="+metricsSvc.Name,
				"--namespace="+ocmNS,
			)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() interface{} {
				var err error
				routeList, err = clientHubDynamic.Resource(common.GvrRoute).Namespace(ocmNS).List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: propagatorMetricsSelector,
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
		propagatorMetricsURL = "https://" + routeHost + "/metrics"
	})
	It("Sets up a ServiceAccount without permissions for metrics", func() {
		_, err := common.OcHub("create", "serviceaccount", noMetricsAccName, "-n", userNamespace)
		Expect(err).ToNot(HaveOccurred())

		_, err = common.OcHub("apply", "-f", noMetricsTokenYaml, "-n", userNamespace)
		Expect(err).ToNot(HaveOccurred())

		var encodedtoken string

		// The secret could take a moment to be populated with the token
		Eventually(func(g Gomega) {
			var err error
			encodedtoken, err = common.OcHub("get", "secret", noMetricsTokenName,
				"-n", userNamespace, "-o", "jsonpath={.data.token}")

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(encodedtoken).ToNot(BeEmpty())
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		decodedToken, err := base64.StdEncoding.DecodeString(encodedtoken)
		Expect(err).ToNot(HaveOccurred())

		noMetricsToken = string(decodedToken)
	})
	It("Sets up a ServiceAccount with specific permission for metrics", func() {
		_, err := common.OcHub("create", "serviceaccount", metricsAccName, "-n", userNamespace)
		Expect(err).ToNot(HaveOccurred())

		_, err = common.OcHub("apply", "-f", metricsRoleYaml, "-n", userNamespace)
		Expect(err).ToNot(HaveOccurred())

		_, err = common.OcHub("create", "clusterrolebinding", roleBindingName,
			"--clusterrole="+metricsRoleName, "--serviceaccount="+userNamespace+":"+metricsAccName)
		Expect(err).ToNot(HaveOccurred())

		_, err = common.OcHub("apply", "-f", metricsTokenYaml, "-n", userNamespace)
		Expect(err).ToNot(HaveOccurred())

		var encodedtoken string

		// The secret could take a moment to be populated with the token
		Eventually(func(g Gomega) {
			var err error
			encodedtoken, err = common.OcHub("get", "secret", metricsTokenName,
				"-n", userNamespace, "-o", "jsonpath={.data.token}")

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(encodedtoken).ToNot(BeEmpty())
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		decodedToken, err := base64.StdEncoding.DecodeString(encodedtoken)
		Expect(err).ToNot(HaveOccurred())

		metricsToken = string(decodedToken)
	})
	It("Checks that the endpoint does not expose metrics to unauthenticated users", func() {
		Eventually(func() interface{} {
			_, status, err := common.GetWithToken(propagatorMetricsURL, "")
			if err != nil {
				return err
			}

			return status
		}, "90s", 1).Should(ContainSubstring("Unauthorized"))
	})
	It("Checks that the endpoint does not expose metrics to users without authorization", func() {
		Eventually(func() interface{} {
			_, status, err := common.GetWithToken(
				propagatorMetricsURL,
				strings.TrimSpace(noMetricsToken),
			)
			if err != nil {
				return err
			}

			return status
		}, defaultTimeoutSeconds, 1).Should(ContainSubstring("403 Forbidden"))
	})
	It("Checks that endpoint has a HELP comment for the metric", func() {
		By("Creating a policy")
		_, err := common.OcHub("apply", "-f", compliantPolicyYaml, "-n", userNamespace)
		Expect(err).ToNot(HaveOccurred())
		// Don't need to check compliance - just need to guarantee there is a policy in the cluster

		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(
				propagatorMetricsURL,
				strings.TrimSpace(metricsToken),
			)
			if err != nil {
				return err
			}

			return resp
		}, defaultTimeoutSeconds, 1).Should(ContainSubstring("HELP " + metricName))
	})
	It("Checks that a compliant policy reports a metric of 0", func() {
		By("Creating a compliant policy")
		_, err := common.OcHub("apply", "-f", compliantPolicyYaml, "-n", userNamespace)
		Expect(err).ToNot(HaveOccurred())
		Eventually(
			common.GetComplianceState(compliantPolicyName),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.Compliant))

		By("Checking the policy metric")
		policyLabel := `policy="` + compliantPolicyName + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(
				propagatorMetricsURL,
				strings.TrimSpace(metricsToken),
			)
			if err != nil {
				return err
			}

			return resp
		}, defaultTimeoutSeconds, 1).Should(common.MatchMetricValue(metricName, policyLabel, "0"))
	})
	It("Checks that a noncompliant policy reports a metric of 1", func() {
		By("Creating a noncompliant policy")
		_, err := common.OcHub("apply", "-f", noncompliantPolicyYaml, "-n", userNamespace)
		Expect(err).ToNot(HaveOccurred())
		Eventually(
			common.GetComplianceState(noncompliantPolicyName),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Checking the policy metric")
		policyLabel := `policy="` + noncompliantPolicyName + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(
				propagatorMetricsURL,
				strings.TrimSpace(metricsToken),
			)
			if err != nil {
				return err
			}

			return resp
		}, defaultTimeoutSeconds, 1).Should(common.MatchMetricValue(metricName, policyLabel, "1"))
	})
	AfterAll(func() {
		_, err := common.OcHub("delete", "-f", compliantPolicyYaml, "-n", userNamespace, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "-f", noncompliantPolicyYaml, "-n", userNamespace, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "route", "-n", ocmNS, "-l", propagatorMetricsSelector, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "clusterrole", metricsRoleName, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "clusterrolebinding", roleBindingName, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "serviceaccount", metricsAccName, "-n", userNamespace, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "serviceaccount", noMetricsAccName, "-n", userNamespace, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "secret", metricsTokenName, "-n", userNamespace, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "secret", noMetricsTokenName, "-n", userNamespace, "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("delete", "namespace", "policy-metric-test-compliant", "--ignore-not-found")
		Expect(err).ToNot(HaveOccurred())
	})
})
