# Iteration Support
source code: [iteration.go](../controllers/iteration.go)

There are two kinds of iterations are supported.
**Scenario (basic iteration)**:  iteration on application parameters for comparing results on different kind of workload
**Configuration**: iteration on benchmark operator or node parameters for tuning application aiming at best performance

### IterationSpec
Both iteration are definied in `iterationSpec` key of cpe.cogadvisor.io/v1/Benchmark as below.

```yaml
spec:
    iterationSpec:
        iterations:
        - name: [variable name]
            values:
            - [list of values]
        configurations:
        - name: [variable name]
            values:
            - [list of values] 
        nodeSelection:
          location: [location to nodeSelector]
          values:
          - [list of tuning profile name]
          selector:
            [node label selector; matchLabels or/and matchExpressions]
            # matchLabels:
            #  label-key: label-value
            # matchExpressions:
            #  - { key: label-key, operator: <In|NotIn,Exists,DoesNotExist>, values: [label-values] }
        sequential: [true|false]
        minimize: [true|false]

```

When Benchmark Controller calls [CreateFromOperator](../controllers/common.go),
- The iteration module will find all combinations in the list of each iteration item and create a Job for each combination with the following job name format: `    
  ```bash
  [benchmark name]-cpeh-[hash32 of <iterations, build, repetition>]
  ```
- Each item in `values` will be used to replace in the benchmarkSpec using [Go Template](https://pkg.go.dev/text/template). 
- For auto-tuning node, the location in `nodeSelection` key is used to specifiy the key location of the target node selector with dotted notation.
- The iteration item will be also labeled to the job. 
- These labels will be sent to Parser component as a `constLabels` attribute.
- `constLabels` will be later pushed as a label to prometheus, see [output](../output/README.md) for more detail.
- `sequential` is to indicate whether the iterated job should run at the same time in parallel or sequentially
- `minimize` is to specify that lower number of performance value is better (default, higher is better)
- `nodeSelection` key is considered as special configuration with the iteration name `profile`

### Composite Iteration 
Composite iteration referes to iteration value that is composed of more than two variable values at the same time.
We support by processing the delimit ';' as an array variable. See [PARSEC benchmark example](../examples/none/cpe_parsec.yaml).
### Node Profile Tuning
[tuned.go](../controllers/tuned.go)
This feature is dependent on [Openshift Node Tuning Operator](https://docs.openshift.com/container-platform/4.8/scalability_and_performance/using-node-tuning-operator.html). This will automatically check valid profile name from `profiles.v1.tuned.openshift.io/rendered`, label the node with each iterated value of `.nodeSelection.values`, then check `profiles.v1.tuned.openshift.io` whether the correct profile name is applied before creating a benchmark job.

- The match label in recommend section of customized tuned resource must be in the following format.
see [example](node-tuning/roks_ext_profile.yaml)
```yaml
  recommend:
  - match:
    - label: profile
      value: [profile name]
    priority: [integer that lower than existing tuned and unique]
    profile: [profile name]
    operand:
      debug: false
```

### Hash Status
The hash value of iterations will be listed in the benchmark status
```yaml
status:
  hash:
  - build: [buildID]
    hash: [jobHash]
    iterations:
      [iterationName]: [iteartionValue]
    run: [run number]
```

### Results Status
When the job is completed and output is parsed and pushed as describe in [output](../output/README.md), the job tracker will also update `.spec.results` and `.spec.bestResults`.
- `.spec.results` lists each iteration results.
- `.spec.bestResults` presents the best performed configuration for each scenarioID derived by `.iterationSpec.iterations` iterations. The best performed configuration is determined by maximum performance value returned from the specified parser. In case of more than one repetition, the average value of all runs will be used.

## Auto-tuning Profile
Set `nodeSelection` value to **auto-tuned** will activate node auto-tuning mechanism
; see [auto-tuned Coremark benchmark](../examples/none/autotuned/coremark.yaml)

To edit node tuning search space, edit configmap `cpe-operator-node-tuning-search-space` and restart controller pod

```
kubectl edit configmap cpe-operator-node-tuning-search-space -n cpe-operator-system
kubectl delete pod $(kubectl get po -n cpe-operator-system|grep controller|tail -1|awk '{print $1}') -n cpe-operator-system
```