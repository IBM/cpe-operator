/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

// Run go test -v parser/fmtrain_test.go

package parser

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/IBM/cpe-operator/cpe-parser/parser"
	"github.com/stretchr/testify/assert"
)

// update log key here
const (
	LOG_KEY string = "fmtrain"
)

func getFileName() string {
	return fmt.Sprintf("sample/%s_pod_log.log", LOG_KEY)
}

var generalParser parser.Parser

// update parser init function
var testParser = parser.NewFMTrainParser()

func TestParseValue(t *testing.T) {
	fileName := getFileName()
	bytes, err := ioutil.ReadFile(fileName)
	generalParser = testParser
	assert.Nil(t, err)
	values, err := generalParser.ParseValue(bytes)
	assert.Nil(t, err)
	// update assert value length
	assert.Equal(t, len(values), 5)
}

func TestGetPerformanceValue(t *testing.T) {
	fileName := getFileName()
	bytes, err := ioutil.ReadFile(fileName)
	generalParser = testParser
	assert.Nil(t, err)
	values, err := generalParser.ParseValue(bytes)
	key, pvalue := testParser.GetPerformanceValue(values)
	fmt.Printf("PKey: %s, Pvalue: %.2f\n", key, pvalue)
	// update assert performance value
	assert.Equal(t, pvalue, 6.763322448730468)
}
