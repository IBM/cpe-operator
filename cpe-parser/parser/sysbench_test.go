/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 // Run go test -v parser/sysbench_test.go

package parser

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/IBM/cpe-operator/cpe-parser/parser"
	"github.com/stretchr/testify/assert"
)

const (
	LOG_KEY string = "sysbench"
)

var sysbenchParser = parser.NewSysbenchParser()
var generalParser parser.Parser

func getFileName() string {
	return fmt.Sprintf("sample/%s_pod_log.log", LOG_KEY)
}

func TestParseValue(t *testing.T) {
	fileName := getFileName()
	bytes, err := ioutil.ReadFile(fileName)
	generalParser = sysbenchParser
	assert.Nil(t, err)
	values, err := generalParser.ParseValue(bytes)
	fmt.Printf("Values: %v\n", values)
	assert.Nil(t, err)
	assert.Equal(t, len(values), len(parser.SysbenchKeyMap))
}

func TestGetPerformanceValue(t *testing.T) {
	fileName := getFileName()
	bytes, err := ioutil.ReadFile(fileName)
	generalParser = sysbenchParser
	assert.Nil(t, err)
	values, err := generalParser.ParseValue(bytes)
	key, pvalue := sysbenchParser.GetPerformanceValue(values)
	fmt.Printf("PKey: %s, Pvalue: %.2f\n", key, pvalue)
	assert.NotEqual(t, pvalue, -1)
}
