/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 // Run go test -v parser/default_test.go
package parser

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/IBM/cpe-operator/cpe-parser/parser"
	"github.com/stretchr/testify/assert"
)

func TestParseValue(t *testing.T) {
	fileName := "sample/raysample_pod_log.log"
	bytes, err := ioutil.ReadFile(fileName)
	defaultParser := parser.NewDefaultParser()
	var generalParser parser.Parser
	generalParser = defaultParser
	assert.Nil(t, err)
	values, err := generalParser.ParseValue(bytes)
	fmt.Printf("Values: %v\n", values)
	assert.Nil(t, err)
	assert.Equal(t, len(values), 1)
}
