# CPE Monitoring System
![single-cluster](img/single-cluster-monitoring.png)
## External Component Setup and Installation
1. New `cpe-monitoring-system` namespace
   ```bash
   # For openshift
   oc new-project cpe-monitoring-system
   # For general kubernetes
   kubectl create ns cpe-monitoring-system
   kubectl config set-context $(kubectl config current-context) --namespace cpe-monitoring-system
   ```
2. Install Prometheus Operator and Grafana Operator
   - For openshift, you can install both operators from Operator Hub
   - For general kubernetes, you can install from stable helm chart
   ```bash
    # add stable repo and update (if not added yet)
    helm repo add stable https://charts.helm.sh/stable
    helm repo update

    helm install -n cpe-monitoring-system prometheus stable/prometheus-operator
   ```
3. Deploy PushGateway
    ```bash
    helm install -n cpe-monitoring-system pushgateway stable/prometheus-pushgateway
    # Deploy service monitor
    kubectl create -f servicemonitor/pushgateway-helm.yaml
    # Check pushgateway service name and port
    kubectl get svc -n cpe-monitoring-system
    ```
4. Deploy Service Monitor (and certificate if need)
   - For openshift, we may copy service monitor from original openshift-monitoring namespace
     ```bash
     kubectl get cm -n openshift-monitoring kubelet-serving-ca-bundle -o yaml --export|kubectl create -f -
     kubectl get cm -n openshift-monitoring serving-certs-ca-bundle -o yaml --export|kubectl create -f -
     
     # Deploy Service Monitor
     kubectl get servicemonitor -n openshift-monitoring kubelet -o yaml --export|kubectl create -f -
     ```
   - For helm chart installation, node exporter as well as kubelet service monitor are already placed
   - Relabel ServiceMonitor: [check here](#metric-exporters)
5. [for Thanos Sidecar] Deploy COS Storage config for thanos forwarding: [see more](https://thanos.io/tip/thanos/storage.md/)
    ```bash
    # S3-based example
    export BUCKET_NAME=[your bucket to store log/metrics]
    export ENDPOINT=[endpoint]
    export REGION=[region]
    export ACCESS_KEY=[access key]
    export SECRET_KEY=[secret key]
    envsubst < thanos/thanos-storage-config.yaml | kubectl apply -f -
    ```
6. [for Thanos Sidecar] Deploy Prometheus with Thanos Sidecar saving to COS
   - For openshift, we need to create new prometheus resource
        ```bash
        export CLUSTER_ID=`kubectl config current-context | awk -F"/" '{ print $1 }'`
        envsubst < prometheus/prometheus.yaml | kubectl create -f 
        kubectl create -f prometheus/cluster-monitoring-view-adv.yaml
        oc adm policy add-cluster-role-to-user cluster-monitoring-view-adv -z prometheus-k8s
        kubectl create -f prometheus/cluster-role-prometheus-roks.yaml
        oc adm policy add-cluster-role-to-user prometheus-roks -z prometheus-k8s
        ```
    - For helm installation, default prometheus is installed
        - edit to add thanos sidecar
        ```bash
        export CLUSTER_ID=`kubectl config current-context | awk -F"/" '{ print $1 }'`
        kubectl edit prometheus prometheus-prometheus-oper-prometheus
        > replace with prometheus/prometheus.yaml .spec 
        > replace ${CLUSTER_ID}
        # Add monitor role
        kubectl create -f prometheus/cluster-monitoring-view-adv.yaml
        kubectl create -f prometheus/cr-binding.yaml
        ```
7. [For Thanos Query] Deploy Thanos Store Gateway
   ```bash
   kubectl create -f store/
   ```
8. [For Thanos Query] Deploy Thanos Query
    ```bash
    kubectl create -f thanos/thanos-query.yaml
    ```
9. [For Grafana] Deploy Grafana
    - For Openshift, we need to create new grafana resoruce 
        ```bash
        export GRAFANA_NAMESPACE=cpe-monitoring-system
        export ADMIN_PWD=[your grafana admin password]
        export GRAFANA_SERVICE_ACCOUNT=grafana-serviceaccount
        # Apply grafana component after installing operators
        oc create -f grafana/grafana_added.yaml -n $GRAFANA_NAMESPACE
        envsubst < grafana/grafana.yaml | oc -n $GRAFANA_NAMESPACE apply -f -
        # Wait until pod running
        # Add monitoring role
        oc adm policy add-cluster-role-to-user cluster-monitoring-view-adv -z $GRAFANA_SERVICE_ACCOUNT
        # Set BEARER TOKEN
        export BEARER_TOKEN=$(oc sa get-token $GRAFANA_SERVICE_ACCOUNT -n $GRAFANA_NAMESPACE) 
        # Deploy thanos-query datasource
        envsubst < grafana/grafana-datasource.yaml | oc -n $GRAFANA_NAMESPACE apply -f -
        # Restart
        kubectl delete replicaset -l app=grafana
        ```
    - For helm installation, default grafana is already installed, need to apply only new data source
        ```bash
        # Add monitoring role
        kubectl create -f prometheus/cluster-monitoring-view-adv.yaml
        export GRAFANA_NAMESPACE=cpe-monitoring-system
        export GRAFANA_SERVICE_ACCOUNT=prometheus-grafana
        envsubst < grafana/cr-binding.yaml | kubectl create -f -
        # Set BEARER TOKEN
        TOKENNAME=$(kubectl -n $GRAFANA_NAMESPACE get serviceaccount/$GRAFANA_SERVICE_ACCOUNT -o jsonpath='{.secrets[0].name}')
        export BEARER_TOKEN=`kubectl -n $GRAFANA_NAMESPACE get secret $TOKENNAME -o jsonpath='{.data.token}'| base64 --decode`
        # Deploy thanos-query datasource
        envsubst < grafana-datasource.yaml | kubectl -n $GRAFANA_NAMESPACE apply -f -
        # Restart
        kubectl delete replicaset -l app=grafana
        ```
        
## Metric Exporters
To enhance CPE visualization with metrics from exporters, we utilize ServiceMonitor resource of prometheus operator to perform relabeling process and extract benchmark, and job label from the pod name by following relabeling item in ServiceMonitor
```yaml
    metricRelabelings:
    - regex: (.*)(\-cpeh)(.*)
      replacement: '${1}'
      sourceLabels:
        - pod
      targetLabel: benchmark
    - regex: '(.*)(\-cpeh)(\-[0-9a-z]+)(\-[0-9a-z]+)'
      replacement: '${1}${2}${3}'
      sourceLabels:
        - pod
      targetLabel: job
``` 
example: [kubelet.yaml](servicemonitor/kubelet.yaml)
This is also applicable to application-specific expoter in both operator level and benchmark level

## Low-level metric exporter
[TO-DO]
- collect metric on-demand by pod name (optional)
- export pod label
- create ServiceMonitor with metricRelabelings section

## Multi-Cluster Integration
![multi-cluster](img/multi-cluster-monitoring.png)

### Thanos Query
add store to thanos querier arguments: [thanos-query.yaml](./thanos/thanos-query.yaml)
```yaml
          args:
            - query
            [...]
            - --store=dnssrv+_grpc._tcp.[sidecar ingress hostname]
            - --store=dnssrv+_grpc._tcp.[storegateway ingress hostname]
```
### Setup secured federation scrape with Prometheus Operator
- secure prometheus app of target cluster by connecting ingress with App ID by [instruction here](https://cloud.ibm.com/docs/appid?topic=appid-kube-auth&locale=it)
- in case of deploying ingress in new namespace (not kube-system or default), check [this issue](https://stackoverflow.com/questions/65230804/ibm-cloud-how-to-enable-app-id-for-app-on-kubernetes-cluster-with-k8s-ingress-a)

- copy ingress secret from target cluster to core cluster

- create file `federate_job.yaml`

  ```yaml
  - job_name: 'federate'
    scrape_interval: 5m
    scrape_timeout: 1m
    honor_labels: true
    metrics_path: '/federate'
    params:
        'match[]':
        - '{job=~".*"}'
    scheme: https
    static_configs:
    - targets:
        - <prometheus server host name without https>
        labels:
          origin: <cluster name>
    oauth2:
        client_id: <service credential Client ID>
        client_secret: <service credential secret>
        token_url: https://<region>.appid.cloud.ibm.com/oauth/v4/<tenant ID>/token
        endpoint_params:
        grant_type: client_credentials
        username: <app Client ID>
        password: <app secret>
    tls_config:
        cert_file: /etc/prometheus/secrets/<ingress secret name>/tls.crt
        key_file: /etc/prometheus/secrets/<ingress secret name>/tls.key
        insecure_skip_verify: true
  ```

- generate federate secret
  
  ```bash
  kubectl create secret generic federate-scrape-config -n cpe-monitoring-system --from-file=federate_job.yaml
  ```

- add the following specification to Prometheus resource
    ```yaml
    spec:
        ...
        # mount ingress secret to prometheus container
        containers:
        - name: prometheus
          ...
          volumeMounts:
          - mountPath: /etc/tls/trl-tok-iks
              name: secret-<ingress secret name>
              readOnly: true
        # add secret
        secrets:
        - <ingress secret name>
        # add federate job
        additionalScrapeConfigs:
          key: federate_job.yaml
          name: federate-scrape-config
    ```
reference: https://prometheus.io/docs/prometheus/latest/federation
** The prometheus pod must be restarted to apply new configuration. **