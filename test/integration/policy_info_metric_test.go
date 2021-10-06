// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-framework/test/common"
	policiesv1 "github.com/open-cluster-management/governance-policy-propagator/pkg/apis/policy/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	propagatorMetricsSelector = "component=ocm-policy-propagator"
	ocmNS                     = "open-cluster-management"
	metricName                = "policy_governance_info"
	saName                    = "grc-framework-sa"
	roleBindingName           = "grc-framework-role-binding"
	compliantPolicyYaml       = "../resources/policy_info_metric/compliant.yaml"
	compliantPolicyName       = "policy-metric-compliant"
	noncompliantPolicyYaml    = "../resources/policy_info_metric/noncompliant.yaml"
	noncompliantPolicyName    = "policy-metric-noncompliant"
)

var propagatorMetricsURL string

var metricToken string

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test policy_governance_info metric", func() {
	It("Sets up the metrics service endpoint for tests", func() {
		By("Create Namespace if needed")
		_, err := clientHub.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: userNamespace,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			Expect(errors.IsAlreadyExists(err)).Should(BeTrue())
		}
		Expect(clientHub.CoreV1().Namespaces().Get(context.TODO(), userNamespace, metav1.GetOptions{})).NotTo(BeNil())

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
		metricToken = string(decodedToken)
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

		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(propagatorMetricsURL, strings.TrimSpace(metricToken))
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
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.Compliant))

		By("Checking the policy metric")
		policyLabel := `policy="` + compliantPolicyName + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(propagatorMetricsURL, strings.TrimSpace(metricToken))
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
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Checking the policy metric")
		policyLabel := `policy="` + noncompliantPolicyName + `"`
		Eventually(func() interface{} {
			resp, _, err := common.GetWithToken(propagatorMetricsURL, strings.TrimSpace(metricToken))
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
		common.OcHub("delete", "clusterrolebinding", roleBindingName)
		common.OcHub("delete", "serviceaccount", saName, "-n", userNamespace)
		common.OcHub("delete", "namespace", userNamespace)
		common.OcHub("delete", "namespace", "policy-metric-test-compliant")
	})
})
