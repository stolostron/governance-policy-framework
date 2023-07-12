// Copyright Contributors to the Open Cluster Management project

package common

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		g.ExpectWithOffset(1, err).ToNot(HaveOccurred())

		for _, statusPerCluster := range policy.Status.Status {
			if statusPerCluster.ClusterNamespace == clusterName {
				return statusPerCluster.ComplianceState
			}
		}

		return nil
	}
}

// Patches the clusterSelector of the specified PlacementRule so that it will
// always only match the targetCluster.
func PatchPlacementRule(namespace, name string) error {
	_, err := OcHub(
		"patch",
		"-n",
		namespace,
		"placementrule.apps.open-cluster-management.io",
		name,
		"--type=json",
		`-p=[{
			"op": "replace",
			"path": "/spec/clusterSelector",
			"value":{"matchExpressions":[{"key": "name", "operator": "In", "values": ["`+ClusterNamespaceOnHub+`"]}]}
		}]`,
	)

	return err
}

// DoCreatePolicyTest runs usual assertions around creating a policy. It will
// create the given policy file to the hub cluster, on the user namespace. It
// also patches the PlacementRule with a PlacementDecision if required. It
// asserts that the policy was distributed to the managed cluster, and for any
// templateGVRs supplied, it asserts that a policy template of that type (for
// example ConfigurationPolicy) and the same name was created on the managed
// cluster.
//
// It assumes that the given filename (stripped of an extension) matches the
// name of the policy, and that the PlacementRule has the same name, with '-plr'
// appended.
func DoCreatePolicyTest(policyFile string, templateGVRs ...schema.GroupVersionResource) {
	policyName := strings.TrimSuffix(filepath.Base(policyFile), filepath.Ext(policyFile))

	By("DoCreatePolicyTest creates " + policyFile + " on namespace " + UserNamespace)
	output, err := OcHub("apply", "-f", policyFile, "-n", UserNamespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	By("DoCreatePolicyTest OcHub apply output: " + output)

	plc := utils.GetWithTimeout(ClientHubDynamic, GvrPolicy, policyName, UserNamespace, true, DefaultTimeoutSeconds)
	ExpectWithOffset(1, plc).NotTo(BeNil())

	if ManuallyPatchDecisions {
		plrName := policyName + "-plr"
		By("Patching " + plrName + " with decision of cluster " + UserNamespace)
		plr := utils.GetWithTimeout(
			ClientHubDynamic, GvrPlacementRule, plrName, UserNamespace, true, DefaultTimeoutSeconds,
		)
		plr.Object["status"] = utils.GeneratePlrStatus(ClusterNamespaceOnHub)
		_, err := ClientHubDynamic.Resource(GvrPlacementRule).Namespace(UserNamespace).UpdateStatus(
			context.TODO(),
			plr,
			metav1.UpdateOptions{},
		)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
	}

	managedPolicyName := UserNamespace + "." + policyName
	By("Checking " + managedPolicyName + " on managed cluster in ns " + ClusterNamespace)
	mplc := utils.GetWithTimeout(
		ClientHostingDynamic, GvrPolicy, managedPolicyName, ClusterNamespace, true, DefaultTimeoutSeconds*2,
	)
	ExpectWithOffset(1, mplc).NotTo(BeNil())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		By("Checking that the policy template " + typedName + " is present on the managed cluster")

		tmplPlc := utils.GetWithTimeout(
			ClientHostingDynamic, tmplGVR, policyName, ClusterNamespace, true, DefaultTimeoutSeconds,
		)
		ExpectWithOffset(1, tmplPlc).NotTo(BeNil())
	}
}

// DoCleanupPolicy deletes the resources specified in the file, and asserts that
// the propagated policy was removed from the managed cluster. For each templateGVR,
// it will check that there is no longer a policy template (for example
// ConfigurationPolicy) of the same name on the managed cluster.
func DoCleanupPolicy(policyFile string, templateGVRs ...schema.GroupVersionResource) {
	policyName := strings.TrimSuffix(filepath.Base(policyFile), filepath.Ext(policyFile))
	By("Deleting " + policyFile)
	_, err := OcHub(
		"delete", "-f", policyFile, "-n", UserNamespace,
		"--ignore-not-found",
	)
	Expect(err).ToNot(HaveOccurred())

	plc := utils.GetWithTimeout(ClientHubDynamic, GvrPolicy, policyName, UserNamespace, false, DefaultTimeoutSeconds)
	ExpectWithOffset(1, plc).To(BeNil())

	managedPolicyName := UserNamespace + "." + policyName
	By("Checking " + managedPolicyName + " was removed from managed cluster in ns " + ClusterNamespace)
	mplc := utils.GetWithTimeout(
		ClientManagedDynamic, GvrPolicy, managedPolicyName, ClusterNamespace, false, DefaultTimeoutSeconds,
	)
	ExpectWithOffset(1, mplc).To(BeNil())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		By("Checking that the policy template " + typedName + " was removed from the managed cluster")

		tmplPlc := utils.GetWithTimeout(
			ClientManagedDynamic, tmplGVR, policyName, ClusterNamespace, false, DefaultTimeoutSeconds,
		)
		ExpectWithOffset(1, tmplPlc).To(BeNil())
	}
}

// DoRootComplianceTest asserts that the given policy has the given compliance
// on the root policy on the hub cluster.
func DoRootComplianceTest(policyName string, compliance policiesv1.ComplianceState) {
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
		fmt.Println("Returned policy history:")
		for i, h := range history {
			historyItem, _ := h.(map[string]interface{})
			m, _, _ := unstructured.NestedString(historyItem, "message")
			historyMsgs = append(historyMsgs, m)
			fmt.Println(fmt.Sprint(i) + ": " + m)
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
	ctx := context.TODO()
	rootPolicyClient := ClientHubDynamic.Resource(GvrPolicy).Namespace(UserNamespace)

	By("Patching remediationAction = " + remediationAction + " on root policy")
	EventuallyWithOffset(1, func(g Gomega) {
		rootPlc, err := rootPolicyClient.Get(ctx, policyName, metav1.GetOptions{})
		g.ExpectWithOffset(1, err).ToNot(HaveOccurred())

		err = unstructured.SetNestedField(rootPlc.Object, remediationAction, "spec", "remediationAction")
		g.ExpectWithOffset(1, err).ToNot(HaveOccurred())

		_, err = rootPolicyClient.Update(ctx, rootPlc, metav1.UpdateOptions{})
		g.ExpectWithOffset(1, err).ToNot(HaveOccurred())
	}, DefaultTimeoutSeconds, 1).Should(Succeed())

	managedPolicyClient := ClientHostingDynamic.Resource(GvrPolicy).Namespace(ClusterNamespace)

	By("Checking that remediationAction = " + remediationAction + " on replicated policy")
	EventuallyWithOffset(1, func(g Gomega) {
		managedPlc, err := managedPolicyClient.Get(ctx, UserNamespace+"."+policyName, metav1.GetOptions{})
		g.ExpectWithOffset(1, err).ToNot(HaveOccurred())

		action, found, err := unstructured.NestedString(managedPlc.Object, "spec", "remediationAction")
		g.ExpectWithOffset(1, err).ToNot(HaveOccurred())
		g.ExpectWithOffset(1, found).To(BeTrue())
		g.ExpectWithOffset(1, action).To(Equal(remediationAction))
	}, DefaultTimeoutSeconds, 1).Should(Succeed())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		By("Checking that remediationAction = " + remediationAction + " on policy template " + typedName)

		templateClient := ClientHostingDynamic.Resource(tmplGVR).Namespace(ClusterNamespace)

		EventuallyWithOffset(1, func(g Gomega) {
			template, err := templateClient.Get(ctx, policyName, metav1.GetOptions{})
			g.ExpectWithOffset(1, err).ToNot(HaveOccurred())

			action, found, err := unstructured.NestedString(template.Object, "spec", "remediationAction")
			g.ExpectWithOffset(1, err).ToNot(HaveOccurred())
			g.ExpectWithOffset(1, found).To(BeTrue())
			g.ExpectWithOffset(1, action).To(Equal(remediationAction))
		}, DefaultTimeoutSeconds, 1).Should(Succeed())
	}
}
