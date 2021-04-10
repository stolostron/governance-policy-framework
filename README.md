[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# Governance Policy Framework
[![Build Status](https://travis-ci.com/open-cluster-management/governance-policy-framework.svg?token=2jHocNax82kqKsGV1uTE&branch=main)](https://travis-ci.com/open-cluster-management/governance-policy-framework)

Open Cluster Management - Governance Policy Framework

The policy framework provides governance capability to gain visibility and drive remediation for various security and configuration aspects to help meet such enterprise standards.

## What it does

View the following functions of the policy framework: 

* Applies policies to managed clusters from hub cluster.
* Collects policy execution results from managed cluster to hub cluster.
* Provides an extensible mechanism to bring your own policy.

## Architecutre

![architecture](images/policy-framework-architecture-diagram.jpg)

The governance policy framework consists of following components:

- [Governance dashboard](https://github.com/open-cluster-management/grc-ui) -- UI
- Govenance policy framework -- A framework to distribute various supported policies to managed cluster and collect results back to hub. It consists of following components:
    - [Policy propagator](https://github.com/open-cluster-management/governance-policy-propagator) 
    - [Policy spec sync](https://github.com/open-cluster-management/governance-policy-spec-sync)
    - [Policy status sync](https://github.com/open-cluster-management/governance-policy-status-sync)
    - [Policy template sync](https://github.com/open-cluster-management/governance-policy-template-sync)
- Policy controllers -- Policy engines running on managed cluster to evaluate policy rules distributed by the policy framework and generate results.
    - [Configuration policy controller](https://github.com/open-cluster-management/config-policy-controller)
    - [Certificate policy controller](https://github.com/open-cluster-management/cert-policy-controller)
    - [IAM policy controller](https://github.com/open-cluster-management/iam-policy-controller)
    - [Gatekeeper](https://github.com/open-policy-agent/gatekeeper)
    - [Kyverno](https://github.com/kyverno/kyverno/)
    - [Bring your own](#bring-your-own-policy-controller)

## The Policy CRDs

The `Policy` is the Custom Resource Definition (CRD), created for policy framework controllers to monitor. It acts as a vechical to deliver policies to managed cluster and collect results back to hub.
This is an example spec of a `Policy` object:
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

The `PlacementBinding` CRD is used to bind the `Policy` with `PlacementRule`. Only bound `Policy` will be distributed by policy framework to the managed cluster
This is an example spec of a `PlacementBinding` object:
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
This is an example spec of a `PlacementRule` object:
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
## Bring your own policy controller

You can bring your own policy by implementing a custom policy and controller. For more information, see the blog, [Develop your own policy controller to integrate with Red Hat Advanced Cluster Management for Kubernetes](https://www.openshift.com/blog/develop-your-own-policy-controller-to-integrate-with-red-hat-advanced-cluster-management-for-kubernetes).

## More policies

You can find more policies or contribute to the open repository, [policy-collection](https://github.com/open-cluster-management/policy-collection).
