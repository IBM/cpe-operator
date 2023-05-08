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
		"value": "{{.valA}}",
	}

	var env2 map[string]interface{} = map[string]interface{}{
		"name":  "MAX_SCALE",
		"value": "{{.valScale}}",
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
					"ibm.com/zone": "{{.zone}}",
				},
				"nodeSelector2": map[string]interface{}{
					"ibm.com/zone": "{{.zone}}",
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
		Name:   "valA",
		Values: []string{"a", "b", "c"},
	},
	cpev1.IterationItem{
		Name:   "valScale",
		Values: []string{"3", "4", "8"},
	},
	cpev1.IterationItem{
		Name:   "zone",
		Values: []string{"jp-tok-1", "jp-tok-2"},
	},
}

var expectedCombinations []map[string]string = []map[string]string{
	map[string]string{"valA": "a", "valScale": "3", "zone": "jp-tok-1"},
	map[string]string{"valA": "b", "valScale": "3", "zone": "jp-tok-1"},
	map[string]string{"valA": "c", "valScale": "3", "zone": "jp-tok-1"},
	map[string]string{"valA": "a", "valScale": "4", "zone": "jp-tok-1"},
	map[string]string{"valA": "b", "valScale": "4", "zone": "jp-tok-1"},
	map[string]string{"valA": "c", "valScale": "4", "zone": "jp-tok-1"},
	map[string]string{"valA": "a", "valScale": "8", "zone": "jp-tok-1"},
	map[string]string{"valA": "b", "valScale": "8", "zone": "jp-tok-1"},
	map[string]string{"valA": "c", "valScale": "8", "zone": "jp-tok-1"},
	map[string]string{"valA": "a", "valScale": "3", "zone": "jp-tok-2"},
	map[string]string{"valA": "b", "valScale": "3", "zone": "jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "3", "zone": "jp-tok-2"},
	map[string]string{"valA": "a", "valScale": "4", "zone": "jp-tok-2"},
	map[string]string{"valA": "b", "valScale": "4", "zone": "jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "4", "zone": "jp-tok-2"},
	map[string]string{"valA": "a", "valScale": "8", "zone": "jp-tok-2"},
	map[string]string{"valA": "b", "valScale": "8", "zone": "jp-tok-2"},
	map[string]string{"valA": "c", "valScale": "8", "zone": "jp-tok-2"},
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
