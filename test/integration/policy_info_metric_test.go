// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"strings"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-framework/test/common"
	policiesv1 "github.com/open-cluster-management/governance-policy-propagator/pkg/apis/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	propagatorMetricsSelector = "component=ocm-policy-propagator"
	ocmNS                     = "open-cluster-management"
	metricName                = "policy_governance_info"
	saName					  = "grc-framework-sa"
	saNamespace				  = "kube-system"
	roleBindingName			  = "grc-framework-role-binding"
	compliantPolicyYaml       = "../resources/policy_info_metric/compliant.yaml"
	compliantPolicyName       = "policy-metric-compliant"
	noncompliantPolicyYaml    = "../resources/policy_info_metric/noncompliant.yaml"
	noncompliantPolicyName    = "policy-metric-noncompliant"
)

var propagatorMetricsURL string

var _ = Describe("Test policy_governance_info metric", func() {
	It("Sets up the metrics service endpoint for tests", func() {
		By("Ensuring the metrics service exists")
		svcList, err := clientHub.CoreV1().Services(ocmNS).List(context.TODO(), metav1.ListOptions{LabelSelector: propagatorMetricsSelector})
		Expect(err).To(BeNil())
		Expect(len(svcList.Items)).To(Equal(1))
		metricsSvc := svcList.Items[0]

		By("Checking for an existing metrics route")
		var routeList *unstructured.UnstructuredList
		Eventually(func() interface{} {
			var err error
			routeList, err = clientHubDynamic.Resource(common.GvrRoute).Namespace(ocmNS).List(context.TODO(), metav1.ListOptions{LabelSelector: propagatorMetricsSelector})
			if err != nil {
				return err
			}
			return len(routeList.Items)
		}, defaultTimeoutSeconds, 1).Should(Or(Equal(0), Equal(1)))

		if len(routeList.Items) == 0 {
			By("Exposing the metrics service as a route")
			_, err = common.OcHub("expose", "service", metricsSvc.Name, "-n", ocmNS, `--overrides={"spec":{"tls":{"termination":"reencrypt"}}}`)
			Expect(err).To(BeNil())

			Eventually(func() interface{} {
				var err error
				routeList, err = clientHubDynamic.Resource(common.GvrRoute).Namespace(ocmNS).List(context.TODO(), metav1.ListOptions{LabelSelector: propagatorMetricsSelector})
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
	It("Checks that the endpoint does not expose metrics without auth", func() {
		Eventually(func() interface{} {
			_, status, err := common.GetWithToken(propagatorMetricsURL, "")
			if err != nil {
				return err
			}
			return status
		}, defaultTimeoutSeconds, 1).Should(ContainSubstring("Unauthorized"))
	})
	It("Checks that endpoint has a HELP comment for the metric", func() {
		By("Creating a policy")
		common.OcHub("apply", "-f", compliantPolicyYaml, "-n", userNamespace)
		// Don't need to check compliance - just need to guarantee there is a policy in the cluster

		//get token from SA
		_, err := common.OcHub("create", "serviceaccount", saName, "-n", saNamespace)
		Expect(err).To(BeNil())
		_, err = common.OcHub("create", "clusterrolebinding", roleBindingName, "--clusterrole=cluster-admin", fmt.Sprintf("--serviceaccount=%s:%s", saNamespace, saName))
		Expect(err).To(BeNil())
		tokenName, err := common.OcHub("get", fmt.Sprintf("serviceaccount/%s", saName), "-n", saNamespace, "-o", "jsonpath='{.secrets[0].name}'")
		Expect(err).To(BeNil())
		token, err := common.OcHub("get", "secret", tokenName, "-n", saNamespace, "-o", "jsonpath='{.data.token}'| base64 --decode")
		Expect(err).To(BeNil())

		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(propagatorMetricsURL, strings.TrimSpace(token))
			if err != nil {
				return err
			}
			return resp
		}, defaultTimeoutSeconds, 1).Should(ContainSubstring("HELP " + metricName))
	})
	It("Checks that a compliant policy reports a metric of 0", func() {
		By("Creating a compliant policy")
		common.OcHub("apply", "-f", compliantPolicyYaml, "-n", userNamespace)
		Eventually(
			getComplianceState(compliantPolicyName),
			defaultTimeoutSeconds,
			1,
		).Should(Equal(policiesv1.Compliant))

		By("Checking the policy metric")
		token, err := common.OcHub("whoami", "-t")
		Expect(err).To(BeNil())
		policyLabel := `policy="` + compliantPolicyName + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(propagatorMetricsURL, strings.TrimSpace(token))
			if err != nil {
				return err
			}
			return resp
		}, defaultTimeoutSeconds, 1).Should(common.MatchMetricValue(metricName, policyLabel, "0"))
	})
	It("Checks that a noncompliant policy reports a metric of 1", func() {
		By("Creating a noncompliant policy")
		common.OcHub("apply", "-f", noncompliantPolicyYaml, "-n", userNamespace)
		Eventually(
			getComplianceState(noncompliantPolicyName),
			defaultTimeoutSeconds,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Checking the policy metric")
		token, err := common.OcHub("whoami", "-t")
		Expect(err).To(BeNil())
		policyLabel := `policy="` + noncompliantPolicyName + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(propagatorMetricsURL, strings.TrimSpace(token))
			if err != nil {
				return err
			}
			return resp
		}, defaultTimeoutSeconds, 1).Should(common.MatchMetricValue(metricName, policyLabel, "1"))
	})
	It("Cleans up", func() {
		common.OcHub("delete", "-f", compliantPolicyYaml, "-n", userNamespace)
		common.OcHub("delete", "-f", noncompliantPolicyYaml, "-n", userNamespace)
		common.OcHub("delete", "route", "-n", ocmNS, "-l", propagatorMetricsSelector)
	})
})
