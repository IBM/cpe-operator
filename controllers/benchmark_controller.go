/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

///////////////////////////////////////////////////////////////////////////
//
// benchmark_controller.go
//
// - Reconcile Loop
//   1. create benchmark job resource manifests from the defined benchmark operator
//   for each defined iteration (application arguments and node tuning)
//   and for each build
//   2. create job tracker if not exists for the target job resource
//   3. deploy only the first undeployed-yet manifests
//      put the rest in waiting to the job tracker
//
////////////////////////////////////////////////////////////////////////////

package controllers

import (
	"context"
	"fmt"

	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
)

// BenchmarkReconciler reconciles a Benchmark object
type BenchmarkReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	DC     *discovery.DiscoveryClient
	DYN    dynamic.Interface
	JTM    *JobTrackManager
	*TunedHandler
}

//+kubebuilder:rbac:groups=cpe.cogadvisor.io,resources=benchmarks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cpe.cogadvisor.io,resources=benchmarks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cpe.cogadvisor.io,resources=benchmarks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Benchmark object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile

const ReconcileTime = 30 * time.Minute

const benchmarkFinalizer = "finalizers.benchmark.cpe.cogadvisor.io"

func (r *BenchmarkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("benchmark", req.NamespacedName)

	r.Log.Info(fmt.Sprintf("Benchmark Request #%v ", req.NamespacedName))

	instance := &cpev1.Benchmark{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.Info(fmt.Sprintf("Cannot get #%v ", err))
			return ctrl.Result{}, nil
		}
		r.Log.Info(fmt.Sprintf("Cannot get #%v ", err))
		// Error reading the object - requeue the request.
		return ctrl.Result{RequeueAfter: ReconcileTime}, nil
	}

	is_deleted := instance.GetDeletionTimestamp() != nil
	if is_deleted {
		if controllerutil.ContainsFinalizer(instance, benchmarkFinalizer) {
			if err := r.finalizeBenchmark(reqLogger, instance); err != nil {
				return ctrl.Result{}, nil
			}

			controllerutil.RemoveFinalizer(instance, benchmarkFinalizer)
			err := r.Client.Update(ctx, instance)
			if err != nil {
				return ctrl.Result{}, nil
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(instance, benchmarkFinalizer) {
		controllerutil.AddFinalizer(instance, benchmarkFinalizer)
		err = r.Client.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, nil
		}
	} else {
		r.Log.Info(fmt.Sprintf("Creating #%s ", instance.ObjectMeta.Name))
		operatorName := instance.Spec.Operator.Name
		operatorNS := instance.Spec.Operator.Namespace
		if operatorNS == "" {
			operatorNS = "default"
		}
		operator := &cpev1.BenchmarkOperator{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: operatorName, Namespace: operatorNS}, operator)
		if err != nil {
			r.Log.Info(fmt.Sprintf("Cannot get #%v ", err))
			return ctrl.Result{}, nil
		}
		var adaptor OperatorAdaptor
		if _, adaptorExists := OperatorAdaptorMap[operator.Spec.Adaptor]; adaptorExists {
			adaptor = OperatorAdaptorMap[operator.Spec.Adaptor]
		} else {
			adaptor = OperatorAdaptorMap["default"]
		}
		r.Log.Info(fmt.Sprintf("Operator #%s ", operator.ObjectMeta.Name))

		CreateFromOperator(r.JTM, r.Client, r.DC, r.DYN, instance, operator, r.Log, adaptor, r.TunedHandler)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BenchmarkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cpev1.Benchmark{}).
		Complete(r)
}

func (r *BenchmarkReconciler) finalizeBenchmark(reqLogger logr.Logger, instance *cpev1.Benchmark) error {
	ctx := context.Background()

	// get operator
	operatorName := instance.Spec.Operator.Name
	operatorNS := instance.Spec.Operator.Namespace

	if operatorNS == "" {
		operatorNS = "default"
	}
	operator := &cpev1.BenchmarkOperator{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: operatorName, Namespace: operatorNS}, operator)
	if err != nil {
		reqLogger.Info(fmt.Sprintf("Cannot get #%v ", err))
		return nil
	}
	reqLogger.Info(fmt.Sprintf("Operator #%s ", operator.ObjectMeta.Name))

	// unsubscribe job from operator
	jobGVK := GetSimpleJobGVK(operator)
	r.JTM.DeleteTracker(jobGVK, instance.ObjectMeta.Name)

	// delete from operator
	err = DeleteFromOperator(r.DC, r.DYN, instance, operator)
	if err != nil {
		reqLogger.Info(fmt.Sprintf("Cannot delete #%v ", err))
	}

	reqLogger.Info(fmt.Sprintf("Finalized %s", instance.ObjectMeta.Name))
	return nil
}
