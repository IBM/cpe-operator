/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"text/template"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/restmapper"
	cache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var itrHandler *IterationHandler = &IterationHandler{}

const (
	BENCHMARK_LABEL        = "cpe-benchmark"
	MAX_LABEL_LEN          = 60
	NODESELECT_ITR_NAME    = "profile"
	NODESELECT_ITR_DEFAULT = "default"
	INIT_BUILD_NAME        = "init"
	BUILD_KEY              = "build"
	REPETITION_KEY         = "repno"
	JOBHASH_KEY            = "cpe-jobhash"
	HASH_DELIMIT           = "-cpeh-"
	INVALID_REGEX          = "[^A-Za-z0-9]"
)

func GetInformerFromGVK(dc *discovery.DiscoveryClient, dyn dynamic.Interface, gvk schema.GroupVersionKind) (cache.SharedIndexInformer, dynamicinformer.DynamicSharedInformerFactory) {

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dyn, 0, v1.NamespaceAll, nil)

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	mapping, _ := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)

	informer := factory.ForResource(mapping.Resource)
	s := informer.Informer()
	return s, factory
}

func getResourceInterface(dc *discovery.DiscoveryClient, dyn dynamic.Interface, gvk *schema.GroupVersionKind, ns string) dynamic.ResourceInterface {

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	mapping, _ := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)

	var dr dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		dr = dyn.Resource(mapping.Resource).Namespace(ns)
	} else {
		dr = dyn.Resource(mapping.Resource)
	}

	return dr
}

func getInfoFromURL(dc *discovery.DiscoveryClient, dyn dynamic.Interface, yamlURL string) (*unstructured.Unstructured, dynamic.ResourceInterface, error) {
	// 1. get yaml body from url
	resp, err := http.Get(yamlURL)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, nil, err
	}

	// 2. get GVK
	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := dec.Decode(bodyBytes, nil, obj)

	if err != nil {
		return nil, nil, err
	}
	dr := getResourceInterface(dc, dyn, gvk, obj.GetNamespace())

	return obj, dr, nil
}

func CreateFromURL(dc *discovery.DiscoveryClient, dyn dynamic.Interface, yamlURL string) (*unstructured.Unstructured, error) {
	obj, dr, err := getInfoFromURL(dc, dyn, yamlURL)
	if err != nil {
		return nil, err
	}

	instance, err := dr.Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
	if err != nil { // create if not exists
		instance, err = dr.Create(context.TODO(), obj, metav1.CreateOptions{})
	}

	return instance, err
}

func DeleteFromURL(dc *discovery.DiscoveryClient, dyn dynamic.Interface, yamlURL string) error {
	obj, dr, err := getInfoFromURL(dc, dyn, yamlURL)
	if dr == nil {
		return err
	}

	err = dr.Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})

	return err
}

func getValidValue(val string) string {
	invalidReg, _ := regexp.Compile(INVALID_REGEX)
	valSize := len(val)
	firstChar := invalidReg.ReplaceAllString(val[0:1], "")
	restChars := invalidReg.ReplaceAllString(val[1:valSize], "-")
	val = firstChar + restChars

	if len(val) > MAX_LABEL_LEN {
		val = val[len(val)-MAX_LABEL_LEN:]
	}
	return val
}

func getIterationPairStr(key string, val string) string {
	return fmt.Sprintf(";%s=%s", key, val)
}

func getSubfixFromIterationLabel(iterationLabel map[string]string) string {
	subfix := ""
	if len(iterationLabel) == 0 {
		return ""
	}
	keys := make([]string, 0, len(iterationLabel))
	for key := range iterationLabel {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		subfix += getIterationPairStr(key, iterationLabel[key])
	}
	return subfix
}

// func getJobName(benchmark *cpev1.Benchmark, iterationLabel map[string]string, build string, repetition int) string {
// 	return fmt.Sprintf("%s%s-bc-%s-rp-%d", benchmark.ObjectMeta.Name, getSubfixFromIterationLabel(iterationLabel), build, repetition)
// }

func getJobHash(iterationLabel map[string]string, build string, repetition int) string {
	fullKey := fmt.Sprintf("%s-bc-%s-rp-%d", getSubfixFromIterationLabel(iterationLabel), build, repetition)
	h := fnv.New32a()
	h.Write([]byte(fullKey))
	return fmt.Sprintf("%d", h.Sum32())
}

func getJobNameFromHash(benchmarkName string, jobHash string) string {
	return benchmarkName + HASH_DELIMIT + jobHash
}

func getJobName(benchmark *cpev1.Benchmark, iterationLabel map[string]string, build string, repetition int) string {
	jobHash := getJobHash(iterationLabel, build, repetition)
	return getJobNameFromHash(benchmark.GetName(), jobHash)
}

func getOrderedKey(hashItem map[string]string) []string {
	var orderedKeys []string
	for k, _ := range hashItem {
		orderedKeys = append(orderedKeys, k)
	}
	sort.Strings(orderedKeys)
	return orderedKeys
}

func GetDetailFromJobName(jobName string, benchmark *cpev1.Benchmark) (benchmarkName string, iterationMap map[string]string, configurationMap map[string]string, repetition string, buildID string, iterationID string, configurationID string) {

	iterations := benchmark.Spec.IterationSpec.Iteration
	var configurations []cpev1.IterationItem
	if benchmark.Spec.IterationSpec.NodeSelection != nil {
		nodeSelectionItr := NodeSelectionSpecToIteration(benchmark.Spec.IterationSpec.NodeSelection)
		configurations = append(benchmark.Spec.IterationSpec.Configuration, nodeSelectionItr)
	} else {
		configurations = benchmark.Spec.IterationSpec.Configuration
	}

	iterationMap = make(map[string]string)
	configurationMap = make(map[string]string)

	var iterationKeys map[string]interface{}
	var configurationKeys map[string]interface{}
	iterationKeys = make(map[string]interface{})
	configurationKeys = make(map[string]interface{})

	for _, item := range iterations {
		iterationKeys[item.Name] = nil
	}
	for _, item := range configurations {
		configurationKeys[item.Name] = nil
	}

	// init value
	buildID = ""
	iterationID = ""
	configurationID = ""
	repetition = "0"

	if strings.Contains(jobName, HASH_DELIMIT) {
		var orderedKeys []string
		if len(benchmark.Status.Hash) > 0 {
			orderedKeys = getOrderedKey(benchmark.Status.Hash[0].Iteration)
		}
		splited := strings.Split(jobName, HASH_DELIMIT)
		targetHash := splited[len(splited)-1]
		for _, hashItem := range benchmark.Status.Hash {
			if hashItem.Hash == targetHash {
				buildID = hashItem.Build
				for _, key := range orderedKeys {
					val := hashItem.Iteration[key]
					if _, ok := iterationKeys[key]; ok {
						iterationMap[key] = val
						iterationID += getIterationPairStr(key, val)
					} else if _, ok := configurationKeys[key]; ok {
						configurationMap[key] = val
						configurationID += getIterationPairStr(key, val)
					}
				}
				repetition = hashItem.Repetition
				break
			}
		}
	}
	if len(iterationID) > 0 {
		iterationID = iterationID[1:]
	}
	if len(configurationID) > 0 {
		configurationID = configurationID[1:]
	}
	return benchmark.GetName(), iterationMap, configurationMap, repetition, buildID, iterationID, configurationID
}

func GetSimpleJobGVK(benchmarkOperator *cpev1.BenchmarkOperator) schema.GroupVersionKind {
	apiVersion := benchmarkOperator.Spec.APIVersion
	kind := benchmarkOperator.Spec.Kind
	benchmarkObj := make(map[string]interface{})
	benchmarkObj["apiVersion"] = apiVersion
	benchmarkObj["kind"] = kind
	extBenchmark := &unstructured.Unstructured{
		Object: benchmarkObj,
	}
	gvk := extBenchmark.GroupVersionKind()
	return gvk
}

func GetIteratedValues(benchmark *cpev1.Benchmark) (firstLabel map[string]string, iterationLabels []map[string]string, builds []string, maxRepetition int) {
	// iterations
	iterations := GetCombinedIterations(benchmark)
	if len(iterations) > 0 {
		iterationLabels = itrHandler.GetAllCombination(iterations)
		firstLabel, iterationLabels = iterationLabels[0], iterationLabels[1:]
	} else {
		firstLabel = make(map[string]string)
		iterationLabels = []map[string]string{}
	}

	// builds
	buildSize := len(benchmark.Status.TrackedBuilds)
	if buildSize > 0 {
		builds = benchmark.Status.TrackedBuilds
	} else {
		builds = []string{INIT_BUILD_NAME}
	}

	// repetition
	maxRepetition = benchmark.Spec.Repetition
	if maxRepetition <= 0 {
		maxRepetition = 1
	}
	return firstLabel, iterationLabels, builds, maxRepetition
}

func patchBenchmarkStatus(client client.Client, benchmark *cpev1.Benchmark, jobHash string, iterationLabel map[string]string, build string, repInString string) error {
	if checkHashExist(benchmark, jobHash) {
		return nil
	}
	newHashItem := cpev1.IterationHash{
		Hash:       jobHash,
		Build:      build,
		Iteration:  iterationLabel,
		Repetition: repInString,
	}
	benchmark.Status.Hash = append(benchmark.Status.Hash, newHashItem)
	benchmark.Status.JobCompleted = GetJobCompletedStatus(benchmark)
	err := client.Status().Update(context.Background(), benchmark)
	return err
}

func GetJobCompletedStatus(benchmark *cpev1.Benchmark) string {
	completedJob := 0
	for _, result := range benchmark.Status.Results {
		completedJob = completedJob + len(result.Items)
	}
	return fmt.Sprintf("%d/%d", completedJob, len(benchmark.Status.Hash))
}

func GetExpandLabel(iterationLabel map[string]string) map[string]interface{} {
	expandLabel := make(map[string]interface{})
	for key, val := range iterationLabel {
		valSplits := strings.Split(val, ";")
		if len(valSplits) > 1 {
			expandLabel[key] = valSplits
		} else {
			expandLabel[key] = val
		}
	}
	return expandLabel
}

func GetBenchmarkWithIteration(client client.Client, ns string, benchmark *cpev1.Benchmark, benchmarkObj map[string]interface{}, iterationLabel map[string]string, build string, repetition int) (*unstructured.Unstructured, error) {

	labels := map[string]interface{}{BENCHMARK_LABEL: benchmark.ObjectMeta.Name}
	for key, value := range iterationLabel {
		labels[key] = getValidValue(value)
	}
	labels[BUILD_KEY] = getValidValue(build)
	repInString := fmt.Sprintf("%d", repetition)
	labels[REPETITION_KEY] = repInString

	// get hash
	jobHash := getJobHash(iterationLabel, build, repetition)
	jobName := getJobNameFromHash(benchmark.GetName(), jobHash)
	patchBenchmarkStatus(client, benchmark, jobHash, iterationLabel, build, repInString)

	labels[JOBHASH_KEY] = jobHash

	benchmarkObj["metadata"] = map[string]interface{}{"name": jobName, "namespace": ns, "labels": labels}

	// generate job spec
	tmpl, err := template.New("").Parse(benchmark.Spec.Spec)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	expandLabel := GetExpandLabel(iterationLabel)
	err = tmpl.Execute(&buffer, expandLabel)
	if err != nil {
		return nil, err
	}
	executedSpec := buffer.String()

	var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	decUnstructured.Decode([]byte(executedSpec), nil, obj)

	specObject := obj.Object

	if _, ok := iterationLabel[NODESELECT_ITR_NAME]; ok {
		if iterationLabel[NODESELECT_ITR_NAME] != NODESELECT_ITR_DEFAULT {
			nodeSelectionItr := NodeSelectionSpecToIteration(benchmark.Spec.IterationSpec.NodeSelection)
			specObject = itrHandler.UpdateValue(specObject, nodeSelectionItr.Location, iterationLabel[NODESELECT_ITR_NAME])
		}
	}
	// add selector label
	nodeSelectionSpec := benchmark.Spec.IterationSpec.NodeSelection
	if nodeSelectionSpec != nil {
		labelMap, _ := metav1.LabelSelectorAsMap(nodeSelectionSpec.TargetSelector)
		for selectorLabelName, selectorLabelValue := range labelMap {
			selectorLabelLocation := nodeSelectionSpec.Location + "." + "(" + selectorLabelName + ")"
			specObject = itrHandler.UpdateValue(specObject, selectorLabelLocation, selectorLabelValue)
		}
	}

	benchmarkObj["spec"] = specObject
	extBenchmark := &unstructured.Unstructured{
		Object: benchmarkObj,
	}
	return extBenchmark, nil
}

func CheckIfJobDone(benchmark *cpev1.Benchmark, jobName string) bool {
	for _, result := range benchmark.Status.Results {
		for _, item := range result.Items {
			if item.JobName == jobName {
				return true
			}
		}
	}
	return false
}

func CreateIfNotExists(dr dynamic.ResourceInterface, benchmark *cpev1.Benchmark, unstructuredInstance *unstructured.Unstructured, adaptor OperatorAdaptor, tunedHandler *TunedHandler, nodeTunedOptimizer *BaysesOptimizer) (error, bool) {
	if CheckIfJobDone(benchmark, unstructuredInstance.GetName()) {
		return nil, false
	}

	existJob, err := dr.Get(context.TODO(), unstructuredInstance.GetName(), metav1.GetOptions{})

	tunedValue := NODESELECT_ITR_DEFAULT
	autoTuned := false
	completed := true
	nodeSelectionSpec := benchmark.Spec.IterationSpec.NodeSelection

	if err == nil {
		// found
		completed = adaptor.CheckComplete(existJob.Object)
	}

	if completed {
		if nodeSelectionSpec != nil {
			tunedValue = getTunedValue(nodeSelectionSpec, unstructuredInstance)
			if tunedValue == RESERVED_AUTOTUNED_PROFILE_NAME && tunedHandler != nil {
				sampledProfileMaps, ok := <-nodeTunedOptimizer.SampleQueue
				if !ok {
					if nodeTunedOptimizer.FinalizedApplied {
						return fmt.Errorf("no more sample"), true
					} else {
						sampledProfileMaps = nodeTunedOptimizer.FinalizedTunedProfile
						nodeTunedOptimizer.SetFinalizedApplied()
					}
				}
				tunedHandler.DeleteAutoTunedProfile()
				err = tunedHandler.CreateAutoTunedProfile(sampledProfileMaps)
				if err != nil {
					return err, true
				}

				// deleted previous tuned job
				dr.Delete(context.TODO(), unstructuredInstance.GetName(), metav1.DeleteOptions{})
				autoTuned = true
			}
		}
	}

	if err != nil || autoTuned { // create if not exists or autotuned deleted
		// handler tuned profile
		if tunedValue != NODESELECT_ITR_DEFAULT && tunedHandler != nil {
			tunedHandler.ApplyProfile(nodeSelectionSpec.TargetSelector, tunedValue)
		}
		// create
		_, err = dr.Create(context.TODO(), unstructuredInstance, metav1.CreateOptions{})
		return err, true
	} else if completed {
		return err, false
	}
	return err, true
}

func checkHashExist(benchmark *cpev1.Benchmark, jobHash string) bool {
	for _, hashItem := range benchmark.Status.Hash {
		if hashItem.Hash == jobHash {
			return true
		}
	}
	return false
}

func JobListChanged(benchmark *cpev1.Benchmark) bool {
	firstLabel, iterationLabels, builds, maxRepetition := GetIteratedValues(benchmark)
	noHash := true
	repetition := 0
	for {
		if repetition >= maxRepetition {
			break
		}
		for _, build := range builds {
			noHash = true
			jobHash := getJobHash(firstLabel, build, repetition)
			if !checkHashExist(benchmark, jobHash) {
				return true
			}
			for _, iterationLabel := range iterationLabels {
				jobHash = getJobHash(iterationLabel, build, repetition)
				if !checkHashExist(benchmark, jobHash) {
					return true
				}
			}
		}
		repetition = repetition + 1
	}
	if len(benchmark.Status.Hash) == 0 && noHash {
		return false
	}
	return true
}

func NewBenchmarkObject(benchmarkOperator *cpev1.BenchmarkOperator) map[string]interface{} {
	apiVersion := benchmarkOperator.Spec.APIVersion
	kind := benchmarkOperator.Spec.Kind

	benchmarkObj := make(map[string]interface{})
	benchmarkObj["apiVersion"] = apiVersion
	benchmarkObj["kind"] = kind
	return benchmarkObj
}

func CreateFromOperator(jtm *JobTrackManager, client client.Client, dc *discovery.DiscoveryClient, dyn dynamic.Interface, benchmark *cpev1.Benchmark, benchmarkOperator *cpev1.BenchmarkOperator, reqLogger logr.Logger, adaptor OperatorAdaptor, tunedHandler *TunedHandler) error {
	gvk := GetSimpleJobGVK(benchmarkOperator)

	if jtm.IsExist(gvk, benchmark.GetName()) {
		reqLogger.Info(fmt.Sprintf("Benchmark %s has already registered", benchmark.GetName()))
		return nil
	}

	dr := getResourceInterface(dc, dyn, &gvk, benchmark.Namespace)

	if dr == nil {
		reqLogger.Info(fmt.Sprintf("Benchmark %s cannot getResourceInterface", benchmark.GetName()))
		return nil
	}

	firstLabel, iterationLabels, builds, maxRepetition := GetIteratedValues(benchmark)

	var waitingJob []*unstructured.Unstructured
	jobOptMap := make(map[string]*BaysesOptimizer)
	isNew := false
	var err error
	repetition := 0
	reqLogger.Info(fmt.Sprintf("Max Repetition: %d", maxRepetition))
	for {

		if repetition >= maxRepetition {
			break
		}

		for _, build := range builds {
			// for firstLabel (iteration0 or nolabel)
			firstBenchmarkObj := NewBenchmarkObject(benchmarkOperator)
			extBenchmark, err := GetBenchmarkWithIteration(client, benchmark.Namespace, benchmark, firstBenchmarkObj, firstLabel, build, repetition)
			if err != nil {
				reqLogger.Info(fmt.Sprintf("Failed to GetBenchmarkWithIteration: %v)", err))
			}
			nodeTunedOptimizer := NewBayesOptimizer(benchmark.Spec.IterationSpec.Minimize)
			jobName := extBenchmark.GetName()
			jobOptMap[jobName] = nodeTunedOptimizer

			nodeSelectionSpec := benchmark.Spec.IterationSpec.NodeSelection
			if nodeSelectionSpec != nil {
				tunedValue := getTunedValue(nodeSelectionSpec, extBenchmark)
				if tunedValue == RESERVED_AUTOTUNED_PROFILE_NAME && tunedHandler != nil {
					// activate auto-tuning
					go nodeTunedOptimizer.AutoTune()
				} else {
					nodeTunedOptimizer.SetFinalizedApplied()
				}
				reqLogger.Info(fmt.Sprintf("Set JobOptimizerMap %s - %s, %v)", jobName, tunedValue, extBenchmark))
			} else {
				nodeTunedOptimizer.SetFinalizedApplied()
			}

			if err == nil && !isNew {
				err, isNew = CreateIfNotExists(dr, benchmark, extBenchmark, adaptor, tunedHandler, nodeTunedOptimizer)
				reqLogger.Info(fmt.Sprintf("Try creating %s (first label)", extBenchmark.GetName()))
				if err != nil {
					reqLogger.Info(fmt.Sprintf("Failed to create benchmark %s: %v)", benchmark.Name, err))
				}
			} else {
				_, existErr := dr.Get(context.TODO(), extBenchmark.GetName(), metav1.GetOptions{})
				if existErr != nil {
					waitingJob = append(waitingJob, extBenchmark.DeepCopy())
				}
			}
			// for the rest iteration
			for _, iterationLabel := range iterationLabels {
				benchmarkObj := NewBenchmarkObject(benchmarkOperator)
				waitExtBenchmark, err := GetBenchmarkWithIteration(client, benchmark.Namespace, benchmark, benchmarkObj, iterationLabel, build, repetition)
				if err != nil {
					reqLogger.Info(fmt.Sprintf("Failed to GetBenchmarkWithIteration: %v)", err))
					continue
				}
				nodeTunedOptimizer := NewBayesOptimizer(benchmark.Spec.IterationSpec.Minimize)
				jobName = waitExtBenchmark.GetName()
				jobOptMap[jobName] = nodeTunedOptimizer

				if nodeSelectionSpec != nil {
					tunedValue := getTunedValue(nodeSelectionSpec, waitExtBenchmark)

					if tunedValue == RESERVED_AUTOTUNED_PROFILE_NAME {
						// activate auto-tuning
						go nodeTunedOptimizer.AutoTune()
					} else {
						nodeTunedOptimizer.SetFinalizedApplied()
					}
					reqLogger.Info(fmt.Sprintf("Set JobOptimizerMap %s - %s, %v)", jobName, tunedValue, waitExtBenchmark))
				} else {
					nodeTunedOptimizer.SetFinalizedApplied()
				}

				if benchmark.Spec.IterationSpec.Sequential || benchmark.Spec.IterationSpec.NodeSelection != nil { // must be sequential if node selection is set
					// to create at least one new job
					if err == nil && !isNew {
						err, isNew = CreateIfNotExists(dr, benchmark, waitExtBenchmark, adaptor, tunedHandler, nodeTunedOptimizer)
						reqLogger.Info(fmt.Sprintf("Try creating %s", waitExtBenchmark.GetName()))
						if err != nil {
							reqLogger.Info(fmt.Sprintf("Failed to create benchmark %s: %v)", benchmark.Name, err))
						}
					} else {
						_, existErr := dr.Get(context.TODO(), waitExtBenchmark.GetName(), metav1.GetOptions{})
						if existErr != nil {
							waitingJob = append(waitingJob, waitExtBenchmark.DeepCopy())
						}
					}
				} else {
					err, _ = CreateIfNotExists(dr, benchmark, waitExtBenchmark, adaptor, tunedHandler, nodeTunedOptimizer)
					if err != nil {
						reqLogger.Info(fmt.Sprintf("Failed to create benchmark %s: %v)", benchmark.Name, err))
					}
				}
			}
		}
		repetition = repetition + 1

	}

	if err != nil {
		reqLogger.Info(fmt.Sprintf("Cannot create #%v: %s", err, benchmark.GetName()))
		return err
	}

	jtm.NewTracker(gvk, benchmark.GetName(), waitingJob, dr, adaptor, jobOptMap)

	return nil
}

func DeleteFromOperator(dc *discovery.DiscoveryClient, dyn dynamic.Interface, benchmark *cpev1.Benchmark, benchmarkOperator *cpev1.BenchmarkOperator) error {

	apiVersion := benchmarkOperator.Spec.APIVersion
	kind := benchmarkOperator.Spec.Kind
	ns := benchmark.ObjectMeta.Namespace

	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	dr := getResourceInterface(dc, dyn, &gvk, ns)

	firstLabel, iterationLabels, builds, maxRepetition := GetIteratedValues(benchmark)
	repetition := 0
	var err error
	for {

		if repetition >= maxRepetition {
			break
		}

		for _, build := range builds {
			// for firstLabel (iteration0 or nolabel)
			jobName := getJobName(benchmark, firstLabel, build, repetition)
			err = dr.Delete(context.TODO(), jobName, metav1.DeleteOptions{})
			// for the rest iteration
			for _, iterationLabel := range iterationLabels {
				jobName = getJobName(benchmark, iterationLabel, build, repetition)
				err = dr.Delete(context.TODO(), jobName, metav1.DeleteOptions{})
			}
		}
		repetition = repetition + 1

	}
	return err
}

func GetCombinedIterations(benchmark *cpev1.Benchmark) []cpev1.IterationItem {
	iterations := append(benchmark.Spec.IterationSpec.Iteration, benchmark.Spec.IterationSpec.Configuration...)

	if benchmark.Spec.IterationSpec.NodeSelection != nil {
		nodeSelectionItr := NodeSelectionSpecToIteration(benchmark.Spec.IterationSpec.NodeSelection)
		iterations = append(iterations, nodeSelectionItr)
	}
	return iterations
}

// NodeSelectionSpec
func NodeSelectionSpecToIteration(spec *cpev1.NodeSelectionSpec) cpev1.IterationItem {
	location := spec.Location + "." + NODESELECT_ITR_NAME
	itr := cpev1.IterationItem{
		Name:     NODESELECT_ITR_NAME,
		Location: location,
		Values:   spec.TunedValues,
	}
	return itr
}

func getTunedValue(spec *cpev1.NodeSelectionSpec, job *unstructured.Unstructured) string {
	location := spec.Location + "." + NODESELECT_ITR_NAME
	tunedValue := itrHandler.GetValue(job.Object["spec"].(map[string]interface{}), location)
	if tunedValue == nil {
		return NODESELECT_ITR_DEFAULT
	}
	return tunedValue[0]
}
