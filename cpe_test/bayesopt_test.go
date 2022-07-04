/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */
// go test -v cpe_test/bayesopt_test.go

package controllers

import (
	"encoding/json"
	"math/rand"
	"os"
	"testing"

	"github.com/IBM/cpe-operator/controllers"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"fmt"
)

const (
	CONFIG_FOLDER = "../iteration/node-tuning/search-space-simple"
)

func TestGetSearchSpaceConfig(t *testing.T) {
	paramMap, paramNameMap, err := controllers.GetSearchSpaceConfig(CONFIG_FOLDER)
	assert.Equal(t, err, nil)
	fmt.Println(paramMap)
	fmt.Println(paramNameMap)
	assert.Greater(t, len(paramMap), 0)
	assert.Greater(t, len(paramNameMap), 0)
}

func printUnstructure(obj *unstructured.Unstructured) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(obj)
}

func TestGetAutoTunedProfile(t *testing.T) {
	paramMap, paramNameMap, err := controllers.GetSearchSpaceConfig(CONFIG_FOLDER)
	assert.Equal(t, err, nil)
	controllers.SearchSpace = paramMap
	controllers.ParamNameMap = paramNameMap
	nodeTunedOptimizer := controllers.NewBayesOptimizer(false)
	defer nodeTunedOptimizer.Finalize()
	go nodeTunedOptimizer.AutoTune()
	for {
		fmt.Println("Queue length: ", len(nodeTunedOptimizer.SampleQueue))
		sampledProfileMaps, ok := <-nodeTunedOptimizer.SampleQueue
		if !ok {
			sampledProfileMaps = nodeTunedOptimizer.FinalizedTunedProfile
			nodeTunedOptimizer.SetFinalizedApplied()
			profile := controllers.GetAutoTunedProfile(sampledProfileMaps)
			printUnstructure(profile)
			break
		} else {
			//	time.Sleep(30)
			nodeTunedOptimizer.ResultQueue <- float64(rand.Intn(10))
		}
	}
	fmt.Println("Total Run: ", nodeTunedOptimizer.SamplingCount)
	fmt.Println("Final: ", nodeTunedOptimizer.FinalizedTunedProfile)
}
