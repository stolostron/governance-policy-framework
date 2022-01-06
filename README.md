[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# Governance Policy Framework
[![Build Status](https://travis-ci.com/stolostron/governance-policy-framework.svg?token=2jHocNax82kqKsGV1uTE&branch=main)](https://travis-ci.com/stolostron/governance-policy-framework)

Open Cluster Management - Governance Policy Framework

The policy framework provides governance capability to gain visibility, and drive remediation for various security and configuration aspects to help meet such enterprise standards.

## What it does

View the following functions of the policy framework: 

* Distributes policies to managed clusters from hub cluster.
* Collects policy execution results from managed cluster to hub cluster.
* Supports multiple policy engines and policy languages.
* Provides an extensible mechanism to bring your own policy.

## Architecutre

![architecture](images/policy-framework-architecture-diagram.jpg)

The governance policy framework consists of following components:

- [Governance dashboard](https://github.com/stolostron/grc-ui): Console
- Govenance policy framework: A framework to distribute various supported policies to managed clusters and collect results to be sent to the hub cluster.
    - [Policy propagator](https://github.com/stolostron/governance-policy-propagator) 
    - [Policy spec sync](https://github.com/stolostron/governance-policy-spec-sync)
    - [Policy status sync](https://github.com/stolostron/governance-policy-status-sync)
    - [Policy template sync](https://github.com/stolostron/governance-policy-template-sync)
- Policy controllers: Policy engines that run on managed clusters to evaluate policy rules distributed by the policy framework and generate results.
    - [Configuration policy controller](https://github.com/stolostron/config-policy-controller)
      - [Usage examples](./doc/configuration-policy/README.md)
    - [Certificate policy controller](https://github.com/stolostron/cert-policy-controller)
    - [IAM policy controller](https://github.com/stolostron/iam-policy-controller)
    - Third-party
      - [Gatekeeper](https://github.com/open-policy-agent/gatekeeper)
      - [Kyverno](https://github.com/kyverno/kyverno/)

## The Policy CRDs

The `Policy` is the Custom Resource Definition (CRD), created for policy framework controllers to monitor. It acts as a vehicle to deliver policies to managed cluster and collect results to send to the hub cluster.

View the following example specification of a `Policy` object:
```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  name: policy-pod
spec:
  remediationAction: inform         # [inform/enforce] If set, it defines the remediationAction globally.
  disabled: false                   # [true/false] If true, the policy will not be distributed to the managed cluster.
  policy-templates:             
    - objectDefinition:             # Use `objectDefinition` to wrap the policy resource to be distributed to the managed cluster
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

The `PlacementBinding` CRD is used to bind the `Policy` with a `PlacementRule`. Only a bound `Policy` is distributed to a managed cluster by the policy framework.

View the following example specification of a `PlacementBinding` object:
```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
  name: binding-policy-pod
placementRef:
  name: placement-policy-pod
  kind: PlacementRule
  apiGroup: apps.open-cluster-management.io
subjects:
- name: policy-pod
  kind: Policy
  apiGroup: policy.open-cluster-management.io
```

The `PlacementRule` CRD is used to determine the target clusters to distribute policies to.

View the following example specification of a `PlacementRule` object:
```yaml
apiVersion: apps.open-cluster-management.io/v1
kind: PlacementRule
metadata:
  name: placement-policy-pod
spec:
  clusterConditions:
  - status: "True"
    type: ManagedClusterConditionAvailable
  clusterSelector:
    matchExpressions:
      - {key: environment, operator: In, values: ["dev"]}
```

## How to install it

You can find installation instructions from [Open Cluster Management](https://stolostron.io/) website.

## More policies

You can find more policies or contribute to the open repository, [policy-collection](https://github.com/stolostron/policy-collection).


<!---
Date: 12/6/2021
-->
