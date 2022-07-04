/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */
// go test -v cpe_test/iteration_test.go

package controllers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
	"github.com/IBM/cpe-operator/controllers"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetInitObject() map[string]interface{} {
	var env1 map[string]interface{} = map[string]interface{}{
		"name":  "A",
		"value": "a",
	}

	var env2 map[string]interface{} = map[string]interface{}{
		"name":  "MAX_SCALE",
		"value": "1",
	}
	var envlist []interface{} = []interface{}{env1, env2}
	var object map[string]interface{} = map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"env": envlist,
						"command": []interface{}{
							"bin",
							"-c",
						},
					},
				},
				"nodeSelector": map[string]interface{}{
					"ibm.com/zone": "jp-tok-1",
				},
				"nodeSelector2": map[string]interface{}{
					"ibm.com/zone": "jp-tok-1",
				},
			},
		},
	}
	return object
}

var null_list []interface{} = []interface{}{}
var null_object map[string]interface{} = map[string]interface{}{
	"template": map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"env": null_list,
				},
			},
		},
	},
}

var instance *unstructured.Unstructured = &unstructured.Unstructured{
	Object: GetInitObject(),
}

var sampleIteration []cpev1.IterationItem = []cpev1.IterationItem{
	cpev1.IterationItem{
		Name:     "valA",
		Location: ".template.spec.containers[0].env[name=A].value",
		Values:   []string{"b", "c"},
	},
	cpev1.IterationItem{
		Name:     "valScale",
		Location: ".template.spec.containers[0].env[name=MAX_SCALE].value",
		Values:   []string{"4", "8"},
	},
	cpev1.IterationItem{
		Name:     "zone",
		Location: ".template.spec.nodeSelector.(ibm.com/zone);.template.spec.nodeSelector2.(ibm.com/zone)",
		Values:   []string{"jp-tok-2;jp-tok-2"},
	},
}

var sampleFreeIteration []cpev1.IterationItem = []cpev1.IterationItem{
	cpev1.IterationItem{
		Name:     "valA",
		Location: ".template.spec.containers[0].env[name=A].value",
		Values:   []string{"b", "c"},
	},
	cpev1.IterationItem{
		Name:     "valScale",
		Location: ".template.spec.containers[0].env[name=MAX_SCALE].value",
		Values:   []string{},
	},
	cpev1.IterationItem{
		Name:     "zone",
		Location: ".template.spec.nodeSelector.(ibm.com/zone);.template.spec.nodeSelector2.(ibm.com/zone)",
		Values:   []string{"jp-tok-2;jp-tok-2"},
	},
}

var expectedCombinationsWithInit []map[string]string = []map[string]string{
	map[string]string{"valA": "a", "valScale": "4", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "b", "valScale": "4", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "c", "valScale": "4", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "a", "valScale": "8", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "b", "valScale": "8", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "c", "valScale": "8", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "a", "valScale": "3", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "b", "valScale": "3", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "c", "valScale": "3", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "a", "valScale": "4", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "b", "valScale": "4", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "4", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "a", "valScale": "8", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "b", "valScale": "8", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "8", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "a", "valScale": "3", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "b", "valScale": "3", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "3", "zone": "jp-tok-2;jp-tok-2"},
}

var expectedCombinations []map[string]string = []map[string]string{
	map[string]string{"valA": "b", "valScale": "4", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "4", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "b", "valScale": "8", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "8", "zone": "jp-tok-2;jp-tok-2"},
}

var expectedCombinationsOfFreeIteration []map[string]string = []map[string]string{
	map[string]string{"valA": "b", "valScale": "nil", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "nil", "zone": "jp-tok-2;jp-tok-2"},
}

var expectedCombinationsOfFreeIterationWithInit []map[string]string = []map[string]string{
	map[string]string{"valA": "a", "valScale": "3", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "b", "valScale": "3", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "c", "valScale": "3", "zone": "jp-tok-1;jp-tok-1"},
	map[string]string{"valA": "a", "valScale": "3", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "b", "valScale": "3", "zone": "jp-tok-2;jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "3", "zone": "jp-tok-2;jp-tok-2"},
}

const (
	valueToSet = "3"
	zoneToSet  = "jp-tok-2;jp-tok-2"
)

var iterationHandler *controllers.IterationHandler = &controllers.IterationHandler{}

func TestUpdateValue(t *testing.T) {
	location := ".template.spec.containers[0].env[name=MAX_SCALE].value"
	zoneLocation := ".template.spec.nodeSelector.(ibm.com/zone);.template.spec.nodeSelector2.(ibm.com/zone)"
	object := GetInitObject()
	fmt.Printf("Original: %v\n", object)
	modifiedObject := iterationHandler.UpdateValue(object, location, valueToSet)
	modifiedObject = iterationHandler.UpdateValue(modifiedObject, zoneLocation, zoneToSet)
	containers := modifiedObject["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})
	envList := containers[0].(map[string]interface{})["env"].([]interface{})
	fmt.Printf("Modify: %v\n", modifiedObject)
	for _, env := range envList {
		if env.(map[string]interface{})["name"] == "MAX_SCALE" {
			setValue := env.(map[string]interface{})["value"]
			assert.Equal(t, setValue, valueToSet)
		}
	}

	fmt.Printf("Original: %v\n", null_object)
	modifiedObject = iterationHandler.UpdateValue(null_object, location, valueToSet)
	modifiedObject = iterationHandler.UpdateValue(modifiedObject, zoneLocation, zoneToSet)
	containers = modifiedObject["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})
	envList = containers[0].(map[string]interface{})["env"].([]interface{})
	fmt.Printf("Modify: %v\n", modifiedObject)
	for _, env := range envList {
		if env.(map[string]interface{})["name"] == "MAX_SCALE" {
			setValue := env.(map[string]interface{})["value"]
			assert.Equal(t, setValue, valueToSet)
		}
	}

	location = ".template.spec.containers[0].command[1]"
	object = GetInitObject()
	modifiedObject = iterationHandler.UpdateValue(object, location, "-f")
	fmt.Printf("Modify Command: %v\n", modifiedObject)
}

func TestGetAllCombination(t *testing.T) {
	combinations := iterationHandler.GetAllCombination(sampleIteration)
	fmt.Printf("Combinations: %v\n", combinations)
	assert.Equal(t, len(combinations), len(expectedCombinations))
	assert.Equal(t, combinations, expectedCombinations)
}

func TestGetInitCombination(t *testing.T) {
	object := GetInitObject()
	containers := object["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})
	envList := containers[0].(map[string]interface{})["env"].([]interface{})
	var valA, valScale, zone string
	for _, env := range envList {
		name := env.(map[string]interface{})["name"]
		if name == "A" {
			valA = env.(map[string]interface{})["value"].(string)
		} else if name == "MAX_SCALE" {
			valScale = env.(map[string]interface{})["value"].(string)
		}
	}
	object = GetInitObject()
	zone1 := object["template"].(map[string]interface{})["spec"].(map[string]interface{})["nodeSelector"].(map[string]interface{})["ibm.com/zone"].(string)
	zone2 := object["template"].(map[string]interface{})["spec"].(map[string]interface{})["nodeSelector2"].(map[string]interface{})["ibm.com/zone"].(string)
	zone = zone1 + ";" + zone2

	var expectedInit map[string]string = map[string]string{
		"valA":     valA,
		"valScale": valScale,
		"zone":     zone,
	}
	fmt.Printf("ValA %v\n", valA)

	initCombination := iterationHandler.GetInitCombination(object, sampleIteration)
	fmt.Printf("Init Combination: %v\n", initCombination)
	assert.Equal(t, initCombination, expectedInit)
}

func TestGetInitAndAllCombination(t *testing.T) {
	object := GetInitObject()
	combinations := iterationHandler.GetInitAndAllCombination(object, sampleIteration)
	fmt.Printf("Combinations: %v\n", combinations)
	assert.Equal(t, len(combinations), len(expectedCombinationsWithInit))
	var nextMap map[string]string
	nextMap, combinations = combinations[0], combinations[1:]
	fmt.Printf("NextMap: %v, Combinations: %v\n", nextMap, combinations)

	combinations = iterationHandler.GetInitAndAllCombination(object, sampleFreeIteration)
	fmt.Printf("Combinations from Free Iteration: %v\n", combinations)
	assert.Equal(t, len(combinations), len(expectedCombinationsOfFreeIterationWithInit))

	null_object = map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"env": null_list,
					},
				},
			},
		},
	}

	combinations = iterationHandler.GetInitAndAllCombination(null_object, sampleIteration)
	fmt.Printf("Null combinations: %v\n", combinations)
	assert.Equal(t, len(combinations), len(expectedCombinations))
	assert.Equal(t, combinations, expectedCombinations)

	combinations = iterationHandler.GetInitAndAllCombination(null_object, sampleFreeIteration)
	fmt.Printf("Combinations from Free Iteration (Null): %v\n", combinations)
	assert.Equal(t, len(combinations), len(expectedCombinationsOfFreeIteration))

}
