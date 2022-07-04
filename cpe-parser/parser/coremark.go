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

type CoremarkParser struct {
	*BaseParser
}

var CoremarkKeyMap map[string]string = map[string]string{
	"Total ticks":       "total_ticks",
	"Total time (secs)": "total_sec",
	"Iterations/Sec":    "iterations_per_sec",
}

/*
Total ticks      : 24007
Total time (secs): 24.007000
Iterations/Sec   : 266588.911567
*/

func NewCoremarkParser() *CoremarkParser {
	coremarkParser := &CoremarkParser{}
	abs := &BaseParser{
		Parser: coremarkParser,
	}
	coremarkParser.BaseParser = abs
	return coremarkParser
}

func (p *CoremarkParser) ParseValue(body []byte) (map[string]interface{}, error) {
	tmpValues := make(map[string][]float64)
	values := make(map[string]interface{})
	for _, valkey := range CoremarkKeyMap {
		values[valkey] = []float64{}
	}
	bytesReader := bytes.NewReader(body)
	bufReader := bufio.NewReader(bytesReader)

	for {
		line, _, err := bufReader.ReadLine()
		linestr := string(line)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		for keyword, valkey := range CoremarkKeyMap {
			if strings.Contains(linestr, keyword) {
				splited := strings.Fields(linestr)
				value := splited[len(splited)-1]
				fval, _ := strconv.ParseFloat(value, 64)
				tmpValues[valkey] = append(tmpValues[valkey], fval)
			}
		}
	}
	for key, tmpValue := range tmpValues {
		values[key] = tmpValue
	}
	return values, nil
}

func (p *CoremarkParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := CoremarkKeyMap["Iterations/Sec"]
	performanceValues := values[performanceKey].([]float64)
	if len(performanceValues) == 0 {
		return performanceKey, -1
	}
	var sum float64
	sum = 0
	for _, val := range performanceValues {
		sum += val
	}
	avg := sum / float64(len(performanceValues))
	return performanceKey, avg
}
