/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/version"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	ctrl "sigs.k8s.io/controller-runtime"

	"open-cluster-management.io/governance-policy-addon-controller/pkg/addon/configpolicy"
	"open-cluster-management.io/governance-policy-addon-controller/pkg/addon/policyframework"
)

//+kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=get;create
//+kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests;certificatesigningrequests/approval,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,verbs=approve
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=clustermanagementaddons,verbs=get;list;watch

// RBAC below will need to be updated if/when new policy controllers are added.

//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;patch;update,resourceNames=governance-policy-framework;config-policy-controller

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=create
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;update;patch;delete,resourceNames="open-cluster-management:policy-framework-hub";"open-cluster-management:config-policy-controller-hub"
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=create
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;update;patch;delete,resourceNames="open-cluster-management:policy-framework-hub";"open-cluster-management:config-policy-controller-hub"

// Cannot limit based on resourceNames because the name is dynamic in hosted mode.
//+kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=create;delete;get;list;patch;update;watch

//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons,verbs=create
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons,verbs=delete,resourceNames=config-policy-controller;governance-policy-framework
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons/finalizers,verbs=update,resourceNames=config-policy-controller;governance-policy-framework
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons/status,verbs=update;patch,resourceNames=config-policy-controller;governance-policy-framework

//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=clustermanagementaddons/finalizers,verbs=update,resourceNames=config-policy-controller;governance-policy-framework
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=addondeploymentconfigs,verbs=get;list;watch

// Permissions required for policy-framework
// (see https://kubernetes.io/docs/reference/access-authn-authz/rbac/#privilege-escalation-prevention-and-bootstrapping)

//+kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies/finalizers,verbs=update
//+kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies/status,verbs=get;patch;update
//+kubebuilder:rbac:groups=core,resources=secrets,resourceNames=policy-encryption-key,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;get;list;patch;update;watch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get;list;watch

var (
	setupLog    = ctrl.Log.WithName("setup")
	ctrlVersion = version.Info{}
)

const (
	ctrlName = "governance-policy-addon-controller"
)

func main() {
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	cmd := &cobra.Command{
		Use:   ctrlName,
		Short: "Governance policy addon controller for Open Cluster Management",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Help(); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			os.Exit(1)
		},
	}

	ctrlconfig := controllercmd.NewControllerCommandConfig(ctrlName, ctrlVersion, runController)
	ctrlconfig.DisableServing = true

	ctrlcmd := ctrlconfig.NewCommandWithContext(context.TODO())
	ctrlcmd.Use = "controller"
	ctrlcmd.Short = "Start the addon controller"

	cmd.AddCommand(ctrlcmd)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runController(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	mgr, err := addonmanager.New(controllerContext.KubeConfig)
	if err != nil {
		setupLog.Error(err, "unable to create new addon manager")
		os.Exit(1)
	}

	agentFuncs := []func(addonmanager.AddonManager, *controllercmd.ControllerContext) error{
		policyframework.GetAndAddAgent,
		configpolicy.GetAndAddAgent,
	}

	for _, f := range agentFuncs {
		err := f(mgr, controllerContext)
		if err != nil {
			setupLog.Error(err, "unable to get or add agent addon")
			os.Exit(1)
		}
	}

	err = mgr.Start(ctx)
	if err != nil {
		setupLog.Error(err, "problem starting manager")
		os.Exit(1)
	}

	<-ctx.Done()

	return nil
}
