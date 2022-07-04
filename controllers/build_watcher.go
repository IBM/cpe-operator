/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

///////////////////////////////////////////////////////////////////////////
//
// build_watcher.go
//
// - Watch Build (builds.v1.build.openshift.io)
// - When Build is completed,
//   put build's namespace-name to the status list of referring benchmarks
//   (further handled by benchmark controller)
//
////////////////////////////////////////////////////////////////////////////

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
)

const NO_BUILD_RESOURCE_ERROR = "the server could not find the requested resource"
const BUILD_RESOURCE = "builds.v1.build.openshift.io"

type BuildWatcher struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	DC         *discovery.DiscoveryClient
	DYN        dynamic.Interface
	BuildQueue chan *unstructured.Unstructured
	Quit       chan struct{}
}

func (r *BuildWatcher) getInformer(DYN dynamic.Interface) (cache.SharedIndexInformer, dynamicinformer.DynamicSharedInformerFactory, error) {

	gvr, _ := schema.ParseResourceArg(BUILD_RESOURCE)
	_, err := r.DYN.Resource(*gvr).Namespace("default").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(DYN, 0, v1.NamespaceAll, nil)
	informer := factory.ForResource(*gvr)

	s := informer.Informer()
	return s, factory, nil
}

func (r *BuildWatcher) InitInformer() error {

	s, factory, err := r.getInformer(r.DYN)

	if err != nil || s == nil {
		if err.Error() == NO_BUILD_RESOURCE_ERROR {
			r.Log.Info(fmt.Sprintf("No Build Resource"))
			return err
		}
		r.Log.Info(fmt.Sprintf("BuildListError: %v", err))
		return err
	}

	handlers := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldInstance, instance interface{}) {
			build := instance.(*unstructured.Unstructured)
			oldBuild := oldInstance.(*unstructured.Unstructured)
			buildObject := build.Object
			oldObject := oldBuild.Object
			buildStatus := buildObject["status"].(map[string]interface{})
			oldStatus := oldObject["status"].(map[string]interface{})

			// status change to "Complete" --> add to process queue
			if oldStatus["phase"] != "Complete" && buildStatus["phase"] == "Complete" {
				r.BuildQueue <- build
			}
		},
	}

	s.AddEventHandler(handlers)
	factory.Start(r.Quit)
	r.Log.Info(fmt.Sprintf("Successfully Init BuildWatcher"))
	return nil
}

func (r *BuildWatcher) Run() {
	wait.Until(r.ProcessBuildQueue, 0, r.Quit)
}

func (r *BuildWatcher) ProcessBuildQueue() {
	build := <-r.BuildQueue
	buildObject := build.Object

	buildStatus := buildObject["status"].(map[string]interface{})
	buildConfig := buildStatus["config"].(map[string]interface{})

	buildKind := buildConfig["kind"].(string)
	buildName := buildConfig["name"].(string)
	buildNamespace := buildConfig["namespace"].(string)

	buildMeta := buildObject["metadata"].(map[string]interface{})
	buildValue := buildMeta["namespace"].(string) + "-" + buildMeta["name"].(string)

	_ = r.Log.WithValues("build", buildName)

	r.Log.Info(fmt.Sprintf("Build #%s, %s, %s", buildKind, buildName, buildNamespace))

	benchmarkList := &cpev1.BenchmarkList{}
	r.Client.List(context.Background(), benchmarkList)

	update_benchmarks := []cpev1.Benchmark{}

	// find corresponding benchmarks
	for _, benchmark := range benchmarkList.Items {
		configSpecs := benchmark.Spec.BuildConfigs
		for _, configSpec := range configSpecs {
			configKind := configSpec.Kind

			if configKind == "" {
				configKind = "BuildConfig"
			}

			configNamespace := configSpec.Namespace
			if configNamespace == "" {
				configNamespace = "default"
			}

			r.Log.Info(fmt.Sprintf("Track #%s, %s, %s", configKind, configSpec.Name, configNamespace))

			if configKind == buildKind &&
				configSpec.Name == buildName &&
				configNamespace == buildNamespace {
				update_benchmarks = append(update_benchmarks, benchmark)
			}
		}
	}

	// update build list of the corresponding benchmarks
	// this update will be further watched by benchmark_controller
	for _, update_benchmark := range update_benchmarks {
		builds := update_benchmark.Status.TrackedBuilds
		builds = append(builds, buildValue)
		update_benchmark.Status.TrackedBuilds = builds

		err := r.Client.Status().Update(context.Background(), &update_benchmark)

		if err != nil {
			r.Log.Info(fmt.Sprintf("Cannot update #%v ", err))
		}
	}
}
