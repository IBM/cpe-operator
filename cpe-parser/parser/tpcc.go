/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 package parser

import (
	"bufio"
	"bytes"
	"io"
	"strconv"
	"strings"
)

const (
	TPCCResultKey       = "_elapsed_______tpmC____efc__avg(ms)__p50(ms)__p90(ms)__p95(ms)__p99(ms)_pMax(ms)"
	TPCCAuditKey        = "Audit check"
	TPCCFailKey         = "FAIL"
	TPCCAuditCheckLabel = "audit"
)

type TPCCParser struct {
	*BaseParser
}

func NewTPCCParser() *TPCCParser {
	tpccParser := &TPCCParser{}
	abs := &BaseParser{
		Parser: tpccParser,
	}
	tpccParser.BaseParser = abs
	return tpccParser
}

func (p *TPCCParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	var columns []string
	bytesReader := bytes.NewReader(body)
	bufReader := bufio.NewReader(bytesReader)
	exists := false
	pass := 1

	for {
		line, _, err := bufReader.ReadLine()
		linestr := string(line)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if !exists {
			if strings.Contains(linestr, TPCCResultKey) {
				exists = true
				resultKey := strings.ReplaceAll(TPCCResultKey, "_", " ")
				columns = strings.Fields(resultKey)
			}
		} else {
			resultValue := strings.ReplaceAll(linestr, "_", " ")
			results := strings.Fields(resultValue)
			for index, column := range columns {
				if index == 0 {
					continue
				}
				result, _ := strconv.ParseFloat(results[index], 64)
				values[column] = result
			}
			exists = false
		}
		if strings.Contains(linestr, TPCCAuditKey) && strings.Contains(linestr, TPCCFailKey) {
			pass = 0
		}
	}
	values[TPCCAuditCheckLabel] = float64(pass)
	return values, nil
}

func (p *TPCCParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := "tpmC"
	if performanceValue, ok := values[performanceKey]; !ok {
		return performanceKey, -1
	} else {
		return performanceKey, performanceValue.(float64)
	}
}
