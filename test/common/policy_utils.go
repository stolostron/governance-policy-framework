// Copyright Contributors to the Open Cluster Management project

package common

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"
)

// GetComplianceState returns a function usable by ginkgo.Eventually that retrieves the
// compliance state of the input policy in the globally configured managed cluster.
func GetComplianceState(policyName string) func(Gomega) interface{} {
	return GetClusterComplianceState(policyName, ClusterNamespaceOnHub)
}

// GetClusterComplianceState returns a function usable by ginkgo.Eventually that retrieves the
// compliance state of the input policy on the specified cluster.
func GetClusterComplianceState(policyName, clusterName string) func(Gomega) interface{} {
	return func(g Gomega) interface{} {
		rootPlc := utils.GetWithTimeout(
			ClientHubDynamic, GvrPolicy, policyName, UserNamespace, true, DefaultTimeoutSeconds,
		)
		var policy policiesv1.Policy
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(rootPlc.UnstructuredContent(), &policy)
		g.Expect(err).ToNot(HaveOccurred())

		for _, statusPerCluster := range policy.Status.Status {
			if statusPerCluster.ClusterNamespace == clusterName {
				return statusPerCluster.ComplianceState
			}
		}

		return nil
	}
}

// Patches the requiredClusterSelector of the specified Placement so that it will
// always only match the targetCluster.
func PatchPlacement(namespace, name string) error {
	By("Patching Placement " + namespace + "/" + name +
		" with requiredClusterSelector {name: " + ClusterNamespaceOnHub + "}")

	_, err := OcHub(
		"patch",
		"-n",
		namespace,
		"placements.cluster.open-cluster-management.io",
		name,
		"--type=json",
		`-p=[{
			"op": "replace",
			"path": "/spec/predicates/0/requiredClusterSelector/labelSelector",
			"value":{"matchExpressions":[{"key": "name", "operator": "In", "values": ["`+ClusterNamespaceOnHub+`"]}]}
		}]`,
	)

	return err
}

// CreatePlacementDecisionStatus creates a PlacementDecision for the specified
// Placement and returns the created decision.
func CreatePlacementDecision(ctx context.Context, namespace, placementName string) (*unstructured.Unstructured, error) {
	pldName := placementName + "-1"

	By("Creating PlacementDecision for Placement " + namespace + "/" + placementName)
	placementDecision := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": GvrPlacementDecision.Group + "/" + GvrPlacementDecision.Version,
			"kind":       "PlacementDecision",
			"metadata": map[string]interface{}{
				"name": pldName,
				"labels": map[string]string{
					"generated-by-policy-test":                     "",
					"cluster.open-cluster-management.io/placement": placementName,
				},
			},
		},
	}

	err := ClientHubDynamic.Resource(GvrPlacementDecision).Namespace(namespace).Delete(
		ctx,
		pldName,
		metav1.DeleteOptions{},
	)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	decision, err := ClientHubDynamic.Resource(GvrPlacementDecision).Namespace(namespace).Create(
		ctx, &placementDecision, metav1.CreateOptions{},
	)
	if err != nil {
		return nil, err
	}

	return decision, nil
}

// ApplyPlacement function creates Placement and PlacementBinding so that it will
// always only match the targetCluster.
func ApplyPlacement(ctx SpecContext, namespace, policyName string) error {
	By("Apply Placement and PlacementBinding " + namespace + "/" +
		"placement-" + policyName + "/" + "placement-binding-" + policyName +
		" with clusterSelector {name: " + ClusterNamespaceOnHub + "}")

	placement := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": GvrPlacement.Group + "/" + GvrPlacement.Version,
			"kind":       "Placement",
			"metadata": map[string]interface{}{
				"name": "placement-" + policyName,
			},
			"spec": map[string]interface{}{
				"predicates": []interface{}{
					map[string]interface{}{
						"requiredClusterSelector": map[string]interface{}{
							"labelSelector": map[string]interface{}{
								"matchExpressions": []interface{}{
									map[string]interface{}{
										"key":      "name",
										"operator": "In",
										"values": []string{
											ClusterNamespaceOnHub,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := ClientHubDynamic.Resource(GvrPlacement).Namespace(namespace).Create(
		ctx, &placement, metav1.CreateOptions{},
	)
	if err != nil {
		return err
	}

	placementBinding := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": GvrPolicy.Group + "/" + GvrPolicy.Version,
			"kind":       "PlacementBinding",
			"metadata": map[string]interface{}{
				"name": "placement-binding-" + policyName,
			},
			"placementRef": map[string]interface{}{
				"name":     "placement-" + policyName,
				"kind":     "Placement",
				"apiGroup": GvrPlacement.Group,
			},
			"subjects": []interface{}{
				map[string]interface{}{
					"name":     policyName,
					"kind":     "Policy",
					"apiGroup": GvrPolicy.Group,
				},
			},
		},
	}

	_, err = ClientHubDynamic.Resource(GvrPlacementBinding).Namespace(namespace).Create(
		ctx, &placementBinding, metav1.CreateOptions{},
	)

	return err
}

// DeletePlacement delete applied Placement and PlacementBinding
func DeletePlacement(namespace, policyName string) error {
	By("Delete Placement and PlacementBinding " + namespace + "/" +
		"placement-" + policyName + "/" + "placement-binding-" + "policyName")

	_, err := OcHub(
		"delete", "placements.cluster.open-cluster-management.io", "placement-"+policyName,
		"-n", namespace, "--ignore-not-found",
	)
	if err != nil {
		return err
	}

	_, err = OcHub(
		"delete", "placementbindings.policy.open-cluster-management.io",
		"placement-binding-"+policyName, "-n", namespace, "--ignore-not-found",
	)

	return err
}

// DoCreatePolicyTest runs usual assertions around creating a policy. It will
// create the given policy file to the hub cluster, on the user namespace. It
// also patches the Placement with a PlacementDecision if required. It
// asserts that the policy was distributed to the managed cluster, and for any
// templateGVRs supplied, it asserts that a policy template of that type (for
// example ConfigurationPolicy) and the same name was created on the managed
// cluster.
//
// It assumes that the given filename (stripped of an extension) matches the
// name of the policy, and that the Placement has the same name, with '-plr'
// appended.
func DoCreatePolicyTest(ctx context.Context, policyFile string, templateGVRs ...schema.GroupVersionResource) {
	GinkgoHelper()

	policyName := strings.TrimSuffix(filepath.Base(policyFile), filepath.Ext(policyFile))

	By("DoCreatePolicyTest creates " + policyFile + " on namespace " + UserNamespace)
	//nolint:contextcheck
	output, err := OcHub("apply", "-f", policyFile, "-n", UserNamespace)
	Expect(err).ToNot(HaveOccurred())
	By("DoCreatePolicyTest OcHub apply output: " + output)

	plc := utils.GetWithTimeout(ClientHubDynamic, GvrPolicy, policyName, UserNamespace, true, DefaultTimeoutSeconds)
	Expect(plc).NotTo(BeNil())

	if ManuallyPatchDecisions {
		plrName := policyName + "-plr"
		By("Patching " + plrName + " with decision of cluster " + ClusterNamespaceOnHub)
		pld, err := CreatePlacementDecision(ctx, UserNamespace, plrName)
		Expect(err).ToNot(HaveOccurred())

		pld.Object["status"] = utils.GeneratePldStatus("", "", ClusterNamespaceOnHub)
		_, err = ClientHubDynamic.Resource(GvrPlacementDecision).Namespace(UserNamespace).UpdateStatus(
			ctx,
			pld,
			metav1.UpdateOptions{},
		)
		Expect(err).ToNot(HaveOccurred())
	}

	managedPolicyName := UserNamespace + "." + policyName
	By("Checking " + managedPolicyName + " on managed cluster in ns " + ClusterNamespace)
	Expect(utils.GetWithTimeout(
		ClientHostingDynamic, GvrPolicy, managedPolicyName, ClusterNamespace, true, DefaultTimeoutSeconds*2,
	)).NotTo(BeNil())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		By("Checking that the policy template " + typedName + " is present on the managed cluster")

		Expect(utils.GetWithTimeout(
			ClientHostingDynamic, tmplGVR, policyName, ClusterNamespace, true, DefaultTimeoutSeconds,
		)).NotTo(BeNil())
	}
}

// DoCleanupPolicy deletes the resources specified in the file, and asserts that
// the propagated policy was removed from the managed cluster. For each templateGVR,
// it will check that there is no longer a policy template (for example
// ConfigurationPolicy) of the same name on the managed cluster.
func DoCleanupPolicy(policyFile string, templateGVRs ...schema.GroupVersionResource) {
	GinkgoHelper()

	policyName := strings.TrimSuffix(filepath.Base(policyFile), filepath.Ext(policyFile))
	By("Deleting " + policyFile)
	_, err := OcHub(
		"delete", "-f", policyFile, "-n", UserNamespace,
		"--ignore-not-found",
	)
	Expect(err).ToNot(HaveOccurred())

	Expect(utils.GetWithTimeout(
		ClientHubDynamic, GvrPolicy, policyName, UserNamespace, false, DefaultTimeoutSeconds,
	)).To(BeNil())

	managedPolicyName := UserNamespace + "." + policyName
	By("Checking " + managedPolicyName + " was removed from managed cluster in ns " + ClusterNamespace)
	Expect(utils.GetWithTimeout(
		ClientManagedDynamic, GvrPolicy, managedPolicyName, ClusterNamespace, false, DefaultTimeoutSeconds,
	)).To(BeNil())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		By("Checking that the policy template " + typedName + " was removed from the managed cluster")
		Expect(utils.GetWithTimeout(
			ClientManagedDynamic, tmplGVR, policyName, ClusterNamespace, false, DefaultTimeoutSeconds,
		)).To(BeNil())
	}
}

// DoRootComplianceTest asserts that the given policy has the given compliance
// on the root policy on the hub cluster.
func DoRootComplianceTest(policyName string, compliance policiesv1.ComplianceState) {
	GinkgoHelper()

	By("Checking if the status of root policy " + policyName + " is " + string(compliance))
	EventuallyWithOffset(
		1,
		GetComplianceState(policyName),
		DefaultTimeoutSeconds,
		1,
	).Should(Equal(compliance))
}

func GetHistoryMessages(policyName string, templateIdx int) ([]interface{}, bool, error) {
	empty := make([]interface{}, 0)
	replicatedPolicyName := UserNamespace + "." + policyName
	policyInterface := ClientHostingDynamic.Resource(GvrPolicy).Namespace(ClusterNamespace)

	policy, err := policyInterface.Get(context.TODO(), replicatedPolicyName, metav1.GetOptions{})
	if err != nil {
		return empty, false, errors.New("error in getting policy")
	}

	details, found, err := unstructured.NestedSlice(policy.Object, "status", "details")
	if !found || err != nil || len(details) <= templateIdx {
		return empty, false, errors.New("error in getting status")
	}

	templateDetails, ok := details[templateIdx].(map[string]interface{})
	if !ok {
		return empty, false, errors.New("error in getting detail")
	}

	history, found, err := unstructured.NestedSlice(templateDetails, "history")

	return history, found, err
}

// GetOpPolicyCompMsg returns a function (so that it can be used in an Eventually)
// that returns the current Compliant condition message on the specified OperatorPolicy.
// It will return an empty string if the OperatorPolicy or condition could not be found.
func GetOpPolicyCompMsg(policyName string) func() string {
	return func() string {
		unstructOpPol := utils.GetWithTimeout(
			ClientManagedDynamic,
			GvrOperatorPolicy,
			policyName,
			ClusterNamespace,
			true,
			DefaultTimeoutSeconds,
		)
		Expect(unstructOpPol).NotTo(BeNil())

		condList, found, err := unstructured.NestedSlice(
			unstructOpPol.Object,
			"status",
			"conditions",
		)
		if err != nil || !found {
			return ""
		}

		for _, cond := range condList {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(condMap, "type")
			if condType == "Compliant" {
				return fmt.Sprintf("%v", condMap["message"])
			}
		}

		return ""
	}
}

// GetLatestStatusMessage returns the most recent status message for the given policy template.
// If the policy, template, or status do not exist for any reason, an empty string is returned.
func GetLatestStatusMessage(policyName string, templateIdx int) func() string {
	return func() string {
		history, found, err := GetHistoryMessages(policyName, templateIdx)
		if !found || err != nil || len(history) == 0 {
			return ""
		}

		topHistoryItem, ok := history[0].(map[string]interface{})
		if !ok {
			return ""
		}

		message, _, _ := unstructured.NestedString(topHistoryItem, "message")

		return message
	}
}

func GetDuplicateHistoryMessage(policyName string) string {
	history, _, err := GetHistoryMessages(policyName, 0)
	if err != nil {
		return ""
	}

	historyMsgs := []string{}

	for _, h := range history {
		historyItem, _ := h.(map[string]interface{})
		m, _, _ := unstructured.NestedString(historyItem, "message")
		historyMsgs = append(historyMsgs, m)
	}

	for i, m := range historyMsgs {
		if i > 0 {
			if m == historyMsgs[i-1] {
				return m
			}
		}
	}

	return ""
}

func DoHistoryUpdatedTest(policyName string, messages ...string) {
	By("Getting policy history")

	// There is a limit of 10 messages in the Policy status, so if more are passed in, just truncate it.
	if len(messages) > 10 {
		messages = messages[:10]
	}

	By("Getting policy history, check latest message")
	Eventually(func(g Gomega) {
		history, _, err := GetHistoryMessages(policyName, 0)
		g.Expect(err).ShouldNot(HaveOccurred())
		lenMessage := len(messages)
		historyMsgs := []string{}
		GinkgoWriter.Println("Returned policy history:")
		for i, h := range history {
			historyItem, _ := h.(map[string]interface{})
			m, _, _ := unstructured.NestedString(historyItem, "message")
			historyMsgs = append(historyMsgs, m)
			GinkgoWriter.Println(strconv.Itoa(i) + ": " + m)
		}
		By("Check history length is same")
		g.Expect(history).Should(HaveLen(lenMessage))

		By("Check history message same")
		g.Expect(strings.Join(historyMsgs, "")).Should(Equal(strings.Join(messages, "")))
	}, DefaultTimeoutSeconds, 1).Should(Succeed())
}

// InformPolicy patches the root policy to be informed and verifies that it propagates. Note: when
// checking a policy template, it assumes the template's name matches the root policy's name.
func InformPolicy(policyName string, templateGVRs ...schema.GroupVersionResource) {
	setRemediationAction(policyName, "inform", templateGVRs...)
}

// EnforcePolicy patches the root policy to be enforced and verifies that it propagates. Note: when
// checking a policy template, it assumes the template's name matches the root policy's name.
func EnforcePolicy(policyName string, templateGVRs ...schema.GroupVersionResource) {
	setRemediationAction(policyName, "enforce", templateGVRs...)
}

// SetRemediationAction patches the root policy, and asserts that the replicated policy on the
// managed cluster, and policy template objects (based on the provided GVRs) have remediationActions
// that match. Note: when checking a policy template, it assumes the template's name matches the
// root policy's name.
func setRemediationAction(
	policyName string, remediationAction string, templateGVRs ...schema.GroupVersionResource,
) {
	GinkgoHelper()

	ctx := context.TODO()
	rootPolicyClient := ClientHubDynamic.Resource(GvrPolicy).Namespace(UserNamespace)

	By("Patching remediationAction = " + remediationAction + " on root policy")
	Eventually(func(g Gomega) {
		rootPlc, err := rootPolicyClient.Get(ctx, policyName, metav1.GetOptions{})
		g.Expect(err).ToNot(HaveOccurred())

		err = unstructured.SetNestedField(rootPlc.Object, remediationAction, "spec", "remediationAction")
		g.Expect(err).ToNot(HaveOccurred())

		_, err = rootPolicyClient.Update(ctx, rootPlc, metav1.UpdateOptions{})
		g.Expect(err).ToNot(HaveOccurred())
	}, DefaultTimeoutSeconds, 1).Should(Succeed())

	managedPolicyClient := ClientHostingDynamic.Resource(GvrPolicy).Namespace(ClusterNamespace)

	By("Checking that remediationAction = " + remediationAction + " on replicated policy")
	Eventually(func(g Gomega) {
		managedPlc, err := managedPolicyClient.Get(ctx, UserNamespace+"."+policyName, metav1.GetOptions{})
		g.Expect(err).ToNot(HaveOccurred())

		action, found, err := unstructured.NestedString(managedPlc.Object, "spec", "remediationAction")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(action).To(Equal(remediationAction))
	}, DefaultTimeoutSeconds, 1).Should(Succeed())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		By("Checking that remediationAction = " + remediationAction + " on policy template " + typedName)

		templateClient := ClientHostingDynamic.Resource(tmplGVR).Namespace(ClusterNamespace)

		Eventually(func(g Gomega) {
			template, err := templateClient.Get(ctx, policyName, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())

			action, found, err := unstructured.NestedString(template.Object, "spec", "remediationAction")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(action).To(Equal(remediationAction))
		}, DefaultTimeoutSeconds, 1).Should(Succeed())
	}
}

// RegisterDebugMessage returns a pointer to a string which this function will register to be
// printed in the ginkgo logs only if the test fails.
func RegisterDebugMessage() *string {
	msg := new(string)

	DeferCleanup(func() {
		if CurrentSpecReport().Failed() {
			GinkgoWriter.Println(*msg)
		}
	})

	return msg
}
