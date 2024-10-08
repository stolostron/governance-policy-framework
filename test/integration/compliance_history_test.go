// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

// Note that this is set to Serial since the cleanup involves restarting the propagator to clear its cache of database
// IDs.
var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the compliance history API", Ordered, Serial, Label("BVT"), func() {
	const policyNS = "policy-test"
	var eventsEndpoint string
	var csvEndpoint string
	var token string
	const saName = "compliance-history-user"

	httpClient := http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	listComplianceEvents := func(ctx context.Context, queryArgs ...string) ([]interface{}, error) {
		url := eventsEndpoint

		if len(queryArgs) > 0 {
			url += "?" + strings.Join(queryArgs, "&")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Got a %d status code", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		respJSON := map[string]any{}

		err = json.Unmarshal(body, &respJSON)
		if err != nil {
			return nil, err
		}

		return respJSON["data"].([]interface{}), nil
	}

	BeforeAll(func(ctx context.Context) {
		By("Setting up Postgres in " + common.OCMNamespace)
		_, err := common.OcHub(
			"apply",
			"-n",
			common.OCMNamespace,
			"-f",
			"../resources/compliance_history/compliance-api-prerequisites.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Creating a service account in the default namespace with access to all managed clusters")
		_, err = common.OcHub("apply", "-f", "../resources/compliance_history/service_account.yaml")
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			By("Getting the service account token")
			secret, err := common.ClientHub.CoreV1().Secrets("default").Get(ctx, saName, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(secret.Data["token"]).ToNot(BeNil())

			token = string(secret.Data["token"])
		}, common.DefaultTimeoutSeconds, 1).Should(Succeed())

		var routeHost string

		Eventually(func(g Gomega) {
			By("Getting the compliance history API route")
			route, err := common.ClientHubDynamic.Resource(common.GvrRoute).Namespace(common.OCMNamespace).Get(
				ctx, "governance-history-api", metav1.GetOptions{},
			)
			g.Expect(err).ToNot(HaveOccurred())

			routeHost, _, _ = unstructured.NestedString(route.Object, "spec", "host")

			g.Expect(routeHost).ToNot(BeEmpty())
		}, common.DefaultTimeoutSeconds, 1).Should(Succeed())

		eventsEndpoint = fmt.Sprintf("https://%s/api/v1/compliance-events", routeHost)
		csvEndpoint = fmt.Sprintf("https://%s/api/v1/reports/compliance-events", routeHost)

		By("Creating the AddOnDeploymentConfig with the complianceHistoryAPIURL variable")
		addonDeploymentConfig := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "addon.open-cluster-management.io/v1alpha1",
				"kind":       "AddOnDeploymentConfig",
				"metadata": map[string]interface{}{
					"name": "governance-policy-framework",
				},
				"spec": map[string]interface{}{
					"customizedVariables": []interface{}{
						map[string]interface{}{
							"name":  "complianceHistoryAPIURL",
							"value": "https://" + routeHost,
						},
					},
				},
			},
		}

		_, err = clientHubDynamic.Resource(common.GvrAddonDeploymentConfig).Namespace(common.OCMNamespace).Create(
			ctx, &addonDeploymentConfig, metav1.CreateOptions{},
		)
		Expect(err).ToNot(HaveOccurred())

		By("Setting the governance-policy-framework ClusterManagementAddOn to use the AddOnDeploymentConfig")
		cma, err := clientHubDynamic.Resource(common.GvrClusterManagementAddOn).Get(
			ctx, "governance-policy-framework", metav1.GetOptions{},
		)
		Expect(err).ToNot(HaveOccurred())

		supportedConfigs := []interface{}{
			map[string]interface{}{
				"group":    "addon.open-cluster-management.io",
				"resource": "addondeploymentconfigs",
				"defaultConfig": map[string]interface{}{
					"name":      "governance-policy-framework",
					"namespace": common.OCMNamespace,
				},
			},
		}

		err = unstructured.SetNestedField(cma.Object, supportedConfigs, "spec", "supportedConfigs")
		Expect(err).ToNot(HaveOccurred())

		_, err = clientHubDynamic.Resource(common.GvrClusterManagementAddOn).Update(ctx, cma, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Verifying the compliance endpoint is up")
		Eventually(func(g Gomega) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, eventsEndpoint, nil)
			g.Expect(err).ToNot(HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := httpClient.Do(req)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(resp.StatusCode).To(
				Equal(http.StatusOK), "expected the compliance API to return the 200 status code",
			)
		}, common.DefaultTimeoutSeconds*4, 1).Should(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		By("Deleting the policy-uninstall-gk policy")
		_, err := common.OcHub(
			"delete",
			"-f",
			"../resources/gatekeeper/policy-uninstall-gk.yaml",
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting Postgres")
		_, err = common.OcHub(
			"delete",
			"-n",
			common.OCMNamespace,
			"-f",
			"../resources/compliance_history/compliance-api-prerequisites.yaml",
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting the service account in the default namespace")
		_, err = common.OcHub(
			"delete", "-f", "../resources/compliance_history/service_account.yaml", "--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Unsetting the governance-policy-framework ClusterManagementAddOn to use the AddOnDeploymentConfig")
		cma, err := clientHubDynamic.Resource(common.GvrClusterManagementAddOn).Get(
			ctx, "governance-policy-framework", metav1.GetOptions{},
		)
		Expect(err).ToNot(HaveOccurred())

		supportedConfigs := []interface{}{
			map[string]interface{}{
				"group":    "addon.open-cluster-management.io",
				"resource": "addondeploymentconfigs",
			},
		}

		err = unstructured.SetNestedField(cma.Object, supportedConfigs, "spec", "supportedConfigs")
		Expect(err).ToNot(HaveOccurred())

		_, err = clientHubDynamic.Resource(common.GvrClusterManagementAddOn).Update(ctx, cma, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Deleting the AddOnDeploymentConfig")
		err = clientHubDynamic.Resource(common.GvrAddonDeploymentConfig).Namespace(common.OCMNamespace).Delete(
			ctx, "governance-policy-framework", metav1.DeleteOptions{},
		)
		if !k8serrors.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("Creates a policy with a compliant and noncompliant configuration policy", func(ctx context.Context) {
		const policyName = "compliance-api-configpolicy"

		startOfTest := time.Now().UTC().Format(time.RFC3339Nano)

		By("Creating the policy")
		_, err := common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the policy")
			_, err := common.OcHub(
				"delete",
				"-f",
				"../resources/compliance_history/policy.yaml",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		clusters := verifyPolicyOnAllClusters(ctx, policyNS, policyName, "NonCompliant", defaultTimeoutSeconds*2)

		expectedEvents := len(clusters) * 2

		By(fmt.Sprintf("Verifying that there are %d compliance events for the parent policy", expectedEvents))
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				// event.timestamp_after is used to filter compliance events from previous runs if the test
				// is run multiple times. This won't be needed after https://issues.redhat.com/browse/ACM-9314 is
				// addressed.
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster,
					"parent_policy.name="+policyName,
					"event.timestamp_after="+startOfTest,
				)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(HaveLen(2), fmt.Sprintf("expected cluster %s to have two events", cluster))

				event1 := events[0].(map[string]interface{})
				event2 := events[1].(map[string]interface{})

				if event1["policy"].(map[string]interface{})["name"].(string) == "default-namespace-must-exist" {
					g.Expect(event1["event"].(map[string]interface{})["compliance"]).To(Equal("Compliant"))
					g.Expect(event2["event"].(map[string]interface{})["compliance"]).To(Equal("NonCompliant"))
				} else {
					g.Expect(event1["event"].(map[string]interface{})["compliance"]).To(Equal("NonCompliant"))
					g.Expect(event2["event"].(map[string]interface{})["compliance"]).To(Equal("Compliant"))
				}
			}
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		now := time.Now().UTC().Format(time.RFC3339Nano)

		By("Deleting a policy template to verify the disabled event")
		_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(policyNS).Patch(
			ctx,
			policyName,
			k8stypes.JSONPatchType,
			[]byte(
				`[{"op": "remove", "path": "/spec/policy-templates/1"}]`,
			),
			metav1.PatchOptions{},
		)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying that there is a single disabled compliance events for the parent policy")
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				events, err := listComplianceEvents(
					ctx,
					"cluster.name="+cluster,
					"parent_policy.name="+policyName,
					"event.timestamp_after="+now,
					"event.compliance=Disabled",
				)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(HaveLen(1), fmt.Sprintf("expected cluster %s to have one disabled events", cluster))

				event := events[0].(map[string]interface{})
				g.Expect(event["policy"].(map[string]interface{})["name"]).To(Equal(
					"does-not-exist-namespace-must-exist",
				))
				eventDetails := event["event"].(map[string]interface{})

				g.Expect(eventDetails["compliance"]).To(Equal("Disabled"))
				g.Expect(eventDetails["message"]).To(Equal("The policy was removed from the parent policy"))
			}
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		now = time.Now().UTC().Format(time.RFC3339Nano)

		_, err = common.OcHub(
			"delete",
			"-f",
			"../resources/compliance_history/policy.yaml",
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying that there is a single disabled compliance event for the parent policy")
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				events, err := listComplianceEvents(
					ctx,
					"cluster.name="+cluster,
					"parent_policy.name="+policyName,
					"event.timestamp_after="+now,
					"event.compliance=Disabled",
				)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(HaveLen(1), fmt.Sprintf("expected cluster %s to have one disabled events", cluster))

				event := events[0].(map[string]interface{})
				g.Expect(event["policy"].(map[string]interface{})["name"]).To(Equal("default-namespace-must-exist"))
				eventDetails := event["event"].(map[string]interface{})

				g.Expect(eventDetails["compliance"]).To(Equal("Disabled"))
				g.Expect(eventDetails["message"]).To(Equal(
					"The policy was removed because the parent policy no longer applies to this cluster",
				))
			}
		}, defaultTimeoutSeconds, 1).Should(Succeed())

		expectedEvents = len(clusters) * 4

		By("Verifying the CSV report")
		Eventually(func(g Gomega) {
			req, err := http.NewRequestWithContext(
				ctx, http.MethodGet, csvEndpoint+"?event.timestamp_after="+startOfTest, nil,
			)
			g.Expect(err).ToNot(HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := httpClient.Do(req)
			g.Expect(err).ToNot(HaveOccurred())

			defer resp.Body.Close()

			g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
			g.Expect(resp.Header.Get("Content-Type")).To(Equal("text/csv"))

			body, err := io.ReadAll(resp.Body)
			g.Expect(err).ToNot(HaveOccurred())
			// Add 1 to expectedEvents to account for the CSV header line
			g.Expect(strings.Split(strings.TrimSpace(string(body)), "\n")).To(HaveLen(expectedEvents + 1))
		}, defaultTimeoutSeconds, 1).Should(Succeed())
	})

	It("Creates a policy with a Gatekeeper constraint", func(ctx context.Context) {
		const installGKPolicyName = "compliance-api-install-gk"
		const uninstallGKPolicyName = "uninstall-gk"
		const gkPrereqPolicyName = "compliance-api-gk-prereq"
		const gkPolicyName = "compliance-api-gk"

		By("Creating the " + installGKPolicyName + " policy to install Gatekeeper")
		_, err := common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy-install-gk.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the " + installGKPolicyName + " policy")
			_, err := common.OcHub(
				"delete",
				"-f",
				"../resources/compliance_history/policy-install-gk.yaml",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			By("Creating the " + uninstallGKPolicyName + " policy to uninstall Gatekeeper")
			_, err = common.OcHub(
				"apply",
				"-n",
				userNamespace,
				"-f",
				"../resources/gatekeeper/policy-uninstall-gk.yaml",
			)
			Expect(err).ToNot(HaveOccurred())

			_ = verifyPolicyOnAllClusters(ctx, policyNS, uninstallGKPolicyName, "Compliant", defaultTimeoutSeconds*2)

			By("Delete the " + uninstallGKPolicyName + " policy")
			_, err = common.OcHub(
				"delete",
				"-n",
				userNamespace,
				"-f",
				"../resources/gatekeeper/policy-uninstall-gk.yaml",
			)
			Expect(err).ToNot(HaveOccurred())

			utils.Kubectl(
				"delete",
				"namespace",
				"openshift-gatekeeper-system",
				"--kubeconfig="+kubeconfigManaged,
				"--ignore-not-found",
			)
		})

		By("Creating an invalid ConfigMap with a policy")
		_, err = common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy-gk-prereq.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the " + gkPrereqPolicyName + " policy")
			_, err = common.OcHub(
				"delete",
				"-f",
				"../resources/compliance_history/policy-gk-prereq.yaml",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		_ = verifyPolicyOnAllClusters(ctx, policyNS, gkPrereqPolicyName, "Compliant", defaultTimeoutSeconds)

		// The audit pod can take a while to become healthy.
		_ = verifyPolicyOnAllClusters(ctx, policyNS, installGKPolicyName, "Compliant", defaultTimeoutSeconds*6)

		now := time.Now().UTC().Format(time.RFC3339Nano)

		By("Creating the " + gkPolicyName + " policy to add Gatekeeper constraints")
		_, err = common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy-gk.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the " + gkPolicyName + " policy")
			_, err := common.OcHub(
				"delete",
				"-f",
				"../resources/compliance_history/policy-gk.yaml",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		clusters := verifyPolicyOnAllClusters(ctx, policyNS, gkPolicyName, "NonCompliant", defaultTimeoutSeconds*2)

		expectedEvents := len(clusters) * 3

		debugMsg := common.RegisterDebugMessage()

		By(fmt.Sprintf("Verifying that there are %d compliance events for the parent policy", expectedEvents))
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				// event.timestamp_after is used to filter compliance events from previous runs if the test
				// is run multiple times. This won't be needed after https://issues.redhat.com/browse/ACM-9314 is
				// addressed.
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster, "parent_policy.name="+gkPolicyName, "event.timestamp_after="+now,
				)
				g.Expect(err).ToNot(HaveOccurred())

				policyToEventDetails := map[string][]map[string]interface{}{}

				for _, event := range events {
					eventTyped := event.(map[string]interface{})

					policyName := eventTyped["policy"].(map[string]interface{})["name"].(string)
					eventDetails := eventTyped["event"].(map[string]interface{})

					policyToEventDetails[policyName] = append(policyToEventDetails[policyName], eventDetails)
				}

				// In case all of these assertions don't pass by the end of the `Eventually`, print the
				// last `policyToEventDetails` in order to try and understand the current state.
				*debugMsg = fmt.Sprintf("Current 'policyToEventDetails': %v", policyToEventDetails)

				// Ensure the ConstraintTemplate has 1 event
				g.Expect(policyToEventDetails["complianceapitest"]).To(HaveLen(1))
				msg := policyToEventDetails["complianceapitest"][0]["message"]
				g.Expect(policyToEventDetails["complianceapitest"][0]["compliance"]).To(
					Equal("Compliant"),
					fmt.Sprintf("The ConstraintTemplate was not compliant on cluster %s: %s", cluster, msg),
				)
				g.Expect(msg).To(Equal("ConstraintTemplate complianceapitest was created successfully"))

				// Ensure the constraint has at least one event.
				g.Expect(policyToEventDetails["compliance-api"]).ToNot(BeEmpty())

				// Sorted by timestamp in descending order
				g.Expect(policyToEventDetails["compliance-api"][0]["compliance"]).To(Equal("NonCompliant"))
				expectedMsg := "warn - All configmaps must have a 'my-gk-test' label (on ConfigMap " +
					"compliance-api-test/compliance-api-test)"
				g.Expect(policyToEventDetails["compliance-api"][0]["message"]).To(
					Equal(expectedMsg),
					"The constraint compliance message didn't match on cluster "+cluster,
				)

				// All other compliance events should be a template-error
				for _, eventDetails := range policyToEventDetails["compliance-api"][1:] {
					g.Expect(eventDetails["compliance"]).To(Equal("NonCompliant"))
					expectedMsg = "template-error; Mapping not found, check if the required ConstraintTemplate has " +
						"been deployed: the resource version was not found: constraints.gatekeeper.sh/v1beta1, " +
						"Kind=ComplianceAPITest"
					g.Expect(eventDetails["message"]).To(
						Equal(expectedMsg), "Unexpected constraint NonCompliant event on cluster "+cluster,
					)
				}
			}
			// It can take a while for the Gatekeeper audit pod to produce audit results.
		}, defaultTimeoutSeconds*6, 1).Should(Succeed())

		By("Updating the ConfigMap to be valid with a policy")
		_, err = common.OcHub(
			"apply",
			"-f",
			"../resources/compliance_history/policy-gk-prereq2.yaml",
		)
		Expect(err).ToNot(HaveOccurred())

		_ = verifyPolicyOnAllClusters(ctx, policyNS, gkPrereqPolicyName, "Compliant", defaultTimeoutSeconds)

		By("Verifying that a compliant event was sent")
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				// event.timestamp_after is used to filter compliance events from previous runs if the test
				// is run multiple times. This won't be needed after https://issues.redhat.com/browse/ACM-9314 is
				// addressed.
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster, "parent_policy.name="+gkPolicyName, "event.timestamp_after="+now,
					"policy.name=compliance-api", "event.compliance=Compliant",
				)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(HaveLen(1), fmt.Sprintf("expected cluster %s to have one compliant event", cluster))

				eventDetails := events[0].(map[string]interface{})["event"].(map[string]interface{})
				g.Expect(eventDetails["compliance"]).To(
					Equal("Compliant"),
					fmt.Sprintf("The constraint was not compliant on cluster %s: %s", cluster, eventDetails["message"]),
				)
				g.Expect(eventDetails["message"]).To(Equal("The constraint has no violations"))
			}
		}, defaultTimeoutSeconds*2, 1).Should(Succeed())
	})

	It("Creates a policy with a compliant and non-compliant cert policy", func(ctx context.Context) {
		const policyName = "cert-policy"
		const prereqPolicyName = "ch-cert-prereq-policy"
		const certPath = "../resources/compliance_history/cert-policy.yaml"
		const nsPolicyPath = "../resources/compliance_history/cert-prereq.yaml"

		now := time.Now().Format(time.RFC3339Nano)

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the cert policy")
			_, err := common.OcHub(
				"delete",
				"-f",
				certPath,
				"-n", policyNS,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting the prereq policy")
			_, err = common.OcHub(
				"delete",
				"-f",
				nsPolicyPath,
				"-n", policyNS,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		By("Creating the prereq policy")
		_, err := common.OcHub(
			"apply",
			"-f",
			nsPolicyPath,
			"-n", policyNS,
		)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying that the policy has created the namespace and secret")
		verifyPolicyOnAllClusters(ctx, policyNS, prereqPolicyName, "Compliant", defaultTimeoutSeconds)

		By("Creating the cert policy")
		_, err = common.OcHub(
			"apply",
			"-f",
			certPath,
			"-n", policyNS,
		)
		Expect(err).ToNot(HaveOccurred())

		By("Checking to see if the cert policy is compliant on the managed clusters")
		clusters := verifyPolicyOnAllClusters(ctx, policyNS, policyName, "Compliant", defaultTimeoutSeconds*2)

		By("Create wrong tls secret")
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).ToNot(HaveOccurred())

		template := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				Organization: []string{"Red Hat"},
			},
			NotBefore: time.Now().Add(time.Hour * -2),
			NotAfter:  time.Now().Add(time.Hour * -1),
			DNSNames:  []string{"www.wrong.com"},
		}
		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
		Expect(err).ToNot(HaveOccurred())

		pemBytes := &bytes.Buffer{}
		err = pem.Encode(pemBytes, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
		Expect(err).ToNot(HaveOccurred())

		tlsEncoded := base64.StdEncoding.EncodeToString(pemBytes.Bytes())

		_, err = common.OcHub("patch", "policies.policy.open-cluster-management.io", prereqPolicyName,
			"-n", policyNS, "--type=json", "-p",
			fmt.Sprintf(`[{
                "op":"add",
                "path":"/spec/policy-templates/1/objectDefinition/spec/`+
				`object-templates/0/objectDefinition/data", 
                "value":{"tls.crt":"%s"}
            }]`, tlsEncoded),
		)
		Expect(err).ToNot(HaveOccurred())

		expectedEvents := 2

		By(fmt.Sprintf("Verifying that there are %d compliance events for the Cert parent policy",
			len(clusters)*expectedEvents))
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster, "parent_policy.name="+policyName, "event.timestamp_after="+now)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(events).To(
					HaveLen(expectedEvents),
					fmt.Sprintf("Verifying that there are %d compliance events per cluster", expectedEvents),
				)

				event0 := events[0].(map[string]interface{})
				g.Expect(event0["event"].(map[string]interface{})["compliance"]).To(Equal("NonCompliant"))
				g.Expect(event0["event"].(map[string]interface{})["message"]).
					To(ContainSubstring("certificates expire in less than 30h0m0s"))
				g.Expect(event0["event"].(map[string]interface{})["message"]).
					To(ContainSubstring("certificates defined SAN entries do not match pattern Allowed: .*.test.com"))

				event1 := events[1].(map[string]interface{})
				g.Expect(event1["event"].(map[string]interface{})["compliance"]).To(Equal("Compliant"))
				g.Expect(event1["event"].(map[string]interface{})["message"]).
					To(ContainSubstring("All evaluated certificates are compliant"))

			}
		}, 120, 1).Should(Succeed())
	})

	It("Creates a policy with a noncompliant and compliant operator policy", func(ctx context.Context) {
		now := time.Now().Format(time.RFC3339Nano)
		const opPolicyPath = "../resources/compliance_history/operator-policy-invalid.yaml"
		const policyName = "op-compliance-api"
		const nsPolicyPath = "../resources/compliance_history/operator-prereq.yaml"
		const prereqPolicyName = "ch-operator-prereq-policy"

		DeferCleanup(func(ctx context.Context) {
			By("Deleting the operator policy")
			_, err := common.OcHub(
				"delete",
				"-f",
				opPolicyPath,
				"-n", policyNS,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			// Delete subscription, catalog, etc at once
			By("Deleting the namespace policy")
			_, err = common.OcHub(
				"delete",
				"-f",
				nsPolicyPath,
				"-n", policyNS,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		By("Creating the namespace")
		_, err := common.OcHub(
			"apply",
			"-f",
			nsPolicyPath,
			"-n", policyNS,
		)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying that the policy has created the namespace")
		verifyPolicyOnAllClusters(ctx, policyNS, prereqPolicyName, "Compliant", defaultTimeoutSeconds)

		By("Creating the Policy")
		_, err = common.OcHub(
			"apply",
			"-f",
			opPolicyPath,
			"-n", policyNS,
		)
		Expect(err).ToNot(HaveOccurred())

		clusters := verifyPolicyOnAllClusters(ctx, policyNS, policyName, "NonCompliant", defaultTimeoutSeconds*2)

		By("Verifying that there are NonCompliant compliance events for the Operator parent policy")
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster, "parent_policy.name="+policyName, "event.timestamp_after="+now)
				g.Expect(err).ToNot(HaveOccurred())

				event0 := events[0].(map[string]interface{})
				g.Expect(event0["event"].(map[string]interface{})["compliance"]).To(Equal("NonCompliant"))
				g.Expect(event0["event"].(map[string]interface{})["message"]).
					To(ContainSubstring("CatalogSource 'invalid' was not found"))
			}
		}, 120, 1).Should(Succeed())

		now = time.Now().Format(time.RFC3339Nano)

		By("Patch operator Policy to be Compliant")
		_, err = common.OcHub("patch", "policies.policy.open-cluster-management.io", policyName,
			"-n", policyNS, "--type=json", "-p", `[{
			"op":"replace",
			"path":"/spec/policy-templates/0/objectDefinition/spec/subscription/source",
			"value":"redhat-operators"
		}]`)
		Expect(err).ToNot(HaveOccurred())
		_, err = common.OcHub("patch", "policies.policy.open-cluster-management.io", policyName,
			"-n", policyNS, "--type=json", "-p", `[{
		"op":"replace",
		"path":"/spec/remediationAction",
		"value":"enforce"
		 }]`)
		Expect(err).ToNot(HaveOccurred())

		debugMsg := common.RegisterDebugMessage()

		By("The Policy should be Compliant on all clusters")
		Eventually(func(g Gomega) {
			*debugMsg = ""

			for _, cluster := range clusters {
				replPol, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(cluster).Get(
					ctx, policyNS+"."+policyName, metav1.GetOptions{},
				)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(replPol).NotTo(BeNil())

				details, found, err := unstructured.NestedSlice(replPol.Object, "status", "details")
				g.Expect(found).To(BeTrue())
				g.Expect(err).NotTo(HaveOccurred())

				det, ok := details[0].(map[string]interface{})
				g.Expect(ok).To(BeTrue())

				history, found, err := unstructured.NestedSlice(det, "history")
				g.Expect(found).To(BeTrue())
				g.Expect(err).NotTo(HaveOccurred())

				item, ok := history[0].(map[string]interface{})
				g.Expect(ok).To(BeTrue())

				msg, found, err := unstructured.NestedString(item, "message")
				g.Expect(found).To(BeTrue())
				g.Expect(err).NotTo(HaveOccurred())

				*debugMsg += fmt.Sprintf("Cluster %v current compliance message: %v\n", cluster, msg)
			}

			clusters = confirmComplianceOnAllClusters(ctx, policyNS, policyName, "Compliant")(g)
		}, defaultTimeoutSeconds*2, 1).Should(Succeed())

		// We don't know the patch version ahead of time and it could be different per managed cluster depending on the
		// OCP version, so just allow 3.8.* in the message.
		expectedMsg := regexp.QuoteMeta("the policy spec is valid, the OperatorGroup matches what "+
			"is required by the policy, the Subscription matches what is required by the policy, "+
			"no InstallPlans requiring approval were found, ClusterServiceVersion (quay-operator.v3.8.") + `\d+` +
			regexp.QuoteMeta(") - install strategy"+
				" completed with no errors, there are CRDs present for the operator, all operator "+
				"Deployments have their minimum availability, CatalogSource was found")

		By("Verifying that there are Compliant compliance events for the Operator parent policy")
		Eventually(func(g Gomega) {
			for _, cluster := range clusters {
				events, err := listComplianceEvents(
					ctx, "cluster.name="+cluster, "parent_policy.name="+policyName, "event.timestamp_after="+now)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(events).ToNot(BeEmpty())

				event0 := events[0].(map[string]interface{})
				g.Expect(event0["event"].(map[string]interface{})["compliance"]).To(Equal("Compliant"))
				g.Expect(event0["event"].(map[string]interface{})["message"]).To(MatchRegexp(expectedMsg))
			}
		}, 120, 1).Should(Succeed())
	})
})

func verifyPolicyOnAllClusters(
	ctx context.Context, namespace string, policy string, compliance string, timeout int,
) (
	clusters []string,
) {
	GinkgoHelper()

	By(fmt.Sprintf("Verifying that the policy %s/%s is %s", namespace, policy, compliance))

	Eventually(func(g Gomega) {
		clusters = confirmComplianceOnAllClusters(ctx, namespace, policy, compliance)(g)
	}, timeout, 1).Should(Succeed())

	return
}

func confirmComplianceOnAllClusters(
	ctx context.Context, namespace string, policy string, compliance string,
) func(g Gomega) []string {
	return func(g Gomega) []string {
		GinkgoHelper()

		clusters := []string{}

		parentPolicy, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(namespace).Get(
			ctx, policy, metav1.GetOptions{},
		)
		g.Expect(err).ToNot(HaveOccurred())

		perClusterStatus, _, _ := unstructured.NestedSlice(parentPolicy.Object, "status", "status")
		g.Expect(perClusterStatus).ToNot(BeEmpty(), "no cluster status was available on the parent policy")

		for _, clusterStatus := range perClusterStatus {
			clusterStatus, ok := clusterStatus.(map[string]interface{})
			g.Expect(ok).To(BeTrue(), "the cluster status was not the right type")

			g.Expect(clusterStatus["compliant"]).To(Equal(compliance))
			g.Expect(clusterStatus["clustername"]).ToNot(BeEmpty())
			clusters = append(clusters, clusterStatus["clustername"].(string))
		}

		return clusters
	}
}
