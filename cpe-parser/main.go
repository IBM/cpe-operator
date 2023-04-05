/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("Parser REST API - Mux Router")
	var parserKeys []string
	for key, _ := range parserMap {
		parserKeys = append(parserKeys, key)
	}
	fmt.Print("Available Parser: ", parserKeys)
	router := GetRouter()
	http.ListenAndServe(":8080", router)
}
