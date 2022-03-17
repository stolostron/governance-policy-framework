// Copyright Contributors to the Open Cluster Management project

package common

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/onsi/gomega"
	policiesv1 "github.com/stolostron/governance-policy-propagator/api/v1"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	KubeconfigHub         string
	KubeconfigManaged     string
	UserNamespace         string
	ClusterNamespace      string
	DefaultTimeoutSeconds int
)

func init() {
	flag.StringVar(&KubeconfigHub, "kubeconfig_hub", "../../kubeconfig_hub", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
	flag.StringVar(&KubeconfigManaged, "kubeconfig_managed", "../../kubeconfig_managed", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
	flag.StringVar(&UserNamespace, "user_namespace", "policy-test", "ns on hub to create root policy")
	flag.StringVar(&ClusterNamespace, "cluster_namespace", "local-cluster", "cluster ns name")
	flag.IntVar(&DefaultTimeoutSeconds, "timeout_seconds", 30, "Timeout seconds for assertion")
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
		gomega.Expect(err).To(gomega.BeNil())
		for _, statusPerCluster := range policy.Status.Status {
			if statusPerCluster.ClusterNamespace == clusterNamespace {
				return statusPerCluster.ComplianceState
			}
		}

		return nil
	}
}

func OcHub(args ...string) (string, error) {
	args = append([]string{"--kubeconfig=" + KubeconfigHub}, args...)
	// Determine whether output should be logged
	printOutput := true
	for _, a := range args {
		if a == "whoami" || strings.HasPrefix(a, "secret") {
			printOutput = false
			break
		}
	}
	output, err := exec.Command("oc", args...).Output()
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

func OcManaged(args ...string) (string, error) {
	args = append([]string{"--kubeconfig=" + KubeconfigManaged}, args...)
	output, err := exec.Command("oc", args...).CombinedOutput()
	if len(args) > 0 && args[0] != "whoami" {
		fmt.Println(string(output))
	}
	return string(output), err
}

func PatchPlacementRule(namespace, name, targetCluster, kubeconfigHub string) error {
	_, err := utils.KubectlWithOutput(
		"patch",
		"-n",
		namespace,
		"placementrule.apps.open-cluster-management.io",
		name,
		"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/clusterSelector/matchExpressions\", \"value\":[{\"key\": \"name\", \"operator\": \"In\", \"values\": ["+targetCluster+"]}]}]",
		"--kubeconfig="+kubeconfigHub,
	)

	return err
}
