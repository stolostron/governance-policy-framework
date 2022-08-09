// Copyright Contributors to the Open Cluster Management project

package common

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"
)

var (
	KubeconfigHub          string
	KubeconfigManaged      string
	UserNamespace          string
	ClusterNamespace       string
	DefaultTimeoutSeconds  int
	ManuallyPatchDecisions bool
	K8sClient              string
)

const MaxTravisTimeoutSeconds = 590 // Travis times out (by default) at 10 minutes

func init() {
	flag.StringVar(&KubeconfigHub, "kubeconfig_hub", "../../kubeconfig_hub", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
	flag.StringVar(&KubeconfigManaged, "kubeconfig_managed", "../../kubeconfig_managed", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
	flag.StringVar(&UserNamespace, "user_namespace", "policy-test", "ns on hub to create root policy")
	flag.StringVar(&ClusterNamespace, "cluster_namespace", "local-cluster", "cluster ns name")
	flag.IntVar(&DefaultTimeoutSeconds, "timeout_seconds", 30, "Timeout seconds for assertion")
	flag.BoolVar(&ManuallyPatchDecisions, "patch_decisions", true, "Whether to 'manually' patch PlacementRules with PlacementDecisions (set to false if the PlacementRule controller is running)")
	flag.StringVar(&K8sClient, "k8s_client", "oc", "Which k8s client to use for some tests - `oc`, `kubectl`, or something else entirely")
}

func NewKubeClient(url, kubeconfig, context string) kubernetes.Interface {
	klog.V(5).Infof("Create kubeclient for url %s using kubeconfig path %s\n", url, kubeconfig)
	config, err := LoadConfig(url, kubeconfig, context)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

func NewKubeClientDynamic(url, kubeconfig, context string) dynamic.Interface {
	klog.V(5).Infof("Create kubeclient dynamic for url %s using kubeconfig path %s\n", url, kubeconfig)
	config, err := LoadConfig(url, kubeconfig, context)
	if err != nil {
		panic(err)
	}

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

func LoadConfig(url, kubeconfig, context string) (*rest.Config, error) {
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	klog.V(5).Infof("Kubeconfig path %s\n", kubeconfig)
	// If we have an explicit indication of where the kubernetes config lives, read that.
	if kubeconfig != "" {
		if context == "" {
			return clientcmd.BuildConfigFromFlags(url, kubeconfig)
		}
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{
				CurrentContext: context,
			}).ClientConfig()
	}
	// If not, try the in-cluster config.
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory.
	if usr, err := user.Current(); err == nil {
		klog.V(5).Infof("clientcmd.BuildConfigFromFlags for url %s using %s\n", url, filepath.Join(usr.HomeDir, ".kube", "config"))
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not create a valid kubeconfig")
}

// GetComplianceState returns a function that requires no arguments that retrieves the
// compliance state of the input policy.
func GetComplianceState(clientHubDynamic dynamic.Interface, namespace, policyName, clusterNamespace string) func() interface{} {
	return func() interface{} {
		rootPlc := utils.GetWithTimeout(clientHubDynamic, GvrPolicy, policyName, namespace, true, DefaultTimeoutSeconds)
		var policy policiesv1.Policy
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(rootPlc.UnstructuredContent(), &policy)
		gomega.ExpectWithOffset(1, err).To(gomega.BeNil())
		for _, statusPerCluster := range policy.Status.Status {
			if statusPerCluster.ClusterNamespace == clusterNamespace {
				return statusPerCluster.ComplianceState
			}
		}

		return nil
	}
}

func oc(args ...string) (string, error) {
	// Determine whether output should be logged
	printOutput := true
	for _, a := range args {
		if a == "whoami" || strings.HasPrefix(a, "secret") {
			printOutput = false
			break
		}
	}
	output, err := exec.Command(K8sClient, args...).Output()
	if len(args) > 0 && printOutput {
		fmt.Println(string(output))
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.Stderr == nil {
			return string(output), nil
		}
		return string(output), fmt.Errorf(string(exitError.Stderr))
	}
	return string(output), err
}

// Runs the given oc/kubectl command against the configured hub cluster.
// Prints and returns the stdout from the command.
// If the command fails (non-zero exit code) and stderr was populated, that
// content will be returned in the error.
func OcHub(args ...string) (string, error) {
	args = append([]string{"--kubeconfig=" + KubeconfigHub}, args...)
	return oc(args...)
}

// Runs the given oc/kubectl command against the configured managed cluster.
// Prints and returns the stdout from the command.
// If the command fails (non-zero exit code) and stderr was populated, that
// content will be returned in the error.
func OcManaged(args ...string) (string, error) {
	args = append([]string{"--kubeconfig=" + KubeconfigManaged}, args...)
	return oc(args...)
}

// Patches the clusterSelector of the specified PlacementRule so that it will
// always only match the targetCluster.
func PatchPlacementRule(namespace, name, targetCluster, kubeconfigHub string) error {
	_, err := utils.KubectlWithOutput(
		"patch",
		"-n",
		namespace,
		"placementrule.apps.open-cluster-management.io",
		name,
		"--type=json",
		`-p=[{"op": "replace", "path": "/spec/clusterSelector", "value":{"matchExpressions":[{"key": "name", "operator": "In", "values": ["`+targetCluster+`"]}]}}]`,
		"--kubeconfig="+kubeconfigHub,
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
func DoCreatePolicyTest(hub, managed dynamic.Interface, policyFile string, templateGVRs ...schema.GroupVersionResource) {
	policyName := strings.TrimSuffix(filepath.Base(policyFile), filepath.Ext(policyFile))

	ginkgo.By("Creating " + policyFile)
	OcHub("apply", "-f", policyFile, "-n", UserNamespace)
	plc := utils.GetWithTimeout(hub, GvrPolicy, policyName, UserNamespace, true, DefaultTimeoutSeconds)
	gomega.ExpectWithOffset(1, plc).NotTo(gomega.BeNil())

	if ManuallyPatchDecisions {
		plrName := policyName + "-plr"
		ginkgo.By("Patching " + plrName + " with decision of cluster " + ClusterNamespace)
		plr := utils.GetWithTimeout(hub, GvrPlacementRule, plrName, UserNamespace, true, DefaultTimeoutSeconds)
		plr.Object["status"] = utils.GeneratePlrStatus(ClusterNamespace)
		_, err := hub.Resource(GvrPlacementRule).Namespace(UserNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
		gomega.ExpectWithOffset(1, err).To(gomega.BeNil())
	}

	managedPolicyName := UserNamespace + "." + policyName
	ginkgo.By("Checking " + managedPolicyName + " on managed cluster in ns " + ClusterNamespace)
	mplc := utils.GetWithTimeout(managed, GvrPolicy, managedPolicyName, ClusterNamespace, true, DefaultTimeoutSeconds)
	gomega.ExpectWithOffset(1, mplc).NotTo(gomega.BeNil())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		ginkgo.By("Checking that the policy template " + typedName + " is present on the managed cluster")
		tmplPlc := utils.GetWithTimeout(managed, tmplGVR, policyName, ClusterNamespace, true, DefaultTimeoutSeconds)
		gomega.ExpectWithOffset(1, tmplPlc).NotTo(gomega.BeNil())
	}
}

// DoCleanupPolicy deletes the resources specified in the file, and asserts that
// the propagated policy was removed from the managed cluster. For each templateGVR,
// it will check that there is no longer a policy template (for example
// ConfigurationPolicy) of the same name on the managed cluster.
func DoCleanupPolicy(hub, managed dynamic.Interface, policyFile string, templateGVRs ...schema.GroupVersionResource) {
	policyName := strings.TrimSuffix(filepath.Base(policyFile), filepath.Ext(policyFile))
	ginkgo.By("Deleting " + policyFile)
	OcHub("delete", "-f", policyFile, "-n", UserNamespace)
	plc := utils.GetWithTimeout(hub, GvrPolicy, policyName, UserNamespace, false, DefaultTimeoutSeconds)
	gomega.ExpectWithOffset(1, plc).To(gomega.BeNil())

	managedPolicyName := UserNamespace + "." + policyName
	ginkgo.By("Checking " + managedPolicyName + " was removed from managed cluster in ns " + ClusterNamespace)
	mplc := utils.GetWithTimeout(managed, GvrPolicy, managedPolicyName, ClusterNamespace, false, DefaultTimeoutSeconds)
	gomega.ExpectWithOffset(1, mplc).To(gomega.BeNil())

	for _, tmplGVR := range templateGVRs {
		typedName := tmplGVR.String() + "/" + policyName
		ginkgo.By("Checking that the policy template " + typedName + " was removed from the managed cluster")
		tmplPlc := utils.GetWithTimeout(managed, tmplGVR, policyName, ClusterNamespace, false, DefaultTimeoutSeconds)
		gomega.ExpectWithOffset(1, tmplPlc).To(gomega.BeNil())
	}
}

// DoRootComplianceTest asserts that the given policy has the given compliance
// on the root policy on the hub cluster.
func DoRootComplianceTest(hub dynamic.Interface, policyName string, compliance policiesv1.ComplianceState) {
	ginkgo.By("Checking if the status of root policy " + policyName + " is " + string(compliance))
	gomega.EventuallyWithOffset(
		1,
		GetComplianceState(hub, UserNamespace, policyName, ClusterNamespace),
		DefaultTimeoutSeconds,
		1,
	).Should(gomega.Equal(compliance))
}

func OutputDebugInfo(testName string, additionalResources ...string) {
	ginkgo.GinkgoWriter.Printf("%s test Kubernetes info:\n", testName)

	resources := []string{
		"policies.policy.open-cluster-management.io",
		"placementrules.apps.open-cluster-management.io",
		"placements.cluster.open-cluster-management.io",
		"placementbindings.policy.open-cluster-management.io",
	}
	resources = append(resources, additionalResources...)

	for _, resource := range resources {
		_, _ = utils.KubectlWithOutput("get", resource, "--all-namespaces", "-o", "yaml")
	}
}

// GetLatestStatusMessage returns the most recent status message for the given policy template.
// If the policy, template, or status do not exist for any reason, an empty string is returned.
func GetLatestStatusMessage(managed dynamic.Interface, policyName string, templateIdx int) string {
	replicatedPolicyName := UserNamespace + "." + policyName
	policyInterface := managed.Resource(GvrPolicy).Namespace(ClusterNamespace)

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
