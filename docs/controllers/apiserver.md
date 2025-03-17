# APIServer Controller

The apiserver controller reconciles `APIServer` and is responsible for providing the k8s cluster environment that is used by the services that are configured for the MCP.

## Configuration

Unlike most of the other controllers that are part of the MCP operator, the apiserver controller requires a configuration file to work properly. This file defines the specifics of where and how the k8s cluster environments are created by the controller.

The config file contains one top-level key per configured handler for cluster provisioning.

### Gardener

The configuration for using Gardener as cluster provisioner is stored under the `gardener` key. It can be specified in two variants: a simple 'single' mode, or a more complex 'multi' mode.

#### Single Mode
```yaml
gardener:
  cloudProfile: gcp
  regions:
  - name: europe-west1
  - name: europe-west3
  - name: us-central1
  - name: asia-south1
  defaultRegion: us-central1
  shootTemplate:
    metadata:
      annotations:
        test.openmcp.cloud/config: single
    spec:
      networking:
        type: "calico"
        nodes: "10.180.0.0/16"
      provider:
        type: gcp
        infrastructureConfig:
          apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
          kind: InfrastructureConfig
          networks:
            workers: 10.180.0.0/16
        controlPlaneConfig:
          apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
          kind: ControlPlaneConfig
          zone: ""
        workers:
        - name: worker-0
          machine:
            type: n1-standard-2
            image:
              name: gardenlinux
              version: 1312.3.0
            architecture: amd64
          maximum: 2
          minimum: 1
          volume:
            type: pd-balanced
            size: 50Gi
      secretBindingName: test
  project: test
  kubeconfig: |
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        certificate-authority-data: ZHVtbXkK
        server: https://127.0.0.1:55761
      name: dummy
    contexts:
    - context:
        cluster: dummy
        user: dummy
      name: dummy
    current-context: dummy
    users:
    - name: dummy
      user:
        token: asdf
```

- `cloudprofile` _string_ - The name of the Gardener `CloudProfile` to use for shoot creation.
- `regions` _array_ - A list of regions. Only regions which are specified here _and_ in the cloudprofile will be available.
  - `name` _string_ - The name of the region.
- `defaultRegion` _string_ - The default region to use, unless specified otherwise.
- `shootTemplate` _object_ - A template to use for the shoot creation. There are a few things to note here:
  - For the valid values and effects of each field in here, please check the [Gardener documentation](https://github.com/gardener/gardener/blob/master/docs/README.md).
  - The shoot template is mainly used for the creation of shoots with workers (`APIServer`s with the `GardenerDedicated` type). For workerless shoots (`Gardener` type), only `metadata.annotations` and `metadata.labels` are taken into account.
  - Most of the fields under `spec.provider` are specific to the chosen cloud provider and have to fit to each other and the chosen cloudprofile.
    - See below for examples for AWS and GCP.
  - Some of the fields in here might be adapted before the actual shoot is created. For example, the worker count is set to `3`, if high-availability is configured.
- `project` _string_ - Name of the Gardener `Project` to create the shoot clusters in.
- `kubeconfig` _string_ - A kubeconfig for the Garden cluster of the Gardener landscape.

##### infrastructureConfig & controlplaneConfig

###### AWS
```yaml
controlPlaneConfig:
  apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
  kind: ControlPlaneConfig
infrastructureConfig:
  apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
  kind: InfrastructureConfig
  networks:
    vpc:
      cidr: 10.180.0.0/16
    zones: # optional
    - name: eu-central-1a
      workers: 10.180.0.0/19
      public: 10.180.32.0/20
      internal: 10.180.48.0/20
```

AWS shoots require subnet CIDR ranges for each zone that workers are put into. These zones can either be specified in the shoot template, or the apiserver controller tries to default them based on the VPC CIDR. It tries to default CIDR ranges similar to the ones shown above for all zones available in the chosen region. Note that, with a `/16` subnet CIDR for the VPC, only four zones (with one `/19` and two `/20` CIDRs) fit into the network range.

###### GCP

```yaml
controlPlaneConfig:
  apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
  kind: ControlPlaneConfig
  zone: "" # injected by controller
infrastructureConfig:
  apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
  kind: InfrastructureConfig
  networks:
    workers: 10.180.0.0/16
```

The controlplane zone is injected by the apiserver controller.

#### Multi Mode

As one might have noticed, the above configuration causes all MCP shoots to be created on the same Gardener landscape, in the same project, with the same cloud provider. For more complex use-cases, multiple configurations can be passed in.

```yaml
gardener:
  defaultConfig: default/gcp
  landscapes:
  - name: default
    kubeconfig: |
      apiVersion: v1
      kind: Config
      clusters:
      - cluster:
          certificate-authority-data: ZHVtbXkK
          server: https://127.0.0.1:55761
        name: dummy
      contexts:
      - context:
          cluster: dummy
          user: dummy
        name: dummy
      current-context: dummy
      users:
      - name: dummy
        user:
          token: asdf
    configs:
    - name: gcp
      cloudProfile: gcp
      regions:
        - name: europe-west1
        - name: europe-west3
        - name: us-central1
        - name: asia-south1
      defaultRegion: us-central1
      shootTemplate:
        metadata:
          annotations:
            test.openmcp.cloud/config: multi/default/gcp
        spec:
          networking:
            type: "calico"
            nodes: "10.180.0.0/16"
          provider:
            type: gcp
            infrastructureConfig:
              apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
              kind: InfrastructureConfig
              networks:
                workers: 10.180.0.0/16
            controlPlaneConfig:
              apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
              kind: ControlPlaneConfig
              zone: ""
            workers:
              - name: worker-0
                machine:
                  type: n1-standard-2
                  image:
                    name: gardenlinux
                    version: 1312.3.0
                  architecture: amd64
                maximum: 2
                minimum: 1
                volume:
                  type: pd-balanced
                  size: 50Gi
          secretBindingName: test
      project: test
    - name: aws
      cloudProfile: aws
      regions:
        - name: eu-central-1
        - name: eu-west-1
        - name: us-east-1
        - name: ap-southeast-1
      defaultRegion: eu-central-1
      shootTemplate:
        metadata:
          annotations:
            test.openmcp.cloud/config: multi/default/aws
        spec:
          networking:
            type: "calico"
            nodes: "10.180.0.0/16"
          provider:
            type: aws
            infrastructureConfig:
              apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
              kind: InfrastructureConfig
              networks:
                vpc:
                  cidr: 10.180.0.0/16
            controlPlaneConfig:
              apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
              kind: ControlPlaneConfig
              zone: ""
            workers:
              - name: worker-0
                machine:
                  type: m5.large
                  image:
                    name: gardenlinux
                    version: 1592.1.0
                  architecture: amd64
                maximum: 2
                minimum: 1
                volume:
                  type: gp3
                  size: 50Gi
          secretBindingName: test
      project: test2
  - name: extra
    kubeconfig: |
      apiVersion: v1
      kind: Config
      clusters:
      - cluster:
          certificate-authority-data: ZHVtbXkK
          server: https://127.0.0.1:55761
        name: dummy
      contexts:
      - context:
          cluster: dummy
          user: dummy
        name: dummy
      current-context: dummy
      users:
      - name: dummy
        user:
          token: asdf
    configs:
    - name: foo
      cloudProfile: gcp
      regions:
        - name: europe-west1
        - name: europe-west3
        - name: us-central1
        - name: asia-south1
      defaultRegion: us-central1
      shootTemplate:
        metadata:
          annotations:
            test.openmcp.cloud/config: multi/extra/foo
        spec:
          networking:
            type: "calico"
            nodes: "10.180.0.0/16"
          provider:
            type: gcp
            infrastructureConfig:
              apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
              kind: InfrastructureConfig
              networks:
                workers: 10.180.0.0/16
            controlPlaneConfig:
              apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
              kind: ControlPlaneConfig
              zone: ""
            workers:
              - name: worker-0
                machine:
                  type: n1-standard-2
                  image:
                    name: gardenlinux
                    version: 1312.3.0
                  architecture: amd64
                maximum: 2
                minimum: 1
                volume:
                  type: pd-balanced
                  size: 50Gi
          secretBindingName: test
      project: foo
```

While syntax and semantic of the config fields explained above remain the same, the structure changes slightly and a few new fields are introduced:
- `defaultConfig` _string_ - The configuration to use by default.
  - This has to follow the format `<landscape>/<config>`.
- `landscapes` _array_ - A list of Gardener landscapes. Each landscape has its own kubeconfig for the Garden cluster and can have multiple configurations.
  - `name` _string_ - The name of the landscape. Used for referencing this section of the configuration.
  - `kubeconfig` _string_ - The kubeconfig for the Garden cluster, as explained above.
  - `configs` _array_ - A list of configurations.
    - `name` _string_ - The name of the config. Used for referencing this section of the configuration.
    - Basically, each item is a complete single-mode config as explained above, without the `kubeconfig` (which has moved one level up).

##### Converting a Single Config to a Multi Config

If this is your single configuration
```yaml
gardener:
  cloudProfile: <cloudprofile>
  regions: <regions>
  defaultRegion: <defaultRegion>
  shootTemplate: <shootTemplate>
  project: <project>
  kubeconfig: <kubeconfig>
```
it can easily be converted into an equivalent multi config:
```yaml
gardener:
  defaultConfig: foo/bar
  landscapes:
  - name: foo
    kubeconfig: <kubeconfig>
    configs:
    - name: bar
      cloudProfile: <cloudprofile>
      regions: <regions>
      defaultRegion: <defaultRegion>
      shootTemplate: <shootTemplate>
      project: <project>
```

> Internally, sinlge configs are always converted into multi configs, using `default` as landscape and config name.

##### Working with Multi Configs

To use a different config than the default one, it has to be specified under `spec.internal.gardener.landscapeConfiguration` in the `APIServer` resource:
```yaml
apiVersion: core.openmcp.cloud/v1alpha1
kind: APIServer
<...>
spec:
  internal:
    gardener:
      landscapeConfiguration: foo/bar
```

To create such an `APIServer` resource via a `ManagedControlPlane`, one has to create a corresponding `InternalConfiguration` resource:
```yaml
apiVersion: core.openmcp.cloud/v1alpha1
kind: InternalConfiguration
metadata:
  name: my-mcp
  namespace: my-project
spec:
  components:
    apiServer:
      gardener:
        landscapeConfiguration: default/aws
```
Putting this `InternalConfiguration` next to a `ManagedControlPlane` named `my-mcp` in namespace `my-project` would result in the MCP controller rendering the landscape configuration into the `APIServer` resource.

###### Caveats

There are a few things that should be noted when working with multiple configurations:
- `InternalConfigurations` can only be seen and modified by landscape operators, not by end-users. This means that there is currently no way to expose multiple configurations to customer (because this feature was implemented for development and testing purposes).
- The apiserver controller one of the first controllers to react on a new MCP resource, as most others depend on the cluster. This means that, if you want to create a non-default cluster, you have to make sure the `InternalConfiguration` that overwrites the default has to exist already before the corresponding `ManagedControlPlane` is read by the MCP controller for the first time. Otherwise, the apiserver controller would likely start to create a shoot from the wrong configuration.
- As the used config determines not only some shoot specifics, but also cloud provider as well as Gardener project and landscape, you should _never_ change the used config while the corresponding shoot exists. So, don't create or delete the `InternalConfiguration` if the shoot already exists and don't change the value of `spec.internal.gardener.landscapeConfiguration` (validation should prevent the latter one). In the best case, this would lead to an orphaned shoot, but it might also mess up the MCP in other undesirable ways.

