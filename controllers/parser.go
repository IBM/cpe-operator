/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package controllers

///////////////////////////////////////////////////////////////////////////
//
// parser.go
//
// parseAndPushLog
// - call parser service to parse the log to prometheus-format metrics
//   and push to prometheus push gateway
//
////////////////////////////////////////////////////////////////////////////

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
)

var CLUSTER_ID string = os.Getenv("CLUSTER_ID")

type LogSpec struct {
	ClusterID     string            `json:"cluster"`
	Instance      string            `json:"instance"`
	BenchmarkName string            `json:"benchmark"`
	JobName       string            `json:"job"`
	PodName       string            `json:"pod"`
	Parser        string            `json:"parser"`
	ConstLabels   map[string]string `json:"labels"`
}

type RawLog struct {
	Parser   string `json:"parser"`
	LogValue []byte `json:"log"`
}

type Response struct {
	Status           string  `json:"status"`
	Message          string  `json:"msg"`
	PerformanceKey   string  `json:"pkey"`
	PerformanceValue float64 `json:"pval"`
}

var PUSH_URL string = os.Getenv("PARSER_SERVICE") + "/push"
var PARSE_URL string = os.Getenv("PARSER_SERVICE") + "/parse"
var PARSE_RAW_URL string = os.Getenv("PARSER_SERVICE") + "/raw-parse"

const (
	reqHeader = "application/json; charset=utf-8"
)

// parseLog: parse remote log on COS only
func parseLog(benchmarkName string, jobName string, podName string, parser string) (Response, error) {

	logSpec := LogSpec{
		CLUSTER_ID,
		"",
		benchmarkName,
		jobName,
		podName,
		parser,
		make(map[string]string),
	}

	jsonReq, err := json.Marshal(logSpec)
	if err != nil {
		return Response{}, err
	} else {
		res, err := http.Post(PARSE_URL, reqHeader, bytes.NewBuffer(jsonReq))
		if err != nil {
			return Response{}, err
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return Response{}, err
		}
		var response Response
		json.Unmarshal(body, &response)
		return response, err
	}
}

// parseAndPushLog: parse remote log on COS and push to pushgateway
func parseAndPushLog(instance string, benchmarkName string, jobName string, podName string, parser string, constLabels map[string]string) (Response, error) {
	logSpec := LogSpec{
		CLUSTER_ID,
		instance,
		benchmarkName,
		jobName,
		podName,
		parser,
		constLabels,
	}

	jsonReq, err := json.Marshal(logSpec)
	if err != nil {
		return Response{}, err
	} else {
		res, err := http.Post(PUSH_URL, reqHeader, bytes.NewBuffer(jsonReq))
		if err != nil {
			return Response{}, err
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return Response{}, err
		}
		var response Response
		json.Unmarshal(body, &response)
		return response, err
	}
}

// parseRawLog: parse raw log
func parseRawLog(parser string, logValue []byte) (Response, error) {
	rawLog := RawLog{
		parser,
		logValue,
	}

	jsonReq, err := json.Marshal(rawLog)
	if err != nil {
		return Response{}, err
	} else {
		res, err := http.Post(PARSE_RAW_URL, reqHeader, bytes.NewBuffer(jsonReq))
		if err != nil {
			return Response{}, err
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return Response{}, err
		}
		var response Response
		json.Unmarshal(body, &response)
		return response, err
	}
}
