// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stolostron/governance-policy-framework/test/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test required metrics are available", Ordered, Label("BVT"), func() {
	const (
		metricsAccName         = "grc-e2e-metrics-test"
		metricsTokenName       = "grc-e2e-metrics-test-token-manual"
		metricsTokenYAML       = "../resources/verify_metrics/metrics_token.yaml"
		monitoringNS           = "openshift-monitoring"
		noncompliantPolicyName = "policy-verify-metrics-noncompliant"
		noncompliantPolicyYAML = "../resources/verify_metrics/noncompliant.yaml"
		prometheusRouteName    = "prometheus-k8s"
		roleBindingName        = "grc-e2e-metrics-test"
	)
	// Note that the spec-sync metrics are skipped due to it not being available on a self-managed Hub. The presence
	// of the other sync metrics indicate that the metrics are exported from governance-policy-framework addon properly.
	// Other metrics that require error conditions to show up are also skipped.
	requiredMetrics := []string{
		"config_policies_evaluation_duration_seconds_bucket",
		"config_policies_evaluation_duration_seconds_count",
		"config_policy_evaluation_seconds_total",
		"config_policy_evaluation_total",
		`controller_runtime_reconcile_errors_total{controller="policy-encryption-keys"}`,
		`controller_runtime_reconcile_errors_total{controller="policy-set"}`,
		`controller_runtime_reconcile_errors_total{controller="policy-propagator"}`,
		`controller_runtime_reconcile_errors_total{controller="policy-status-sync"}`,
		`controller_runtime_reconcile_time_seconds_bucket{controller="policy-propagator"}`,
		`controller_runtime_reconcile_total{controller="policy-propagator"}`,
		"ocm_handle_root_policy_duration_seconds_bucket_bucket",
		`workqueue_depth{name="policy-status-sync"}`,
		`workqueue_depth{name="policy-template-sync"}`,
	}

	AfterEach(func() {
		_, err := common.OcHub("delete", "-f", noncompliantPolicyYAML, "-n", userNamespace, "--ignore-not-found")
		Expect(err).To(BeNil())
		_, err = common.OcHub("delete", "secret", metricsTokenName, "-n", userNamespace, "--ignore-not-found")
		Expect(err).To(BeNil())
		_, err = common.OcHub("delete", "clusterrolebinding", roleBindingName, "--ignore-not-found")
		Expect(err).To(BeNil())
		_, err = common.OcHub("delete", "serviceaccount", metricsAccName, "-n", userNamespace, "--ignore-not-found")
		Expect(err).To(BeNil())
	})

	It("Verifies all required metrics are available", func() {
		By("Creating a noncompliant policy")
		_, err := common.OcHub("apply", "-f", noncompliantPolicyYAML, "-n", userNamespace)
		Expect(err).To(BeNil())
		Eventually(
			common.GetComplianceState(noncompliantPolicyName),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Finding the Prometheus route")
		route, err := clientHubDynamic.Resource(common.GvrRoute).Namespace(monitoringNS).Get(
			context.TODO(), prometheusRouteName, metav1.GetOptions{},
		)
		Expect(err).To(BeNil())

		prometheusHost, _, _ := unstructured.NestedString(route.Object, "spec", "host")
		Expect(prometheusHost).ToNot(HaveLen(0))

		prometheusURL := "https://" + prometheusHost + "/api/v1/query"

		By("Getting a token for a new service account")
		_, err = common.OcHub("create", "serviceaccount", metricsAccName, "-n", userNamespace)
		Expect(err).To(BeNil())

		_, err = common.OcHub(
			"create",
			"clusterrolebinding",
			roleBindingName,
			"--clusterrole=cluster-admin",
			"--serviceaccount="+userNamespace+":"+metricsAccName,
		)
		Expect(err).To(BeNil())

		_, err = common.OcHub("apply", "-f", metricsTokenYAML, "-n", userNamespace)
		Expect(err).To(BeNil())

		var encodedtoken string

		// The secret could take a moment to be populated with the token
		Eventually(func(g Gomega) {
			var err error
			encodedtoken, err = common.OcHub(
				"get", "secret", metricsTokenName, "-n", userNamespace, "-o", "jsonpath={.data.token}",
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(encodedtoken).ToNot(HaveLen(0))
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		decodedToken, err := base64.StdEncoding.DecodeString(encodedtoken)
		Expect(err).To(BeNil())

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient := http.Client{Timeout: 15 * time.Second, Transport: transport}

		for _, metric := range requiredMetrics {
			By("Checking the metric " + metric)
			// Timeout after 60 seconds since this is double the Prometheus scrape time, so it should show up by then.
			Eventually(func(g Gomega) {
				req, err := http.NewRequest(http.MethodGet, prometheusURL, nil)
				g.Expect(err).To(BeNil())

				req.Header.Set("Authorization", "Bearer "+string(decodedToken))
				req.Header.Set("Accept", "application/json")

				q := req.URL.Query()
				q.Add("query", metric)
				req.URL.RawQuery = q.Encode()

				res, err := httpClient.Do(req)
				g.Expect(err).To(BeNil())

				resBody, err := io.ReadAll(res.Body)
				g.Expect(err).To(BeNil())

				resJSON := map[string]interface{}{}
				err = json.Unmarshal(resBody, &resJSON)
				g.Expect(err).To(BeNil())
				g.Expect(resJSON["status"]).To(Equal("success"))

				data, ok := resJSON["data"].(map[string]interface{})
				g.Expect(ok).To(BeTrue())

				result, ok := data["result"].([]interface{})
				g.Expect(ok).To(BeTrue())
				g.Expect(result).ToNot(HaveLen(0))
			}, "60s", 1).Should(Succeed())
		}
	})
})
