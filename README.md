[comment]: # " Copyright Contributors to the Open Cluster Management project "

# Governance Policy Framework

[![GRC Integration Test](https://github.com/stolostron/governance-policy-framework/actions/workflows/integration.yml/badge.svg)](https://github.com/stolostron/governance-policy-framework/actions/workflows/integration.yml)

Open Cluster Management - Governance Policy Framework

The policy framework provides governance capability to gain visibility, and drive remediation for
various security and configuration aspects to help meet such enterprise standards.

## What it does

View the following functions of the policy framework:

- Distributes policies to managed clusters from hub cluster.
- Collects policy execution results from managed cluster to hub cluster.
- Supports multiple policy engines and policy languages.
- Provides an extensible mechanism to bring your own policy.

## Architecture

![architecture](images/policy-framework-architecture-diagram.png)

The governance policy framework consists of the following components:

- Govenance policy framework: A framework to distribute various supported policies to managed
  clusters and collect results to be sent to the hub cluster. The framework replicates `Policy` Custom Resources (CRs) from the "user namespace" to the "cluster namespace" on the hub cluster. The `Policy` CRs in the "cluster namespace" are further replicated to the managed clusters.
  - [Policy propagator](https://github.com/stolostron/governance-policy-propagator)
  - [Governance policy framework addon](https://github.com/stolostron/governance-policy-framework-addon)
- Policy controllers: Policy engines that run on managed clusters to evaluate policy rules
  distributed by the policy framework and generate results. The results are reported back to the hub cluster.
  - [Configuration policy controller](https://github.com/stolostron/config-policy-controller)
    - [Usage examples](./doc/configuration-policy/README.md)
  - [Certificate policy controller](https://github.com/stolostron/cert-policy-controller)
  - Third-party (optional)
    - [Gatekeeper](https://github.com/open-policy-agent/gatekeeper)
    - [Kyverno](https://github.com/kyverno/kyverno/)

If desired, users can request automated actions to perform when a policy is violated. These automated actions consist of `PolicyAutomation` CRs and `AnsibleJob` CRs as shown in the hub cluster.

## The Policy CRDs

The `Policy` is the Custom Resource Definition (CRD), created for policy framework controllers to
monitor. It acts as a vehicle to deliver policies to managed cluster and collect results to send to
the hub cluster.

View the following example specification of a `Policy` object:

```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  name: policy-pod
spec:
  remediationAction: inform # [inform/enforce] If set, it defines the remediationAction globally.
  disabled: false # [true/false] If true, the policy will not be distributed to the managed cluster.
  policy-templates:
    - objectDefinition: # Use `objectDefinition` to wrap the policy resource to be distributed to the managed cluster
        apiVersion: policy.open-cluster-management.io/v1
        kind: ConfigurationPolicy
        metadata:
          name: policy-pod-example
        spec:
          remediationAction: inform
          object-templates:
            - complianceType: musthave
              objectDefinition:
                apiVersion: v1
                kind: Pod
                metadata:
                  name: sample-nginx-pod
                  namespace: default
                spec:
                  containers:
                    - image: nginx:1.7.9
                      name: nginx
                      ports:
                        - containerPort: 80
```

The `PlacementBinding` CRD is used to bind the `Policy` with a `Placement`. Only a bound `Policy` is
distributed to a managed cluster by the policy framework.

View the following example specification of a `PlacementBinding` object:

```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
  name: binding-policy-pod
placementRef:
  name: placement-policy-pod
  kind: Placement
  apiGroup: cluster.open-cluster-management.io
subjects:
  - name: policy-pod
    kind: Policy
    apiGroup: policy.open-cluster-management.io
```

The `Placement` CRD is used to determine the target clusters to distribute policies to.

View the following example specification of a `Placement` object:

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: placement-policy-pod
spec:
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchExpressions:
            - { key: environment, operator: In, values: ["dev"] }
  tolerations:
    - key: cluster.open-cluster-management.io/unreachable
      operator: Exists
    - key: cluster.open-cluster-management.io/unavailable
      operator: Exists
```

## How to install it

You can find installation instructions from
[Open Cluster Management](https://open-cluster-management.io/) website.

## More policies

You can find more policies or contribute to the open repository,
[policy-collection](https://github.com/stolostron/policy-collection).

<!---
Date: 09/18/2024
-->
