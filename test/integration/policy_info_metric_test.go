// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"
	"os/exec"

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
	compliantPolicyYaml       = "../resources/policy_info_metric/compliant.yaml"
	compliantPolicyName       = "policy-metric-compliant"
	noncompliantPolicyYaml    = "../resources/policy_info_metric/noncompliant.yaml"
	noncompliantPolicyName    = "policy-metric-noncompliant"
)

var routeURL string

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
			_, err = oc("expose", "service", metricsSvc.Name, "-n", ocmNS, `--overrides={"spec":{"tls":{"termination":"reencrypt"}}}`)
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

		routeURL = routeList.Items[0].Object["spec"].(map[string]interface{})["host"].(string)
		By("Got the metrics route url: " + routeURL)
	})
	It("Checks that the endpoint does not expose metrics without auth", func() {
		Eventually(func() interface{} {
			resp, err := curlMetricsRoute()
			if err != nil {
				return err
			}
			return resp
		}, defaultTimeoutSeconds, 1).Should(ContainSubstring("Unauthorized"))
	})
	It("Checks that endpoint has a HELP comment for the metric", func() {
		By("Creating a policy")
		oc("apply", "-f", compliantPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		// Don't need to check compliance - just need to guarantee there is a policy in the cluster

		token, err := oc("whoami", "-t")
		Expect(err).To(BeNil())
		Eventually(func() interface{} {
			resp, err := curlMetricsRoute("--header", "Authorization: Bearer "+token)
			if err != nil {
				return err
			}
			return resp
		}, defaultTimeoutSeconds, 1).Should(ContainSubstring("HELP " + metricName))
	})
	It("Checks that a compliant policy reports a metric of 0", func() {
		By("Creating a compliant policy")
		oc("apply", "-f", compliantPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		Eventually(
			getComplianceState(compliantPolicyName),
			defaultTimeoutSeconds,
			1,
		).Should(Equal(policiesv1.Compliant))

		By("Checking the policy metric")
		token, err := oc("whoami", "-t")
		Expect(err).To(BeNil())
		regex := `(?m)` + metricName + `{.*policy="` + compliantPolicyName + `.*} 0$`
		Eventually(func() interface{} {
			resp, err := curlMetricsRoute("--header", "Authorization: Bearer "+token)
			if err != nil {
				return err
			}
			return resp
		}, defaultTimeoutSeconds, 1).Should(MatchRegexp(regex))
	})
	It("Checks that a noncompliant policy reports a metric of 1", func() {
		By("Creating a noncompliant policy")
		oc("apply", "-f", noncompliantPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		Eventually(
			getComplianceState(noncompliantPolicyName),
			defaultTimeoutSeconds,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Checking the policy metric")
		token, err := oc("whoami", "-t")
		Expect(err).To(BeNil())
		regex := `(?m)` + metricName + `{.*policy="` + noncompliantPolicyName + `.*} 1$`
		Eventually(func() interface{} {
			resp, err := curlMetricsRoute("--header", "Authorization: Bearer "+token)
			if err != nil {
				return err
			}
			return resp
		}, defaultTimeoutSeconds, 1).Should(MatchRegexp(regex))
	})
	It("Cleans up", func() {
		oc("delete", "-f", compliantPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		oc("delete", "-f", noncompliantPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		oc("delete", "route", "-n", ocmNS, "-l", propagatorMetricsSelector)
	})
})

func oc(args ...string) (string, error) {
	output, err := exec.Command("oc", args...).CombinedOutput()
	if len(args) > 0 && args[0] != "whoami" {
		fmt.Println(string(output))
	}
	return string(output), err
}

func curlMetricsRoute(args ...string) (string, error) {
	argList := []string{"-k", "https://" + routeURL + "/metrics"}
	argList = append(argList, args...)
	output, err := exec.Command("curl", argList...).CombinedOutput()
	return string(output), err
}
