/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 package main

//////////////////////////////////////////////
// Test handler function
// router.HandleFunc("/log", reqLog)
// router.HandleFunc("/parse", reqParsedValue)
// router.HandleFunc("/push", reqPushLog)
//
// go test -v api_test.go
//////////////////////////////////////////////

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	cpe "github.com/IBM/cpe-operator/cpe-parser"
	"github.com/IBM/cpe-operator/cpe-parser/common"
	"github.com/IBM/cpe-operator/cpe-parser/secret"
	"github.com/stretchr/testify/assert"
)

var logSpec = getSampleLogSpec()

func getSampleLogSpec() cpe.LogSpec {
	spec, _ := json.Marshal(secret.TPCCSpecData)
	var cpeLog cpe.LogSpec
	json.Unmarshal(spec, &cpeLog)
	return cpeLog
}

func makeRequest(t *testing.T, path string, handlerFunc func(http.ResponseWriter, *http.Request)) cpe.Response {
	secret.Setenvs()
	jsonReq, err := json.Marshal(logSpec)
	assert.Nil(t, err)
	req, err := http.NewRequest("PUT", path, bytes.NewBuffer(jsonReq))
	assert.Nil(t, err)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res := httptest.NewRecorder()
	handler := http.HandlerFunc(handlerFunc)
	handler.ServeHTTP(res, req)
	assert.Equal(t, http.StatusOK, res.Code)
	body, _ := ioutil.ReadAll(res.Body)
	var response cpe.Response
	json.Unmarshal(body, &response)
	assert.Equal(t, response.Status, "OK")
	if response.Status != "OK" {
		fmt.Printf("%s\n", string(body))
	}
	return response
}

func putSampleLog(t *testing.T) error {
	cos := common.NewCOS()
	keyName := fmt.Sprintf("%s/%s/%s/%s.log", logSpec.BenchmarkName, logSpec.ClusterID, logSpec.JobName, logSpec.PodName)
	podLogFileName := fmt.Sprintf("parser/%s_pod_log.log", logSpec.BenchmarkName)
	podLogs, err := ioutil.ReadFile(podLogFileName)
	assert.Nil(t, err)
	return common.PutLog(cos, keyName, podLogs)
}

func TestReqLog(t *testing.T) {
	putSampleLog(t)
	response := makeRequest(t, "/log", cpe.ReqLog)
	if response.Status == "OK" {
		fmt.Printf("Log Response: %s\n", string(response.Message))
	}
}

func TestReqParsedValue(t *testing.T) {
	response := makeRequest(t, "/parse", cpe.ReqParsedValue)
	if response.Status == "OK" {
		fmt.Printf("Parse Response: %v\n", response)
	}
}

func TestReqPushValue(t *testing.T) {
	response := makeRequest(t, "/push", cpe.ReqPushLog)
	if response.Status == "OK" {
		fmt.Printf("Push Response: %s\n", string(response.Message))
		fmt.Printf("Performance Key: %s, Performance Value: %.2f\n", response.PerformanceKey, response.PerformanceValue)
	}
}
