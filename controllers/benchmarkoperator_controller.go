/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

///////////////////////////////////////////////////////////////////////////
//
// benchmarkoperator_controller.go
//
// - Reconcile Loop
//   create benchmark operator from yaml files or helm chart
//   create RBAC resource for the defined job resource for allow this controller
//   to create the target job
//
////////////////////////////////////////////////////////////////////////////

package controllers

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	helmclient "github.com/mittwald/go-helm-client"
)

const (
	defaultOperatorNamespace = "cpe-operator-system"
)

var (
	operatorNamespace string = getOperatorNamespace()
)

// BenchmarkOperatorReconciler reconciles a BenchmarkOperator object
type BenchmarkOperatorReconciler struct {
	*kubernetes.Clientset
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	DC         *discovery.DiscoveryClient
	DYN        dynamic.Interface
	HelmClient helmclient.Client
}

const operatorFinalizer = "finalizers.benchmarkoperators.cpe.cogadvisor.io"

//+kubebuilder:rbac:groups=cpe.cogadvisor.io,resources=benchmarkoperators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cpe.cogadvisor.io,resources=benchmarkoperators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cpe.cogadvisor.io,resources=benchmarkoperators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BenchmarkOperator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile

func Find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func (r *BenchmarkOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("benchmarkoperator", req.NamespacedName)

	r.Log.Info(fmt.Sprintf("Operator Request #%v ", req.NamespacedName))
	instance := &cpev1.BenchmarkOperator{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{RequeueAfter: ReconcileTime}, nil
	}

	is_deleted := instance.GetDeletionTimestamp() != nil
	if is_deleted {
		if controllerutil.ContainsFinalizer(instance, operatorFinalizer) {
			if err := r.finalizeBenchmarkOperator(reqLogger, instance); err != nil {
				return ctrl.Result{}, nil
			}

			controllerutil.RemoveFinalizer(instance, operatorFinalizer)
			err := r.Client.Update(ctx, instance)
			if err != nil {
				return ctrl.Result{}, nil
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(instance, operatorFinalizer) {
		controllerutil.AddFinalizer(instance, operatorFinalizer)
		err = r.Client.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, nil
		}
	} else {
		// deploy benchmark operator
		// 1. create namespace
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: instance.Spec.DeploySpec.Namespace}}
		r.Client.Create(ctx, ns)

		// 2. deploy yamls
		host := instance.Spec.CRD.Host
		pathList := instance.Spec.CRD.Paths

		gvk := GetSimpleJobGVK(instance)

		var rules []rbacv1.PolicyRule
		var apigroups []string
		// init with job crd
		apigroups = append(apigroups, gvk.Group)
		for _, path := range pathList {
			yamlURL := host + path
			r.Log.Info(fmt.Sprintf("Create from yaml %s ", yamlURL))
			newResource, err := CreateFromURL(r.DC, r.DYN, yamlURL)
			if err != nil {
				r.Log.Info(fmt.Sprintf("Path %s err #%v ", yamlURL, err))
				continue
			}
			crdSpec := newResource.Object["spec"].(map[string]interface{})
			crdGroup := crdSpec["group"].(string)
			found := false
			for _, group := range apigroups {
				if group == crdGroup {
					found = true
					break
				}
			}
			if !found {
				apigroups = append(apigroups, crdGroup)
				rule := rbacv1.PolicyRule{
					APIGroups: []string{crdGroup},
					Verbs:     []string{rbacv1.VerbAll},
					Resources: []string{rbacv1.ResourceAll},
				}
				rules = append(rules, rule)
			}
		}

		// add role for list, get, create, delete job resource
		jobRule := rbacv1.PolicyRule{
			APIGroups: []string{gvk.Group},
			Verbs:     []string{"list", "get", "create", "delete", "watch"},
			Resources: []string{rbacv1.ResourceAll},
		}
		rules = append(rules, jobRule)
		r.Log.Info(fmt.Sprintf("Create job rule %v: %s,%s", jobRule, gvk.Group, gvk.Kind))

		// add cluster roles
		roleName := instance.ObjectMeta.Name + "-cpe-cluster-role"
		roleMeta := metav1.ObjectMeta{Name: roleName}
		role := &rbacv1.ClusterRole{
			ObjectMeta: roleMeta,
			Rules:      rules,
		}
		r.Client.Create(ctx, role)
		r.Log.Info(fmt.Sprintf("Create cluster role %s ", roleName))

		bindName := instance.ObjectMeta.Name + "-cpe-cluster-role-binding"

		roleRef := rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		}
		bindMeta := metav1.ObjectMeta{Name: bindName}
		binding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: bindMeta,
			Subjects: []rbacv1.Subject{
				rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "cpe-operator-controller-manager",
					Namespace: "cpe-operator-system",
				},
			},
			RoleRef: roleRef,
		}
		r.Client.Create(ctx, binding)
		r.Log.Info(fmt.Sprintf("Create cluster role binding %s ", bindName))

		if !reflect.DeepEqual(cpev1.HelmSpec{}, instance.Spec.DeploySpec.Helm) {
			err = r.installHelm(ctx, instance)
		} else if !reflect.DeepEqual(cpev1.YAMLSpec{}, instance.Spec.DeploySpec.YAML) {
			err = r.installYAML(instance)
		} else {
			r.Log.Info("No deployment specification")
		}

		if err != nil {
			r.Log.Info(fmt.Sprintf("Deployment err #%v ", err))
		}
	}
	return ctrl.Result{}, nil
}

func (r *BenchmarkOperatorReconciler) installHelm(ctx context.Context, instance *cpev1.BenchmarkOperator) error {
	r.Log.Info(fmt.Sprintf("Install helm %s/%s ", instance.Spec.DeploySpec.Helm.RepoName, instance.Spec.DeploySpec.Helm.Entity))
	chartRepo := repo.Entry{}
	if instance.Spec.DeploySpec.Helm.Username == "" {
		chartRepo = repo.Entry{
			Name: instance.Spec.DeploySpec.Helm.RepoName,
			URL:  instance.Spec.DeploySpec.Helm.URL,
		}
	} else {
		chartRepo = repo.Entry{
			Name:     instance.Spec.DeploySpec.Helm.RepoName,
			URL:      instance.Spec.DeploySpec.Helm.URL,
			Username: instance.Spec.DeploySpec.Helm.Username,
			Password: instance.Spec.DeploySpec.Helm.Password,
		}
	}

	releaseName := instance.Spec.DeploySpec.Helm.Release
	if releaseName == "" {
		releaseName = instance.Spec.DeploySpec.Helm.Entity + "-release"
	}

	chartName := instance.Spec.DeploySpec.Helm.RepoName + "/" + instance.Spec.DeploySpec.Helm.Entity
	chartSpec := &helmclient.ChartSpec{
		ReleaseName: releaseName,
		ChartName:   chartName,
		Namespace:   instance.Spec.DeploySpec.Namespace,
	}

	if instance.Spec.DeploySpec.Helm.ValuesYaml != "" {
		chartSpec.ValuesYaml = instance.Spec.DeploySpec.Helm.ValuesYaml
	}

	err := r.HelmClient.AddOrUpdateChartRepo(chartRepo)
	if err != nil {
		r.Log.Info(fmt.Sprintf("Add or Update Chart Repo err #%v ", err))
		return err
	}
	err = r.HelmClient.InstallOrUpgradeChart(ctx, chartSpec)
	if err != nil {
		r.Log.Info(fmt.Sprintf("Install or upgrade err #%v ", err))
		return err
	}
	return err
}

func (r *BenchmarkOperatorReconciler) uninstallHelm(instance *cpev1.BenchmarkOperator) error {
	r.Log.Info(fmt.Sprintf("Uninstall helm %s/%s ", instance.Spec.DeploySpec.Helm.RepoName, instance.Spec.DeploySpec.Helm.Entity))
	chartName := instance.Spec.DeploySpec.Helm.RepoName + "/" + instance.Spec.DeploySpec.Helm.Entity
	releaseName := instance.Spec.DeploySpec.Helm.Release
	if releaseName == "" {
		releaseName = instance.Spec.DeploySpec.Helm.Entity + "-release"
	}

	chartSpec := &helmclient.ChartSpec{
		ReleaseName: releaseName,
		ChartName:   chartName,
		Namespace:   instance.Spec.DeploySpec.Namespace,
	}

	err := r.HelmClient.UninstallRelease(chartSpec)
	if err != nil {
		r.Log.Info(fmt.Sprintf("Uninstall err #%v ", err))
		return err
	}
	return nil
}

func (r *BenchmarkOperatorReconciler) installYAML(instance *cpev1.BenchmarkOperator) error {

	host := instance.Spec.DeploySpec.YAML.Host
	pathList := instance.Spec.DeploySpec.YAML.Paths
	var any_err error

	for _, path := range pathList {
		yamlURL := host + path
		r.Log.Info(fmt.Sprintf("Install yaml %s ", yamlURL))
		_, err := CreateFromURL(r.DC, r.DYN, yamlURL)
		if err != nil {
			r.Log.Info(fmt.Sprintf("Path %s err #%v ", yamlURL, err))
			any_err = err
			continue
		}
	}
	return any_err
}

func (r *BenchmarkOperatorReconciler) uninstallYAML(instance *cpev1.BenchmarkOperator) error {

	host := instance.Spec.DeploySpec.YAML.Host
	pathList := instance.Spec.DeploySpec.YAML.Paths
	var any_err error

	for _, path := range pathList {
		yamlURL := host + path
		r.Log.Info(fmt.Sprintf("Uninstall yaml %s ", yamlURL))
		err := DeleteFromURL(r.DC, r.DYN, yamlURL)
		if err != nil {
			r.Log.Info(fmt.Sprintf("Path %s err #%v ", yamlURL, err))
			any_err = err
			continue
		}
	}
	return any_err
}

// SetupWithManager sets up the controller with the Manager.
func (r *BenchmarkOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cpev1.BenchmarkOperator{}).
		Complete(r)
}

func (r *BenchmarkOperatorReconciler) removeRoleBinding(instance *cpev1.BenchmarkOperator) {
	roleName := instance.ObjectMeta.Name + "-cpe-cluster-role"
	bindName := instance.ObjectMeta.Name + "-cpe-cluster-role-binding"
	r.Clientset.RbacV1().ClusterRoles().Delete(context.Background(), roleName, metav1.DeleteOptions{})
	r.Clientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), bindName, metav1.DeleteOptions{})
}

func (r *BenchmarkOperatorReconciler) finalizeBenchmarkOperator(reqLogger logr.Logger, instance *cpev1.BenchmarkOperator) error {

	if !reflect.DeepEqual(cpev1.HelmSpec{}, instance.Spec.DeploySpec.Helm) {
		r.uninstallHelm(instance)
		r.removeRoleBinding(instance)
	} else if !reflect.DeepEqual(cpev1.YAMLSpec{}, instance.Spec.DeploySpec.YAML) {
		r.uninstallYAML(instance)
		r.removeRoleBinding(instance)
	} else {
		reqLogger.Info("No change to installation")
	}
	reqLogger.Info(fmt.Sprintf("Finalized %s", instance.ObjectMeta.Name))
	return nil
}

func getOperatorNamespace() string {
	key := "OPERATOR_NAMESPACE"
	val, found := os.LookupEnv(key)
	if !found {
		return defaultOperatorNamespace
	}
	return val
}

func (r *BenchmarkOperatorReconciler) DeployNoneOperator() {
	noneOperator := &cpev1.BenchmarkOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "none",
			Namespace: operatorNamespace,
		},
		Spec: cpev1.BenchmarkOperatorSpec{
			APIVersion: "batch/v1",
			Kind:       "Job",
			DeploySpec: cpev1.DeploymentSpec{},
		},
	}
	err := r.Create(context.TODO(), noneOperator)
	if err != nil {
		r.Log.Info(fmt.Sprintf("Cannot deploy none operator: %v", err))
	}
}
