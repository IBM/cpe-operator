/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/IBM/cpe-operator/cpe-parser/common"
	"github.com/IBM/cpe-operator/cpe-parser/parser"
	"github.com/gorilla/mux"
)

// ///////////////////////////////////////////
// add parser key -> module map here
var codaitParser parser.Parser = parser.NewCodaitParser()
var defaultParser parser.Parser = parser.NewDefaultParser()
var sysbenchParser parser.Parser = parser.NewSysbenchParser()
var iperf3Parser parser.Parser = parser.NewIperf3Parser()
var osuParser parser.Parser = parser.NewOSUParser()
var tpccParser parser.Parser = parser.NewTPCCParser()
var coremarkParser parser.Parser = parser.NewCoremarkParser()
var fioParser parser.Parser = parser.NewFioParser()
var glooParser parser.Parser = parser.NewGlooParser()
var gromacsParser parser.Parser = parser.NewGromacsParser()
var linpackParser parser.Parser = parser.NewLinpackParser()
var timeParser parser.Parser = parser.NewTimeParser()
var stressParser parser.Parser = parser.NewStressParser()

var parserMap map[string]parser.Parser = map[string]parser.Parser{
	"codait":   codaitParser,
	"default":  defaultParser,
	"sysbench": sysbenchParser,
	"iperf3":   iperf3Parser,
	"osu":      osuParser,
	"tpcc":     tpccParser,
	"coremark": coremarkParser,
	"fio":      fioParser,
	"gloo":     glooParser,
	"gromacs":  gromacsParser,
	"linpack":  linpackParser,
	"time":     timeParser,
	"stress":   stressParser,
}

/////////////////////////////////////////////

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

func getLogSpec(r *http.Request) (LogSpec, error) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var logSpec LogSpec
	err := json.Unmarshal(reqBody, &logSpec)
	return logSpec, err
}

func getRawLog(r *http.Request) (RawLog, error) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var rawLog RawLog
	err := json.Unmarshal(reqBody, &rawLog)
	return rawLog, err
}

func getLogFromSpec(logSpec LogSpec) ([]byte, error) {
	cos := common.NewCOS()
	keyName := fmt.Sprintf("%s/%s/%s/%s.log", logSpec.BenchmarkName, logSpec.ClusterID, logSpec.JobName, logSpec.PodName)
	return common.GetLog(cos, keyName)
}

func parseValue(parserKey string, body []byte) (string, float64, map[string]interface{}, error) {
	if generalParser, ok := parserMap[parserKey]; ok {
		values, err := generalParser.ParseValue(body)
		if err == nil {
			pkey, pvalue := generalParser.GetPerformanceValue(values)
			return pkey, pvalue, values, err
		}
		return "", -1, values, err
	}
	return "", -1, nil, errors.New("ParserKeyNotFound")
}

func ReqLog(w http.ResponseWriter, r *http.Request) {
	logSpec, err := getLogSpec(r)
	var data string
	var status string
	if err != nil {
		status = "ERROR"
		data = fmt.Sprintf("%v", err)
	} else {
		body, err := getLogFromSpec(logSpec)
		if err != nil {
			status = "ERROR"
			data = fmt.Sprintf("%v", err)
		} else {
			status = "OK"
			data = string(body)
		}
	}
	res := Response{status, data, "", -1.0}
	json.NewEncoder(w).Encode(res)

}

func ReqParsedValue(w http.ResponseWriter, r *http.Request) {
	logSpec, err := getLogSpec(r)
	var msg string
	var status string
	pkey := ""
	pval := -1.0
	if err != nil {
		status = "ERROR"
		msg = fmt.Sprintf("%v", err)
	} else {
		body, err := getLogFromSpec(logSpec)

		if err != nil {
			status = "ERROR"
			msg = fmt.Sprintf("%v", err)
		} else {
			ppkey, ppval, values, err := parseValue(logSpec.Parser, body)
			if err != nil {
				status = "ERROR"
				msg = fmt.Sprintf("%v", err)
			} else {
				status = "OK"
				dataBytes, _ := json.Marshal(values)
				msg = string(dataBytes)
				pkey = ppkey
				pval = ppval
			}
		}
	}
	res := Response{status, msg, pkey, pval}
	json.NewEncoder(w).Encode(res)
}

func ReqPushLog(w http.ResponseWriter, r *http.Request) {
	logSpec, err := getLogSpec(r)
	var msg string
	var status string
	pkey := ""
	pval := -1.0
	if err != nil {
		status = "ERROR"
		msg = fmt.Sprintf("%v", err)
	} else {
		body, err := getLogFromSpec(logSpec)
		if err != nil {
			status = "ERROR"
			msg = fmt.Sprintf("%v", err)
		} else {
			ppkey, ppval, values, err := parseValue(logSpec.Parser, body)
			if err != nil {
				status = "ERROR"
				msg = fmt.Sprintf("%v", err)
			} else {
				err := common.PushValues(logSpec.Parser, logSpec.ClusterID, logSpec.Instance, logSpec.BenchmarkName, logSpec.JobName, logSpec.PodName, logSpec.ConstLabels, values)
				if err != nil {
					status = "ERROR"
					msg = fmt.Sprintf("%v", err)
					pkey = ppkey
					pval = ppval
				} else {
					status = "OK"
					msg = fmt.Sprintf("%v", logSpec.ConstLabels)
					pkey = ppkey
					pval = ppval
				}
			}
		}
	}
	res := Response{status, msg, pkey, pval}
	json.NewEncoder(w).Encode(res)
}

func ReqRawParse(w http.ResponseWriter, r *http.Request) {
	var msg string
	var status string
	pkey := ""
	pval := -1.0
	rawLog, err := getRawLog(r)
	if err != nil {
		status = "ERROR"
		msg = fmt.Sprintf("%v", err)
	} else {
		ppkey, ppval, values, err := parseValue(rawLog.Parser, rawLog.LogValue)
		if err != nil {
			status = "ERROR"
			msg = fmt.Sprintf("%v", err)
		} else {
			status = "OK"
			dataBytes, _ := json.Marshal(values)
			msg = string(dataBytes)
			pkey = ppkey
			pval = ppval
		}
	}
	res := Response{status, msg, pkey, pval}
	json.NewEncoder(w).Encode(res)
}

func GetRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/log", ReqLog).Methods("POST")
	router.HandleFunc("/parse", ReqParsedValue).Methods("POST")
	router.HandleFunc("/push", ReqPushLog).Methods("POST")
	router.HandleFunc("/raw-parse", ReqRawParse).Methods("POST")
	return router
}
