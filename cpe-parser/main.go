/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

var requiredEnvs []string = []string{"CPE_COS_LOG_APIKEY", "CPE_COS_LOG_SERVICE_ID", "CPE_COS_LOG_AUTH_ENDPOINT", "CPE_COS_LOG_SERVICE_ENDPOINT", "CPE_COS_LOG_RAW_BUCKET", "PUSHGATEWAY_URL"}

func envChecked() bool {

	for _, env := range requiredEnvs {
		_, present := os.LookupEnv(env)
		if !present {
			return false
		}
	}
	return true

}

func main() {
	if !envChecked() {
		log.Fatal(fmt.Sprintf("Some environment is not set (%v)", requiredEnvs))
	}
	fmt.Println("Parser REST API - Mux Router")
	var parserKeys []string
	for key, _ := range parserMap {
		parserKeys = append(parserKeys, key)
	}
	fmt.Print("Available Parser: ", parserKeys)
	router := GetRouter()
	http.ListenAndServe(":8080", router)
}
