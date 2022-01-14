# Governance Policy Framework
[![Build Status](https://travis-ci.com/stolostron/governance-policy-framework.svg?token=1xoYGv8XzWhB2heDk2My&branch=master)](https://travis-ci.com/stolostron/governance-policy-framework)

Red Hat Advanced Cluster Management Governance - Policy Framework

The policy framework provides governance capability to gain visibility and drive remediation for various security and configuration aspects to help meet such enterprise standards.

## What it does

View the following functions of the policy framework: 

* Applies policies to managed clusters from hub cluster.
* Collects policy execution results from managed cluster to hub cluster.
* Provides an extensible mechanism to bring your own policy.

## Architecutre

The policy framework consists of following components:

- [Governance dashboard](https://github.com/stolostron/grc-ui)
- [Policy propagator](https://github.com/stolostron/governance-policy-propagator) 
- [Policy spec sync](https://github.com/stolostron/governance-policy-spec-sync)
- [Policy status sync](https://github.com/stolostron/governance-policy-status-sync)
- [Policy template sync](https://github.com/stolostron/governance-policy-template-sync)
- Policy controllers: Policy controllers include predefined [out-of-box policy controllers](#out-of-box-policies-and-controllers), or you can [bring your own policy](#bring-your-own-policy).

![architecture](images/policy-framework-architecture-diagram.jpg)

## Out-of-box policies and controllers

View the following list of predefined policy controllers that are offered with Red Hat Advanced Cluster Management:

- [Configuration policy controller](https://github.com/stolostron/config-policy-controller)
- [Certificate policy controller](https://github.com/stolostron/cert-policy-controller)
- [IAM policy controller](https://github.com/stolostron/iam-policy-controller)

## Bring your own policy

You can bring your own policy by implementing a custom policy and controller. For more information, see the blog, [Develop your own policy controller to integrate with Red Hat Advanced Cluster Management for Kubernetes](https://www.openshift.com/blog/develop-your-own-policy-controller-to-integrate-with-red-hat-advanced-cluster-management-for-kubernetes).

## More policies

You can find more policies or contribute to the open repostitory, [policy-collection](https://github.com/stolostron/policy-collection).
