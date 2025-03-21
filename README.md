[![REUSE status](https://api.reuse.software/badge/github.com/openmcp-project/mcp-operator)](https://api.reuse.software/info/github.com/openmcp-project/mcp-operator)

# mcp-operator

## About this project

This repository contains the controllers which reconcile ManagedControlPlane resources.

This document provides a preliminary overview of the contract between the ManagedControlPlane controller and the component controllers. A more comprehensive and detailed version will be added shortly.

### Component Controllers

Component controllers have to follow a specific contract in order to work together with the rest of the onboarding system. Below, the general reconciliation flow that a component controller should follow is quickly summarized, then some aspects are explained in more detail.

#### Reconciliation Flow

- Fetch resource from cluster.
- Check for operation annotations and act accordingly.
- If the component has dependencies, fetch the corresponding resources.
- Check if resource is in deletion and proceed with the corresponding logic.
- Update component status (unless the resource was deleted).
  - Note that the status should also be updated in case of an error, so that the failure is visible to operators and the customer.

##### Create/Update

- Ensure finalizer on own component resource.
- Ensure dependency finalizers on all resources of components depended on.
- Perform actual component logic.

##### Delete

- Check if there are any dependency finalizers on the own resource. If so, update the status accordingly and requeue the resource to wait until all dependency finalizers have been removed. Do not proceed with any actual deletion logic if there are dependency finalizers left!
- Perform actual component deletion logic.
- If the deletion has to wait for something:
  - Update status accordingly and requeue resource.
- If the deletion is finished:
  - Remove own dependency finalizers from all resources of components depended on.
  - Remove own finalizer from own resource. This should result in the resource being deleted.

#### No-Gos for Component Controllers

This is a (incomplete) list of things that component controller **must not** do:

- Modify any component resource's spec, including their own one.
  - The 'ground truth' for the spec of a component resource is the `ManagedControlPlane`, from which it is generated.
- Read or modify the owning `ManagedControlPlane`.
  - Component resources are expected to be self-contained. It should never be necessary to even fetch the owning `ManagedControlPlane` resource, and under no circumstances should it be modified by a component's controller. The `ManagedControlPlane`'s spec is configured by the customer, its `status` is managed by the ManagedControlPlane Controller (which also fetches the component resources' status on its own).
  - Reading the `ManagedControlPlane`'s status to check the conditions of other components for dependency reasons would be ok, but if a component depends on another one, it will likely need some information only available on the component's resource (not exported into the `ManagedControlPlane`'s status) anyway and should therefore fetch that component directly.
    - All component resources generated a `ManagedControlPlane` share its name and namespace. The `componentutils` package has helper functions to easily fetch resources belonging to other components.
- Expose secret/internal information in a component's condition or external status.
  - The external part of a component's status as well as its conditions will be visible to the customer who created the `ManagedControlPlane`.

#### Reacting to Changes

Component controllers are expected to reconcile their own resource if

- the resource's spec changes
- the resource's labels change
- an operation annotation with value `reconcile` was added to the resource
- a deletion timestamp was added to the resource

Apart from that, there is usually no need to react to other changes to the resource.
The `componentutils` package provides some event filters that can be passed into the `ControllerBuilder` to apply corresponding filtering rules. The `DefaultComponentControllerPredicates(...)` function should work as a default filter configuration for most component controllers.

#### Annotations

The `componentutils` package contains a `PatchAnnotation` function to easily add or remove an annotation to/from a resource.

##### The Operation Annotation

The annotation `openmcp.cloud/operation` (available as constant `OperationAnnotation` in the `types` repo) can be used to control the behavior of the controller responsible for reconciling the annotated resource. Currently, two values are supported:

- **reconcile** means the resource should be reconciled as if it was changed. The corresponding controller is expected to remove the annotation and perform the reconciliation.
- **ignore** means that the resource should be ignored. The corresponding controller (and all other ones touching the resource) is expected to treat this resource as if it didn't exist. It must not remove the annotation or change the resource in any way.

#### Finalizers

The component's controller is expected to put a finalizer onto the component's resource. The finalizer should follow the format `openmcp.cloud.<lowercase component type>`, e.g. `openmcp.cloud.apiserver` for the `APIServer` component. The `ComponentType` type has a `Finalizer()` method that returns the finalizer for a given component type.

When depending on another component, the depending component's controller is expected to add a dependency finalizer to the required other component. It can fetch the component's resource using this snippet (using a dependency towards the `APIServer` component as an example):

```golang
  // r is the Reconciler struct
  // obj is the currently reconciled component resource
 ownCPGeneration, ownICGeneration, _ := componentutils.GetCreatedFromGeneration(obj)
 apiServerComp, err := componentutils.GetComponent(ctx, r.Client, openmcpv1alpha1.APIServerComponent, obj.Name, obj.Namespace)
 if err != nil {
  return ctrl.Result{}, fmt.Errorf("error checking for APIServer dependency component: %w", err)
 }
 if apiServerComp == nil || !componentutils.IsDependencyReady(apiServerComp.Condition(), ownCPGeneration, ownICGeneration) {
  log.Debug("APIServer not found or it isn't ready")
    // TODO: update own status and requeue for retry
 }
 as, ok := apiServerComp.(*openmcpv1alpha1.APIServer)
 if !ok {
  panic("resource of APIServer component is not a APIServer")
 }
```

Dependency finalizers have the format `dependency.openmcp.cloud/<lowercase component type>` and can be generated from a `ComponentType` by using its `DependencyFinalizer()` method.

The `componentutils` package contains several helper functions to deal with dependencies.

#### Conditions

Each component resource's status is expected to contain at least one condition displaying the current state of the component. Each condition must have a **globally unique** identifier (because all of them are merged in the `ManagedControlPlane`'s status) and a status that must be either `True`, `False`, or `Unknown`. For any condition with a non-`True` status, the `reason` and `message` fields should be set to provide error messages or other information about why the condition is not `True`. The `reason` field is expected to contain a enum-like, CamelCase string that can be evaluated programmatically, while the `message` field should contain a human-readable message.

The `componentutils` package contains functions that help with updating a list of conditions.

#### Kubebuilder Scaffolding

This project uses a structure which diverges from standard kubebuilder inside the `cmd` package. As a result, not all scaffolding functionality works out of the box. Most prominently this affects webhook scaffolding. In order to work around this we create a `cmd/main.go` shim file before running any scaffolding:

```sh
cat << EOF > cmd/main.go
package main

import (
 "fmt" // we need to import something here so golint is happy
 //+kubebuilder:scaffold:imports
)

func main() {
 fmt.Println("I should never be called")
 //+kubebuilder:scaffold:builder
}
EOF
```

Unfortunately leaving the shim `cmd/main.go` file in, would break any go builds or tests which use the `./...` operator. As a result before committing, remove the shim main.go again:

```sh
rm cmd/main.go
```

#### Test using envtest

In order to run any tests which are using envtest, you need etcd, kube-apiserver and kubectl. You can make use of the `api/utils/envtest/` package in your tests to automatically install them. This is mainly needed, because we cannot easily run additional commands in CI.


## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/openmcp-project/mcp-operator/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/openmcp-project/mcp-operator/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/openmcp-project/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright 2025 SAP SE or an SAP affiliate company and mcp-operator contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/openmcp-project/mcp-operator).
