# configuration policy use case

## Basic usage
### Create

1. [create a k8s resource in a single namespace](./create/create-single-ns.yaml)
1. [create same k8s resource in multiple namespaces](./create/create-multiple-ns.yaml)

### Merge Patch

1. [patch an existing k8s resource in a single namespace](./merge-patch/merge-patch-role-single-ns.yaml)
1. [patch same k8s resource in multiple namespaces](./merge-patch/merge-patch-role-multiple-ns.yaml)

### Replace Patch

1. [replace an existing k8s resource in a single namespace](./replace-patch/replace-patch-role-single-ns.yaml)
1. [replace same k8s resource in multiple namespaces](.y/replace-patch/replace-patch-role-multiple-ns.yaml)

### Delete
1. [delete a k8s resource in a single namespace](./delete/delete-role-single-ns.yaml)
1. [delete same k8s resource in multiple namespaces](./delete/delete-role-multiple-ns.yaml)

### Audit
1. [audit a single resource in a single namespace](./audit/audit-role-single-ns.yaml)
1. [audit a single resource in multiple namespaces](./audit/audit-role-multiple-ns.yaml)
1. [audit a kind of resource](./audit/audit-pod-kind.yaml)
1. [audit a kind of resource with desired fields and value](./audit/audit-pod-kind-field-filter.yaml)

## Advanced usage
### Integrate with Gatekeeper
1. [Install and configure Gatekeeper](./gatekeeper/gatekeeper-install.yaml)
2. [Create Gatekeeper policy](./gatekeeper/gatekeeper-policy-sample.yaml#L14-L66)
3. Report Gatekeeper violations for audit scenario
4. Report Gatekeeper violations for admission scenario

### Integrate with Kyverno
1. Install Kyverno
2. Create Kyverno policy
3. Report Kyverno violations
