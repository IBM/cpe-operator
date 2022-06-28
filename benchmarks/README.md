## How to use the off-the-shelf/your own operator
#### Prerequiries
- CRD file, and yaml or helm for deployment of your operator

1. Create a repo and put your operator files (crd and yaml files or helm repo for deployment) there so that CPE can get them 
2. If your CR uses its original completion status, you need to add adaptor to CPE (see [operator_adaptor.go](controllers/operator_adaptor.go))
3. Create the benchmark operator yaml for your operator (see the below templates)
4. Deploy your benchmark operator
5. Create Benchmark job file (see the below template)
    1. Specify your benchmark operator w/ spec.benchmarkOperator.name in your Benchmark file
    2. Define configuration variables of your benchmark application in benchmarkSpec
    3. Set the values of the variables that you want to change w/ iterationSpec.iterations
6. Deploy your Benchmark job

## Benchmark Operator
Define the operator deployment spec and its benchmark or job GVK (group-version-kind)
#### Deploy by YAML
```yaml
apiVersion: cpe.cogadvisor.io/v1
kind: BenchmarkOperator
metadata:
  name: [BenchmarkOperator name]
  namespace: [BenchmarkOperator namespace]
spec:
  apiVersion: [benchmark apiVersion]
  kind: [benchmark kind]
  adaptor: [benchmark adaptor]
  crd:
    host: [operator crd host for role binding]
    paths:
    - [operator crd path for role binding]
  deploySpec:
    namespace: [deploying namespace]
    yaml:
      host: [host for general deploying yaml]
      paths:
      - [paths for general deploying yaml]
```
#### Note:
- Put a deployment yaml of namespace at the beginning of the yaml path list.
- Use one yaml for one resource. The system does not handle multiple resources in the same file.

#### Deploy by Helm
```yaml
apiVersion: cpe.cogadvisor.io/v1
kind: BenchmarkOperator
metadata:
  name: [operator name]
spec:
  apiVersion: [benchmark apiVersion]
  kind: [benchmark kind]
  adaptor: [benchmark adaptor]
  crd:
    host: [operator crd host for role binding]
    paths:
    - [operator crd path for role binding]
  deploySpec:
    namespace: [deploying namespace]
    helm:
      entity: [package]
      release: [release name]
      repoName: [repo name]
      url: [helm repo url]
      valuesYaml: |
        [modified values in YAML format]
```
To create repo from your git repo: https://blog.softwaremill.com/hosting-helm-private-repository-from-github-ff3fa940d0b7

## Benchmark
```yaml
apiVersion: cpe.cogadvisor.io/v1
kind: Benchmark
metadata:
  name: [Benchmark name]
  namespace: [Benchmark namespace]
spec:
  benchmarkOperator:
    name: [BenchmarkOperator name]
    namespace: [BenchmarkOperator namespace]
  benchmarkSpec: |
    [spec will be appended to defined benchmark GVK .spec]
  trackBuildConfigs: [build tracker arguments]
  iterationSpec: [iteration arguments]
  parserKey: [parser arguments]
  sidecar: true|false
  repetition: [repeating number of run]
```
