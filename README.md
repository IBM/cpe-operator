<!-- TOC tocDepth:2..4 chapterDepth:2..6 -->

- [Installation](#installation)
  - [1. Install supplementary/complementary systems](#1-install-supplementarycomplementary-systems)
  - [2. Deploy controller (and parser)](#2-deploy-controller-and-parser)
    - [2.1. Deploy default CPE system (controller with parser)](#21-deploy-default-cpe-system-controller-with-parser)
    - [2.2. Deploy with recommended CPE system (controller with parser and service monitor)](#22-deploy-with-recommended-cpe-system-controller-with-parser-and-service-monitor)
    - [2.3. Custom deploy using kustomize](#23-custom-deploy-using-kustomize)
  - [3. Deploy Operators and Benchmark](#3-deploy-operators-and-benchmark)
- [Roadmap](#roadmap)

<!-- /TOC -->

# CPE Operator

CPE operator is a project that originally implements the AutoDECK framework. AutoDECK (**Auto**mated **DEC**larative Performance Evaluation and Tuning Framework on **K**ubernetes) is an evaluation system of Kubernetes-as-a-Service (KaaS) that automates configuring, deploying, evaluating, summarizing, and visualizing benchmarking workloads with a fully declarative manner. 

![system](img/system.PNG)

## Installation

### 1. Install supplementary/complementary systems

System|Description|Installation guide
---|---|---
Prometheus Operator|for monitoring and visualization |[read more](metric/README.md)
Openshift Build Controller|for image tracking|[read more](https://docs.openshift.com/container-platform/4.7/rest_api/workloads_apis/buildconfig-build-openshift-io-v1.html)
Node Tuning Operator|for node tuning|[read more](https://docs.openshift.com/container-platform/4.2/nodes/nodes/nodes-node-tuning-operator.html)
Cloud Object Storaget (COS)|for job result logging|[read more](./output/README.md)

### 2. Deploy controller (and parser)

Clone the repo and enter the workspace
```bash
git clone https://github.com/IBM/cpe-operator.git
cd cpe-operator
```

Deploy with either of the following choices:

#### 2.1. Deploy default CPE system (controller with parser)

   ```bash
   kubectl create -f ./config/samples/cpe-operator/default.yaml
   ```

#### 2.2. Deploy with recommended CPE system (controller with parser and service monitor)
  
  - require `Prometheus Operator` to be deployed.
  - need to replace `openshift-monitoring` with the namespace that prometheus has been deployed for correcting [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) resource.

   ```bash
   kubectl create -f ./config/samples/cpe-operator/recommended.yaml
   ```

#### 2.3. Custom deploy using kustomize

  2.3.1. Set IMAGE_REGISTRY to your registry and update image in kustomization.yaml

  ```bash
  export IMAGE_REGISTRY=[your registry URL]
  export VERSION=[your image version tag]
  ```
  
  ** VERSION value must be specified as a valid semantic version for operator-sdk (Major.Minor.Patch)

  2.3.2.  Modify kustomization in [/config](./config/) 

  TAG|description|dependencies|to-modify kustomization|note
  ---|---|---|---|---
  [PARSER]|Deploy CPE parser|-|[default](./config/default/kustomization.yaml)|Can specifiy `PARSER_IMG` environment for custom parser image
  [PROMETHEUS]|Deploy ServiceMonitor and RBAC|Prometheus Operator|[default](./config/default/kustomization.yaml)|May need to modify namespace label in [manager.yaml](./config/manager/manager.yaml) and [RBAC](./config/prometheus/rbac.yaml) depending on Prometheus deployment 
  [AUTO-TUNE]|Deploy tuning namespace and mounted to controller|Node Tuning Operator|[default](./config/default/kustomization.yaml)|
  [LOG-COS]|Set environment for COS secret|Cloud Object Storage secret (see [/output](./output/README.md#raw-output-collection))|[default](./config/default/kustomization.yaml) (and [parser](./config/parser/kustomization.yaml) if [PARSER] enabled)|

  2.3.3.  Deploy custom manifests

  ```bash
  # require operator-sdk (>= 1.4), go (>= 1.13)
  make deploy

  # confirm cpe operator is running
  kubectl get po -n cpe-operator-system
  # see manager log
  kubectl logs $(kubectl get po -n cpe-operator-system|grep controller|tail -1|awk '{print $1}') -n cpe-operator-system -c manager
  ```

> To remove this operator run: `make undeploy`

### 3. Deploy Operators and Benchmark

See [How to use the off-the-shelf/your own operator](./examples)

Deploy simple coremark benchmark deployment:
```bash
kubectl create -f examples/none/cpe_coremark.yaml
```

Try benchmark with custom benchmark operator:
```bash
kubectl create -f examples/benchmark_operator/cpe_v1_benchmarkoperator_helm.yaml
# confirm ripsaw operator is running
kubectl get po -n my-ripsaw
kubectl create -f examples/benchmark_operator/cpe_v1_benchmark_iperf3.yaml
# confirm the job
kubectl get po -n my-ripsaw
```

## Roadmap
- [x] design custom resource; see [benchmark_types.go](api/v1/benchmark_types.go), [benchmarkoperator_types.go](api/v1/benchmarkoperator_types.go)
- [x] integrate to off-the-shelf benchmark operator; see [benchmarks](benchmarks/README.md)
- [x] implement build tracker; see [tracker](tracker/README.md)
- [x] raw output collector/parser; see [output](output/README.md)
  - [ ] integrate wrapper from [snafu](https://github.com/cloud-bulldozer/benchmark-wrapper/tree/master/snafu)
- [x] iteration support; see [iteration](iteration/README.md)
  - [x] app-parameter variation (scenario)
  - [x] spec configuration
  - [x] node profile tuning
- [x] visualize multi-cluster; see [multi-cluster](metric/README.md#multi-cluster-integration)
- [ ] insert a sidecar if set
- [ ] combine resource usage metric; see [metric](metric/README.md)
    - [x] prometheus-export metrics
    - [ ] app-export metrics
    - [ ] eBPF metric collector