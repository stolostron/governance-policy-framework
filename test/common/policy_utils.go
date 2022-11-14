// Copyright Contributors to the Open Cluster Management project

package common

import (
	"context"
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
	return GetClusterComplianceState(policyName, ClusterNamespace)
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
		g.ExpectWithOffset(1, err).To(BeNil())

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
			"value":{"matchExpressions":[{"key": "name", "operator": "In", "values": ["`+ClusterNamespace+`"]}]}
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
	ExpectWithOffset(1, err).To(BeNil())
	By("DoCreatePolicyTest OcHub apply output: " + output)

	plc := utils.GetWithTimeout(ClientHubDynamic, GvrPolicy, policyName, UserNamespace, true, DefaultTimeoutSeconds)
	ExpectWithOffset(1, plc).NotTo(BeNil())

	if ManuallyPatchDecisions {
		plrName := policyName + "-plr"
		By("Patching " + plrName + " with decision of cluster " + ClusterNamespace)
		plr := utils.GetWithTimeout(
			ClientHubDynamic, GvrPlacementRule, plrName, UserNamespace, true, DefaultTimeoutSeconds,
		)
		plr.Object["status"] = utils.GeneratePlrStatus(ClusterNamespace)
		_, err := ClientHubDynamic.Resource(GvrPlacementRule).Namespace(UserNamespace).UpdateStatus(
			context.TODO(),
			plr,
			metav1.UpdateOptions{},
		)
		ExpectWithOffset(1, err).To(BeNil())
	}

	managedPolicyName := UserNamespace + "." + policyName
	By("Checking " + managedPolicyName + " on managed cluster in ns " + ClusterNamespace)
	mplc := utils.GetWithTimeout(
		ClientManagedDynamic, GvrPolicy, managedPolicyName, ClusterNamespace, true, DefaultTimeoutSeconds,
	)
	ExpectWithOffset(1, mplc).NotTo(BeNil())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		By("Checking that the policy template " + typedName + " is present on the managed cluster")

		tmplPlc := utils.GetWithTimeout(
			ClientManagedDynamic, tmplGVR, policyName, ClusterNamespace, true, DefaultTimeoutSeconds,
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
	Expect(err).To(BeNil())

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

// GetLatestStatusMessage returns the most recent status message for the given policy template.
// If the policy, template, or status do not exist for any reason, an empty string is returned.
func GetLatestStatusMessage(policyName string, templateIdx int) func() string {
	return func() string {
		replicatedPolicyName := UserNamespace + "." + policyName
		policyInterface := ClientManagedDynamic.Resource(GvrPolicy).Namespace(ClusterNamespace)

		policy, err := policyInterface.Get(context.TODO(), replicatedPolicyName, metav1.GetOptions{})
		if err != nil {
			return ""
		}

		details, found, err := unstructured.NestedSlice(policy.Object, "status", "details")
		if !found || err != nil || len(details) <= templateIdx {
			return ""
		}

		templateDetails, ok := details[templateIdx].(map[string]interface{})
		if !ok {
			return ""
		}

		history, found, err := unstructured.NestedSlice(templateDetails, "history")
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

// EnforcePolicy patches the root policy to be enforced, and asserts that the
// replicated policy on the managed cluster, and policy template objects (based
// on the provided GVRs) are enforced. Note: when checking a policy template, it
// assumes the template's name matches the root policy's name.
func EnforcePolicy(policyName string, templateGVRs ...schema.GroupVersionResource) {
	ctx := context.TODO()
	rootPolicyClient := ClientHubDynamic.Resource(GvrPolicy).Namespace(UserNamespace)

	By("Patching remediationAction = enforce on root policy")
	EventuallyWithOffset(1, func(g Gomega) {
		rootPlc, err := rootPolicyClient.Get(ctx, policyName, metav1.GetOptions{})
		g.ExpectWithOffset(1, err).To(BeNil())

		err = unstructured.SetNestedField(rootPlc.Object, "enforce", "spec", "remediationAction")
		g.ExpectWithOffset(1, err).To(BeNil())

		_, err = rootPolicyClient.Update(ctx, rootPlc, metav1.UpdateOptions{})
		g.ExpectWithOffset(1, err).To(BeNil())
	}, DefaultTimeoutSeconds, 1).Should(Succeed())

	managedPolicyClient := ClientManagedDynamic.Resource(GvrPolicy).Namespace(ClusterNamespace)

	By("Checking that remediationAction = enforce on replicated policy")
	EventuallyWithOffset(1, func(g Gomega) {
		managedPlc, err := managedPolicyClient.Get(ctx, UserNamespace+"."+policyName, metav1.GetOptions{})
		g.ExpectWithOffset(1, err).To(BeNil())

		action, found, err := unstructured.NestedString(managedPlc.Object, "spec", "remediationAction")
		g.ExpectWithOffset(1, err).To(BeNil())
		g.ExpectWithOffset(1, found).To(BeTrue())
		g.ExpectWithOffset(1, action).To(Equal("enforce"))
	}, DefaultTimeoutSeconds, 1).Should(Succeed())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		By("Checking that remediationAction = enforce on policy template " + typedName)

		templateClient := ClientManagedDynamic.Resource(tmplGVR).Namespace(ClusterNamespace)

		EventuallyWithOffset(1, func(g Gomega) {
			template, err := templateClient.Get(ctx, policyName, metav1.GetOptions{})
			g.ExpectWithOffset(1, err).To(BeNil())

			action, found, err := unstructured.NestedString(template.Object, "spec", "remediationAction")
			g.ExpectWithOffset(1, err).To(BeNil())
			g.ExpectWithOffset(1, found).To(BeTrue())
			g.ExpectWithOffset(1, action).To(Equal("enforce"))
		}, DefaultTimeoutSeconds, 1).Should(Succeed())
	}
}
