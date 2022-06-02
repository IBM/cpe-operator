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