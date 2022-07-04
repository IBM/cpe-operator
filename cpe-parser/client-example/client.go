/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

var BenchmarkName string = "ray-nlp-benchmark"
var JobName string = "ray-nlp-benchmark-init"
var PodName string = "ray-nlp-benchmark-init-9bdh6"
var Instance string = "10.244.0.8"

type LogSpec struct {
	Instance      string `json:"instance"`
	BenchmarkName string `json:"benchmark"`
	JobName       string `json:"job"`
	PodName       string `json:"pod"`
	Parser        string `json:"parser"`
}

type Response struct {
	Status string `json:"status"`
	Data   string `json:"data"`
}

func getSampleLogSpec() LogSpec {
	logSpec := LogSpec{
		Instance,
		BenchmarkName,
		JobName,
		PodName,
		"codait",
	}
	return logSpec
}

func main() {

	logSpec := getSampleLogSpec()
	jsonReq, err := json.Marshal(logSpec)
	if err != nil {
		log.Fatal("Json Error")
	} else {
		res, err := http.Post("http://localhost:8080/push", "application/json; charset=utf-8", bytes.NewBuffer(jsonReq))
		if err != nil {
			log.Fatal("NewRequest Error")
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Fatal("Read Error")
		}
		var response Response
		json.Unmarshal(body, &response)
		fmt.Printf("%s\n", string(body))
	}

}
