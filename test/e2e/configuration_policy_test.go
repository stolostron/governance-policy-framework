// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"

	"k8s.io/klog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

func configPolicyTestCleanUp(roleName, rolePolicyName, rolePolicyYAML string) {
	By("Deleting the role, policy, and events on managed cluster")
	common.DoCleanupPolicy(rolePolicyYAML, common.GvrConfigurationPolicy)
	_, err := common.OcManaged(
		"delete", "events", "-n", "managed",
		"--field-selector=involvedObject.name="+common.UserNamespace+"."+rolePolicyName,
		"--ignore-not-found",
	)
	ExpectWithOffset(1, err).To(BeNil())
	_, err = common.OcHub(
		"delete", "events", "-n", "managed",
		"--field-selector=involvedObject.name="+common.UserNamespace+"."+rolePolicyName,
		"--ignore-not-found",
	)
	ExpectWithOffset(1, err).To(BeNil())
	_, err = common.OcManaged(
		"delete", "events", "-n", "managed",
		"--field-selector=involvedObject.name="+rolePolicyName,
		"--ignore-not-found",
	)
	ExpectWithOffset(1, err).To(BeNil())
	_, err = common.OcManaged(
		"delete", "role", "-n", "default", roleName,
		"--ignore-not-found",
	)
	ExpectWithOffset(1, err).To(BeNil())
}

var _ = Describe("Test configuration policy inform", Ordered, func() {
	const roleName string = "role-policy-e2e"

	Describe("Test object musthave inform", Ordered, func() {
		const rolePolicyName string = "role-policy-musthave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-musthave.yaml"
		expectedStatusMsgs := []string{}
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(rolePolicyYaml, common.GvrConfigurationPolicy)
		})
		It("the policy should be noncompliant", func() {
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{"NonCompliant; violation - roles not found: [role-policy-e2e] in namespace default missing"},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant after manually creating the role that matches", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default found as specified, " +
						"therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be noncompliant after removing the role", func() {
			By("Deleting the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"delete", "-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n", "default", "--ignore-not-found",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{"NonCompliant; violation - roles not found: [role-policy-e2e] in namespace default missing"},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant after manually creating a role that more", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e-more.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default found as specified, " +
						"therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be noncompliant after manually creating a role that has less rule", func() {
			By("Creating the mismatch role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e-less.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{
					"NonCompliant; violation - roles not found: [role-policy-e2e] in namespace " +
						"default found but not as specified",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant after manually creating the role that matches", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default found " +
						"as specified, therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be noncompliant after removing the role", func() {
			By("Deleting the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"delete", "-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n", "default", "--ignore-not-found",
			)
			Expect(err).To(BeNil())

			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{"NonCompliant; violation - roles not found: [role-policy-e2e] in namespace default missing"},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		AfterAll(func() {
			configPolicyTestCleanUp(roleName, rolePolicyName, rolePolicyYaml)
		})
	})

	Describe("Test object mustnothave inform", Ordered, func() {
		const rolePolicyName string = "role-policy-mustnothave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-mustnothave.yaml"
		expectedStatusMsgs := []string{}
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(rolePolicyYaml, common.GvrConfigurationPolicy)
		})
		It("the policy should be compliant", func() {
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default missing " +
						"as expected, therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be noncompliant after manually creating the role on managed cluster", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{"NonCompliant; violation - roles found: [role-policy-e2e] in namespace default"},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant after removing the role", func() {
			By("Deleting the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"delete", "role", "-n", "default", roleName,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default missing " +
						"as expected, therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		AfterAll(func() {
			configPolicyTestCleanUp(roleName, rolePolicyName, rolePolicyYaml)
		})
	})

	Describe("Test object mustonlyhave inform", Ordered, func() {
		const rolePolicyName string = "role-policy-mustonlyhave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-mustonlyhave.yaml"
		expectedStatusMsgs := []string{}
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(rolePolicyYaml, common.GvrConfigurationPolicy)
		})
		It("the policy should be noncompliant", func() {
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{"NonCompliant; violation - roles not found: [role-policy-e2e] in namespace default missing"},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant if manually created", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply",
				"-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n",
				"default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the role should be noncompliant if mismatch", func() {
			By("Creating a role with different rules")
			_, err := common.OcManaged(
				"apply",
				"-f",
				"../resources/configuration_policy/role-policy-e2e-mismatch.yaml",
				"-n",
				"default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{
					"NonCompliant; violation - roles not found: [role-policy-e2e] in " +
						"namespace default found but not as specified",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant if matches", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply",
				"-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n",
				"default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be noncompliant if has less rules", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply",
				"-f",
				"../resources/configuration_policy/role-policy-e2e-less.yaml",
				"-n",
				"default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{
					"NonCompliant; violation - roles not found: [role-policy-e2e] in " +
						"namespace default found but not as specified",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant if matches", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply",
				"-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n",
				"default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be noncompliant if has more rules", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply",
				"-f",
				"../resources/configuration_policy/role-policy-e2e-more.yaml",
				"-n",
				"default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{
					"NonCompliant; violation - roles not found: [role-policy-e2e] in " +
						"namespace default found but not as specified",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		AfterAll(func() {
			configPolicyTestCleanUp(roleName, rolePolicyName, rolePolicyYaml)
		})
	})
})

var _ = Describe("Test configuration policy enforce", Ordered, func() {
	const roleName string = "role-policy-e2e"

	Describe("Test object musthave enforce", Ordered, func() {
		const rolePolicyName string = "role-policy-musthave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-musthave.yaml"
		expectedStatusMsgs := []string{}
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(rolePolicyYaml, common.GvrConfigurationPolicy)
		})
		It("the policy should be noncompliant", func() {
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{
					"NonCompliant; violation - roles not found: [role-policy-e2e] in " +
						"namespace default missing",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant after enforcing it", func() {
			common.EnforcePolicy(rolePolicyName, common.GvrConfigurationPolicy)
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"was missing, and was created successfully",
					"NonCompliant; violation - No instances of `roles` found as specified " +
						"in namespaces: default",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("should recreate the role if manually deleted", func() {
			By("Deleting the role in default namespace on managed cluster")
			klog.Info("deleting role")
			del, err := common.OcManaged(
				"delete", "role", "-n", "default", roleName,
				"--ignore-not-found",
			)
			klog.Info(del)
			Expect(err).To(BeNil())

			By("Checking if the role has been recreated")
			klog.Info("checking for role")
			Eventually(func() interface{} {
				role, _ := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").Get(
					context.TODO(),
					roleName,
					metav1.GetOptions{},
				)
				if role != nil {
					klog.Info(role)
				}

				return role
			}, defaultTimeoutSeconds, 1).ShouldNot(BeNil())

			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)

			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"was missing, and was created successfully",
					"NonCompliant; violation - No instances of `roles` found as specified " +
						"in namespaces: default",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should not be patched after manually creating a role that has more rules", func() {
			By("Creating the mismatch role in default namespace on managed cluster")
			mismatch, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e-more.yaml",
				"-n", "default",
			)
			klog.Info("patch more")
			klog.Info(mismatch)
			Expect(err).To(BeNil())
			By("Checking if the role is not patched to match in 30s")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e-more.yaml")
			Consistently(func() interface{} {
				managedRole := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrRole,
					roleName,
					"default",
					true,
					defaultTimeoutSeconds,
				)
				klog.Info(managedRole)

				return managedRole.Object["rules"]
			}, 30, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))

			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be patched after manually creating a role that has less rules", func() {
			By("Creating the mismatch role in default namespace on managed cluster")
			mismatch, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e-less.yaml",
				"-n", "default",
			)
			klog.Info("patch less")
			klog.Info(mismatch)
			Expect(err).To(BeNil())
			By("Checking if the role has been patched to match")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrRole,
					roleName,
					"default",
					true,
					defaultTimeoutSeconds,
				)
				klog.Info(managedRole)

				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))

			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default was updated successfully",
					"NonCompliant; violation - No instances of `roles` found as specified in namespaces: default",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		AfterAll(func() {
			configPolicyTestCleanUp(roleName, rolePolicyName, rolePolicyYaml)
		})
	})

	Describe("Test object mustnothave enforce", Ordered, func() {
		const rolePolicyName string = "role-policy-mustnothave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-mustnothave.yaml"
		expectedStatusMsgs := []string{}
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(rolePolicyYaml, common.GvrConfigurationPolicy)
		})
		It("the policy should be compliant", func() {
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default missing " +
						"as expected, therefore this Object template is compliant",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be noncompliant after manually creating the role on managed cluster", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(rolePolicyName, policiesv1.NonCompliant)
			expectedStatusMsgs = append(
				[]string{"NonCompliant; violation - roles found: [role-policy-e2e] in namespace default"},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant after enforcing it", func() {
			common.EnforcePolicy(rolePolicyName, common.GvrConfigurationPolicy)
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default missing " +
						"as expected, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default existed, " +
						"and was deleted successfully",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should remove the role on managed cluster if manually created", func() {
			By("Creating the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			By("Checking if the role has been deleted")
			Eventually(func() interface{} {
				role, _ := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").Get(
					context.TODO(),
					roleName,
					metav1.GetOptions{},
				)

				return role
			}, defaultTimeoutSeconds, 1).Should(BeNil())

			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default missing " +
						"as expected, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default existed, " +
						"and was deleted successfully",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		AfterAll(func() {
			configPolicyTestCleanUp(roleName, rolePolicyName, rolePolicyYaml)
		})
	})

	Describe("Test object mustonlyhave enforce", Ordered, func() {
		const rolePolicyName string = "role-policy-mustonlyhave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-mustonlyhave.yaml"
		expectedStatusMsgs := []string{}
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(rolePolicyYaml, common.GvrConfigurationPolicy)
			expectedStatusMsgs = append(
				[]string{"NonCompliant; violation - roles not found: [role-policy-e2e] in namespace default missing"},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the policy should be compliant after enforcing it", func() {
			common.EnforcePolicy(rolePolicyName, common.GvrConfigurationPolicy)
			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in " +
						"namespace default was missing, and was created successfully",
					"NonCompliant; violation - No instances of `roles` found as specified in namespaces: default",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the role should be created by policy", func() {
			By("Checking if the role has been created")
			Eventually(func() interface{} {
				role, _ := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").Get(
					context.TODO(),
					roleName,
					metav1.GetOptions{},
				)

				return role
			}, defaultTimeoutSeconds, 1).ShouldNot(BeNil())
		})
		It("the role should be recreated if manually deleted", func() {
			By("Deleting the role in default namespace on managed cluster")
			_, err := common.OcManaged(
				"delete", "role", "-n", "default", roleName,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())

			By("Checking if the role has been recreated")
			Eventually(func() interface{} {
				role, _ := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").Get(
					context.TODO(),
					roleName,
					metav1.GetOptions{},
				)

				return role
			}, defaultTimeoutSeconds, 1).ShouldNot(BeNil())

			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"was missing, and was created successfully",
					"NonCompliant; violation - No instances of `roles` found as specified in namespaces: default",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the role should be patched if has less rules", func() {
			By("Creating a role with less rules")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/configuration_policy/role-policy-e2e-less.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			By("Checking if the role has been patched to match by config policy")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrRole, roleName,
					"default",
					true,
					defaultTimeoutSeconds,
				)

				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))

			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default was updated successfully",
					"NonCompliant; violation - No instances of `roles` found as specified in namespaces: default",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the role should be patched if has more rules", func() {
			By("Creating a role with more rules")
			_, err := common.OcManaged(
				"apply",
				"-f",
				"../resources/configuration_policy/role-policy-e2e-more.yaml",
				"-n",
				"default",
			)
			Expect(err).To(BeNil())
			By("Checking if the role has been patched to match by config policy")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrRole,
					roleName,
					"default",
					true,
					defaultTimeoutSeconds,
				)

				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))

			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default was updated successfully",
					"NonCompliant; violation - No instances of `roles` " +
						"found as specified in namespaces: default",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		It("the role should be patched if mismatch", func() {
			By("Creating a role with different rules")
			_, err := common.OcManaged(
				"apply",
				"-f",
				"../resources/configuration_policy/role-policy-e2e-mismatch.yaml", "-n", "default",
			)
			Expect(err).To(BeNil())
			By("Checking if the role has been patched to match by config policy")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrRole,
					roleName,
					"default",
					true,
					defaultTimeoutSeconds,
				)

				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))

			common.DoRootComplianceTest(rolePolicyName, policiesv1.Compliant)
			expectedStatusMsgs = append(
				[]string{
					"Compliant; notification - roles [role-policy-e2e] in namespace default " +
						"found as specified, therefore this Object template is compliant",
					"Compliant; notification - roles [role-policy-e2e] in namespace default was updated successfully",
					"NonCompliant; violation - No instances of `roles` " +
						"found as specified in namespaces: default",
				},
				expectedStatusMsgs...,
			)
			common.DoHistoryUpdatedTest(rolePolicyName, expectedStatusMsgs...)
		})
		AfterAll(func() {
			configPolicyTestCleanUp(roleName, rolePolicyName, rolePolicyYaml)
		})
	})
})
