
Deploy
```bash
# deploy operator
kubectl create -f benchmarks/cockroach_operator/cpe_v1_cockroachoperator_helm.yaml
# wait for cockroachdb running
watch kubectl get po -n cockroach-operator-system
# load database
kubectl create -f benchmarks/cockroach_operator/tpcc_load_job.yaml
# wait for tpcc job completed
watch kubectl get po
# run
kubectl create -f benchmarks/cockroach_operator/cpe_v1_cockroach_tpcc.yaml
```

Clear
```bash
kubectl delete pod --field-selector=status.phase==Succeeded

kubectl delete -f benchmarks/cockroach_operator/cpe_v1_cockroachoperator_helm.yaml
kubectl delete pvc -n cockroach-operator-system --all
```