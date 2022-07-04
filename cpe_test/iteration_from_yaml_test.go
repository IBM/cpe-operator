/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */
// go test -v cpe_test/iteration_from_yaml_test.go

package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
	"github.com/IBM/cpe-operator/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const benchmarkOperatorFile = "../benchmarks/mpi_operator/cpe_v1_mpioperator_v1alpha2.yaml"
const benchmarkFile = "../benchmarks/mpi_operator/cpe_v1_gromacs.yaml"

func readYaml(filename string, t *testing.T) []byte {
	yamlBytes, err := ioutil.ReadFile(filename)
	assert.Equal(t, err, nil)

	obj := &unstructured.Unstructured{}

	// decode YAML into unstructured.Unstructured
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err = dec.Decode([]byte(yamlBytes), nil, obj)
	assert.Equal(t, err, nil)

	jsonBytes, err := json.Marshal(obj)
	assert.Equal(t, err, nil)
	return jsonBytes

}

func getBenchmark(filename string, t *testing.T) *cpev1.Benchmark {
	jsonBytes := readYaml(filename, t)
	var benchmark = cpev1.Benchmark{}
	err := json.Unmarshal(jsonBytes, &benchmark)
	assert.Equal(t, err, nil)
	return &benchmark
}

func getBenchmarkOperator(filename string, t *testing.T) *cpev1.BenchmarkOperator {
	jsonBytes := readYaml(filename, t)
	var benchmarkOperator = cpev1.BenchmarkOperator{}
	err := json.Unmarshal(jsonBytes, &benchmarkOperator)
	assert.Equal(t, err, nil)
	return &benchmarkOperator

}

var iterationHandlerYAML *controllers.IterationHandler = &controllers.IterationHandler{}

func TestGetInitCombinationFromYAML(t *testing.T) {
	benchmark := getBenchmark(benchmarkFile, t)

	iterations := controllers.GetCombinedIterations(benchmark)
	benchmarkSpecStr := benchmark.Spec.Spec
	var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	object := &unstructured.Unstructured{}
	decUnstructured.Decode([]byte(benchmarkSpecStr), nil, object)

	initCombination := iterationHandlerYAML.GetInitCombination(object.Object, iterations)
	fmt.Printf("Init Combination: %v\n", initCombination)
}

func TestGetInitAndAllCombinationFromYAML(t *testing.T) {
	benchmark := getBenchmark(benchmarkFile, t)

	iterations := controllers.GetCombinedIterations(benchmark)
	benchmarkSpecStr := benchmark.Spec.Spec
	var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	object := &unstructured.Unstructured{}
	decUnstructured.Decode([]byte(benchmarkSpecStr), nil, object)

	combinations := iterationHandlerYAML.GetInitAndAllCombination(object.Object, iterations)
	fmt.Printf("Combinations: %v\n", combinations)

}

func GetJobResource(benchmark *cpev1.Benchmark) (obj *unstructured.Unstructured, firstLabel map[string]string, iterationLabels []map[string]string) {
	// iterations
	benchmarkSpecStr := benchmark.Spec.Spec
	var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj = &unstructured.Unstructured{}
	decUnstructured.Decode([]byte(benchmarkSpecStr), nil, obj)

	iterations := controllers.GetCombinedIterations(benchmark)

	if len(iterations) > 0 {
		iterationLabels = iterationHandlerYAML.GetInitAndAllCombination(obj.Object, iterations)
		firstLabel, iterationLabels = iterationLabels[0], iterationLabels[1:]
	} else {
		firstLabel = make(map[string]string)
		iterationLabels = []map[string]string{}
	}

	return obj, firstLabel, iterationLabels
}

func getBenchmarkWithIteration(ns string, benchmark *cpev1.Benchmark, benchmarkObj map[string]interface{}, iterationLabel map[string]string, build string, repetition int) *unstructured.Unstructured {
	// get hash
	jobName := "test"

	benchmarkObj["metadata"] = map[string]interface{}{"name": jobName, "namespace": ns}
	specObject := benchmarkObj["spec"].(map[string]interface{})
	for _, item := range benchmark.Spec.IterationSpec.Iteration {
		valueToSet := iterationLabel[item.Name]
		location := item.Location
		specObject = iterationHandlerYAML.UpdateValue(specObject, location, valueToSet)
	}
	for _, item := range benchmark.Spec.IterationSpec.Configuration {
		valueToSet := iterationLabel[item.Name]
		location := item.Location
		specObject = iterationHandlerYAML.UpdateValue(specObject, location, valueToSet)
	}
	if _, ok := iterationLabel[controllers.NODESELECT_ITR_NAME]; ok {
		if iterationLabel[controllers.NODESELECT_ITR_NAME] != controllers.NODESELECT_ITR_DEFAULT {
			nodeSelectionItr := controllers.NodeSelectionSpecToIteration(benchmark.Spec.IterationSpec.NodeSelection)
			specObject = iterationHandlerYAML.UpdateValue(specObject, nodeSelectionItr.Location, iterationLabel[controllers.NODESELECT_ITR_NAME])
		}
	}
	// add selector label
	nodeSelectionSpec := benchmark.Spec.IterationSpec.NodeSelection
	if nodeSelectionSpec != nil {
		labelMap, _ := metav1.LabelSelectorAsMap(nodeSelectionSpec.TargetSelector)
		for selectorLabelName, selectorLabelValue := range labelMap {
			selectorLabelLocation := nodeSelectionSpec.Location + "." + "(" + selectorLabelName + ")"
			specObject = iterationHandlerYAML.UpdateValue(specObject, selectorLabelLocation, selectorLabelValue)
		}
	}

	benchmarkObj["spec"] = specObject
	extBenchmark := &unstructured.Unstructured{
		Object: benchmarkObj,
	}
	return extBenchmark
}

func TestGeneratedBenchmark(t *testing.T) {
	benchmark := getBenchmark(benchmarkFile, t)
	benchmarkOperator := getBenchmarkOperator(benchmarkOperatorFile, t)

	obj, firstLabel, iterationLabels, builds, maxRepetition := controllers.GetInfoToIterateFromBenchmark(benchmark)

	repetition := 0
	for {

		if repetition >= maxRepetition {
			break
		}

		for _, build := range builds {
			// for firstLabel (iteration0 or nolabel)
			benchmarkObj := controllers.NewBenchmarkObject(benchmarkOperator, obj)
			job := getBenchmarkWithIteration(benchmark.Namespace, benchmark, benchmarkObj, firstLabel, build, repetition)
			fmt.Println(firstLabel)
			fmt.Println(job)
			// for the rest iteration
			for _, iterationLabel := range iterationLabels {
				benchmarkObj = controllers.NewBenchmarkObject(benchmarkOperator, obj)
				fmt.Println(iterationLabel)
				job = getBenchmarkWithIteration(benchmark.Namespace, benchmark, benchmarkObj, iterationLabel, build, repetition)
				fmt.Println(job)
			}
		}
		repetition = repetition + 1

	}
}
