# Governance Policy Framework
[![Build Status](https://travis-ci.com/open-cluster-management/governance-policy-framework.svg?token=1xoYGv8XzWhB2heDk2My&branch=master)](https://travis-ci.com/open-cluster-management/governance-policy-framework)

Red Hat Advance Cluster Management Governance - Policy Framework

Policy framework provides governance capability to gain visibility and drive remediation for various security and configuration aspects to help meet such enterprise standards.

## What it does
1. Applies policies to managed clusters from hub cluster
2. Collects policy execution results from managed cluster to hub cluster
3. Provides an extensible mechanism to bring your own policy

## Architecutre
The policy framework consists of following components
- [Policy propagator](https://github.com/open-cluster-management/governance-policy-propagator) 
- [Policy spec sync](https://github.com/open-cluster-management/governance-policy-spec-sync)
- [Policy status sync](https://github.com/open-cluster-management/governance-policy-status-sync)
- [Policy template sync](https://github.com/open-cluster-management/governance-policy-template-sync)
- Policy controllers
  - out-of-box
  - bring your own

![architecture](images/policy-framework-architecture-diagram.jpg)

## Out-of-box policies and controllers
- [configuration policy controller](https://github.com/open-cluster-management/config-policy-controller)
- [cert expiration policy controller](https://github.com/open-cluster-management/cert-policy-controller)
- [iam policy controller](https://github.com/open-cluster-management/iam-policy-controller)
- [cis policy controller](https://github.com/open-cluster-management/cis-controller)

## Bring your own policy
You can bring your own policy by implementing a custom policy and controller. Read [medium article](https://medium.com/ibm-cloud/develop-your-own-policy-controller-to-integrate-with-ibm-cloud-pak-for-multicloud-management-b5a83f8396e)

## More policies
Find more policies in [policy-collection](https://github.com/open-cluster-management/policy-collection) repo