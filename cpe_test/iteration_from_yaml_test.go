/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */
// go test -v cpe_test/iteration_from_yaml_test.go

package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
	"github.com/IBM/cpe-operator/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	benchmarkOperatorFile         = "./sample/benchmark_operator.yaml"
	benchmarkFile                 = "./sample/benchmark.yaml"
	plainBenchmarkFile            = "./sample/plain_benchmark.yaml"
	combinedBenchmarkFile         = "./sample/combined_benchmark.yaml"
	expectedJobFile               = "./sample/expected_job.yaml"
	expectedCombinedBenchmarkFile = "./sample/expected_combined_job.yaml"
)

func readYamlObj(filename string, t *testing.T) *unstructured.Unstructured {
	yamlBytes, err := ioutil.ReadFile(filename)
	assert.Equal(t, err, nil)
	obj := &unstructured.Unstructured{}

	// decode YAML into unstructured.Unstructured
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err = dec.Decode([]byte(yamlBytes), nil, obj)
	assert.Equal(t, err, nil)
	return obj
}

func readYaml(filename string, t *testing.T) []byte {
	obj := readYamlObj(filename, t)
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

func TestGetAllCombinationFromYAML(t *testing.T) {
	benchmark := getBenchmark(benchmarkFile, t)

	iterations := controllers.GetCombinedIterations(benchmark)
	benchmarkSpecStr := benchmark.Spec.Spec
	var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	object := &unstructured.Unstructured{}
	decUnstructured.Decode([]byte(benchmarkSpecStr), nil, object)

	combinations := iterationHandlerYAML.GetAllCombination(iterations)
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
		iterationLabels = iterationHandlerYAML.GetAllCombination(iterations)
		firstLabel, iterationLabels = iterationLabels[0], iterationLabels[1:]
	} else {
		firstLabel = make(map[string]string)
		iterationLabels = []map[string]string{}
	}

	return obj, firstLabel, iterationLabels
}

func getBenchmarkWithIteration(ns string, benchmark *cpev1.Benchmark, benchmarkObj map[string]interface{}, iterationLabel map[string]string, build string, repetition int) (*unstructured.Unstructured, error) {
	// get hash
	jobName := "test"

	benchmarkObj["metadata"] = map[string]interface{}{"name": jobName, "namespace": ns}

	// generate job spec
	tmpl, err := template.New("").Parse(benchmark.Spec.Spec)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	expandLabel := controllers.GetExpandLabel(iterationLabel)
	err = tmpl.Execute(&buffer, expandLabel)
	if err != nil {
		return nil, err
	}
	executedSpec := buffer.String()

	var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	decUnstructured.Decode([]byte(executedSpec), nil, obj)

	specObject := obj.Object
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
	return extBenchmark, nil
}

func TestGeneratedBenchmark(t *testing.T) {
	benchmark := getBenchmark(benchmarkFile, t)
	benchmarkOperator := getBenchmarkOperator(benchmarkOperatorFile, t)

	firstLabel, iterationLabels, builds, maxRepetition := controllers.GetIteratedValues(benchmark)
	assert.Greater(t, len(iterationLabels), 0)

	repetition := 0
	for {

		if repetition >= maxRepetition {
			break
		}

		for _, build := range builds {
			// for firstLabel (iteration0 or nolabel)
			benchmarkObj := controllers.NewBenchmarkObject(benchmarkOperator)
			job, err := getBenchmarkWithIteration(benchmark.Namespace, benchmark, benchmarkObj, firstLabel, build, repetition)
			assert.Equal(t, err, nil)

			fmt.Println("First Label: ", firstLabel)
			fmt.Println("Job: ", job)
			expectedObj := readYamlObj(expectedJobFile, t)
			resources := iterationHandlerYAML.GetValue(job.Object, ".spec.template.spec.resources.limits.cpu")
			expectedResources := iterationHandlerYAML.GetValue(expectedObj.Object, ".spec.template.spec.resources.limits.cpu")
			assert.Equal(t, resources, expectedResources)

			command := iterationHandlerYAML.GetValue(job.Object, ".spec.template.spec.containers[0].command[2]")
			expectedCommand := iterationHandlerYAML.GetValue(expectedObj.Object, ".spec.template.spec.containers[0].command[2]")
			assert.Equal(t, command, expectedCommand)

			// for the rest iteration
			for _, iterationLabel := range iterationLabels {
				benchmarkObj = controllers.NewBenchmarkObject(benchmarkOperator)
				fmt.Println(iterationLabel)
				job, err = getBenchmarkWithIteration(benchmark.Namespace, benchmark, benchmarkObj, iterationLabel, build, repetition)
				assert.Equal(t, err, nil)
				fmt.Println(job)
				if iterationLabel["thread"] != "1" {
					expectedCommand = []string{fmt.Sprintf("./coremark-%sthreads.exe", iterationLabel["thread"])}
				} else {
					expectedCommand = []string{"./coremark-1thread.exe"}
				}
				command := iterationHandlerYAML.GetValue(job.Object, ".spec.template.spec.containers[0].command[2]")
				assert.Equal(t, command, expectedCommand)
			}
		}
		repetition = repetition + 1
	}
}

func TestGeneratedPlainBenchmark(t *testing.T) {
	benchmark := getBenchmark(plainBenchmarkFile, t)
	benchmarkOperator := getBenchmarkOperator(benchmarkOperatorFile, t)

	firstLabel, iterationLabels, builds, maxRepetition := controllers.GetIteratedValues(benchmark)
	assert.Equal(t, len(firstLabel), 0)
	assert.Equal(t, len(iterationLabels), 0)

	repetition := 0
	for {

		if repetition >= maxRepetition {
			break
		}

		for _, build := range builds {
			benchmarkObj := controllers.NewBenchmarkObject(benchmarkOperator)
			job, err := getBenchmarkWithIteration(benchmark.Namespace, benchmark, benchmarkObj, firstLabel, build, repetition)
			assert.Equal(t, err, nil)

			fmt.Println("First Label: ", firstLabel)
			fmt.Println("Job: ", job)
			expectedObj := readYamlObj(expectedJobFile, t)
			resources := iterationHandlerYAML.GetValue(job.Object, ".spec.template.spec.resources.limits.cpu")
			expectedResources := iterationHandlerYAML.GetValue(expectedObj.Object, ".spec.template.spec.resources.limits.cpu")
			assert.Equal(t, resources, expectedResources)

			command := iterationHandlerYAML.GetValue(job.Object, ".spec.template.spec.containers[0].command[2]")
			expectedCommand := iterationHandlerYAML.GetValue(expectedObj.Object, ".spec.template.spec.containers[0].command[2]")
			assert.Equal(t, command, expectedCommand)
		}
		repetition = repetition + 1
	}
}

func TestGeneratedCombinedBenchmark(t *testing.T) {
	benchmark := getBenchmark(combinedBenchmarkFile, t)
	benchmarkOperator := getBenchmarkOperator(benchmarkOperatorFile, t)
	firstLabel, iterationLabels, builds, maxRepetition := controllers.GetIteratedValues(benchmark)
	expandLabel := controllers.GetExpandLabel(firstLabel)
	assert.Equal(t, len(expandLabel["stressor"].([]string)), 2)
	assert.Equal(t, len(iterationLabels), 2)

	repetition := 0
	for {

		if repetition >= maxRepetition {
			break
		}

		for _, build := range builds {
			benchmarkObj := controllers.NewBenchmarkObject(benchmarkOperator)
			job, err := getBenchmarkWithIteration(benchmark.Namespace, benchmark, benchmarkObj, firstLabel, build, repetition)
			assert.Equal(t, err, nil)

			fmt.Println("First Label: ", firstLabel)
			fmt.Println("Job: ", job)
			expectedObj := readYamlObj(expectedCombinedBenchmarkFile, t)
			stressor := iterationHandlerYAML.GetValue(job.Object, ".spec.template.spec.containers[0].env[name=STRESSOR].value")
			expectedStressor := iterationHandlerYAML.GetValue(expectedObj.Object, ".spec.template.spec.containers[0].env[name=STRESSOR].value")
			assert.Equal(t, stressor, expectedStressor)

			load := iterationHandlerYAML.GetValue(job.Object, ".spec.template.spec.containers[0].env[name=STRESS_LOAD].value")
			expectedLoad := iterationHandlerYAML.GetValue(expectedObj.Object, ".spec.template.spec.containers[0].env[name=STRESS_LOAD].value")
			assert.Equal(t, load, expectedLoad)
		}
		repetition = repetition + 1
	}
}
