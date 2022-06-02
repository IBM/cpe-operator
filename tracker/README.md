# Image Build Tracking on Openshift Platform
---
### Benchmark Spec

Define list of build resource to track with `trackBuildConfigs` key

```yaml
  trackBuildConfigs:
  - kind: [buildconfig kind]
    name: [buildconfig name]
    namespace: [buildconfig namespace]
```

### BuildWatcher
- [BuildWatcher](../controllers/build_watcher.go) watch UpdateEvent of `build.openshift.io/v1/builds` wait for first-time status.phase=Complete
- BuildWatcher match `status.configs` to `cpe.cogadvisor.io/v1/benchmarks`
- BuildWatcher update buildname to `status.trackedBuilds` list `cpe.cogadvisor.io/v1/benchmarks` 
- [BenchmarkController](../controllers/benchmark_controller.go) get Update request of `cpe.cogadvisor.io/v1/benchmarks`
- BenchmarkController add new job according to operator attached with the build name

#### Ray Image Build Example:
##### 1. Prepare source repo
[reference](https://blog.softwaremill.com/hosting-helm-private-repository-from-github-ff3fa940d0b7)
- fork [ray-project/ray](https://github.com/ray-project/ray) to [ray-repo-clone](https://github.com/sunya-ch/ray-repo-clone)
- add `{{ .Values.operatorNamespace }}` to template
- add securityContext.privileged
- run
    ```bash
    cd ray
    helm package .
    helm repo index . --url https://raw.githubusercontent.com/sunya-ch/ray-repo-clone/master/deploy/charts/ray
    git add .
    git commit -m "index helm"
    git push git@github.com:sunya-ch/ray-repo-clone.git master
    ```

##### 2. Create ImageStream tag 
[buildconfig/example-ray-job-image-stream.yaml](buildconfig/example-ray-job-image-stream.yaml)
```yaml
apiVersion: image.openshift.io/v1
kind: ImageStream
metadata:
  name: example-ray-job-image
spec:
  lookupPolicy:
    local: false
  tags:
  - name: latest
    from:
      kind: DockerImage
      name: res-cpe-team-docker-local.artifactory.swg-devops.com/cpe/tracker/example-ray-job
    referencePolicy:
      type: Source
```
##### 3. Create BuildConfig
[buildconfig/example-ray-job-build.yaml](buildconfig/example-ray-job-build.yaml)
```yaml
apiVersion: build.openshift.io/v1
kind: BuildConfig
metadata:
  name: ray-example
  labels:
    app: ray-example
spec:
  triggers: 
    - type: "GitHub"
      github:
        secret: "rayrepoclone"
  source:
    type: Git
    git:
      uri: https://github.com/sunya-ch/ray-repo-clone
    contextDir: docker/ray
  strategy:
    type: Docker                      
    dockerStrategy:
      dockerfilePath: Dockerfile
  output:
    to:
      kind: ImageStreamTag
      name: example-ray-job-image:latest
    pushSecret:
      name: res-cpe-team-docker-local
```

