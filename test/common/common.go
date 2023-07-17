// Copyright Contributors to the Open Cluster Management project

package common

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/mod/semver"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"open-cluster-management.io/governance-policy-propagator/test/utils"
)

var (
	KubeconfigHub          string
	KubeconfigManaged      string
	UserNamespace          string
	ClusterNamespace       string
	ClusterNamespaceOnHub  string
	OCMNamespace           string
	OCMAddOnNamespace      string
	DefaultTimeoutSeconds  int
	ManuallyPatchDecisions bool
	K8sClient              string
	IsHosted               bool

	ClientHub            kubernetes.Interface
	ClientHubDynamic     dynamic.Interface
	ClientManaged        kubernetes.Interface
	ClientManagedDynamic dynamic.Interface
	ClientHosting        kubernetes.Interface
	ClientHostingDynamic dynamic.Interface
)

const MaxTimeoutSeconds = 900 // 15 minutes

func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	flagset.StringVar(
		&KubeconfigHub, "kubeconfig_hub", "../../kubeconfig_hub",
		"Location of the kubeconfig to use; defaults to KUBECONFIG if not set",
	)
	flagset.StringVar(
		&KubeconfigManaged, "kubeconfig_managed", "../../kubeconfig_managed",
		"Location of the kubeconfig to use; defaults to KUBECONFIG if not set",
	)

	flagset.BoolVar(
		&IsHosted, "is_hosted", false,
		"Whether is hosted mode or not",
	)
	flagset.StringVar(&UserNamespace, "user_namespace", "policy-test", "ns on hub to create root policy")
	flagset.StringVar(&ClusterNamespace, "cluster_namespace", "local-cluster", "cluster ns name")
	flagset.StringVar(&ClusterNamespaceOnHub, "cluster_namespace_on_hub", "", "cluster ns name on hub")
	flagset.StringVar(&OCMNamespace, "ocm_namespace", "open-cluster-management", "ns of ocm installation")
	flagset.StringVar(
		&OCMAddOnNamespace,
		"ocm_addon_namespace",
		"open-cluster-management-agent-addon",
		"ns of ocm addon installations",
	)
	flagset.IntVar(&DefaultTimeoutSeconds, "timeout_seconds", 30, "Timeout seconds for assertion")
	flagset.BoolVar(
		&ManuallyPatchDecisions, "patch_decisions", true,
		"Whether to 'manually' patch PlacementRules with PlacementDecisions "+
			"(set to false if the PlacementRule controller is running)",
	)
	flagset.StringVar(
		&K8sClient, "k8s_client", "oc",
		"Which k8s client to use for some tests - `oc`, `kubectl`, "+
			"or something else entirely",
	)
}

// Initializes the Hub and Managed Clients. Should be called after InitFlags,
// and before any tests using common functions are run.
func InitInterfaces(hubConfig, managedConfig string, isHosted bool) {
	if isHosted {
		ClientHosting = NewKubeClient("", hubConfig, "")
		ClientHostingDynamic = NewKubeClientDynamic("", hubConfig, "")
	} else {
		ClientHosting = NewKubeClient("", managedConfig, "")
		ClientHostingDynamic = NewKubeClientDynamic("", managedConfig, "")
	}

	if ClusterNamespaceOnHub == "" {
		ClusterNamespaceOnHub = ClusterNamespace
	}

	ClientHub = NewKubeClient("", hubConfig, "")
	ClientHubDynamic = NewKubeClientDynamic("", hubConfig, "")
	ClientManaged = NewKubeClient("", managedConfig, "")
	ClientManagedDynamic = NewKubeClientDynamic("", managedConfig, "")
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
		klog.V(5).Infof(
			"clientcmd.BuildConfigFromFlags for url %s using %s\n",
			url,
			filepath.Join(usr.HomeDir, ".kube", "config"),
		)

		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not create a valid kubeconfig")
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
		klog.V(2).Infof("OC command output %s\n", output)
	}

	var exitError *exec.ExitError

	ok := errors.As(err, &exitError)
	if ok {
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

func OcHosting(args ...string) (string, error) {
	if IsHosted {
		args = append([]string{"--kubeconfig=" + KubeconfigHub}, args...)
	} else {
		args = append([]string{"--kubeconfig=" + KubeconfigManaged}, args...)
	}

	return oc(args...)
}

func OutputDebugInfo(testName string, kubeconfig string, additionalResources ...string) {
	GinkgoWriter.Printf("%s test Kubernetes info:\n", testName)

	resources := []string{
		"policies.policy.open-cluster-management.io",
		"placementrules.apps.open-cluster-management.io",
		"placements.cluster.open-cluster-management.io",
		"placementbindings.policy.open-cluster-management.io",
	}
	resources = append(resources, additionalResources...)

	for _, resource := range resources {
		_, _ = utils.KubectlWithOutput("get", resource, "--all-namespaces", "-o", "yaml", "--kubeconfig="+kubeconfig)
	}
}

// IsAtLeastVersion detects OCP versions given an x.y version lower bound
func IsAtLeastVersion(minVersion string) bool {
	clusterVersion, err := ClientManagedDynamic.Resource(GvrClusterVersion).Get(
		context.TODO(),
		"version",
		metav1.GetOptions{},
	)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// no version CR, not ocp
			klog.V(5).Info("This is not an OCP cluster")
		} else {
			klog.Infof("Encountered an error fetching the OCP version: %v", err)
		}

		return false
	}

	version, _, _ := unstructured.NestedString(clusterVersion.Object, "status", "desired", "version")

	klog.V(5).Info("OCP Version " + version)

	// Convert to valid semantic versions by adding the "v" prefix
	minSemVer := fmt.Sprintf("v%s", minVersion)
	ocpSemVer := semver.MajorMinor(fmt.Sprintf("v%s", version))

	// Compare returns: 0 if ver1 == ver2, -1 if ver1 < ver2, or +1 if ver1 > ver2
	return semver.Compare(ocpSemVer, minSemVer) >= 0
}

func CleanupHubNamespace(namespace string) {
	By("Deleting namespace " + namespace)

	err := ClientHub.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	}

	// Wait for the namespace to be fully deleted before proceeding.
	EventuallyWithOffset(1,
		func() bool {
			_, err := ClientHub.CoreV1().Namespaces().Get(
				context.TODO(), namespace, metav1.GetOptions{},
			)
			isNotFound := k8serrors.IsNotFound(err)
			if !isNotFound && err != nil {
				GinkgoWriter.Printf("'%s' namespace 'get' error: %w", err)
			}

			return isNotFound
		},
		DefaultTimeoutSeconds*6,
		1,
	).Should(BeTrue(), fmt.Sprintf("Namespace %s should be deleted.", namespace))
}
