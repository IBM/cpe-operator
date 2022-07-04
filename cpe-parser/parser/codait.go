/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var CodaitKeys []string = []string{"NoWatson_NLP_Benchmark", "SerialNoWatson_NLP_Benchmark", "CodaitNlp_SerialPostProcessing_Microbenchmark"}

type CodaitParser struct {
	*BaseParser
}

func NewCodaitParser() *CodaitParser {
	codait := &CodaitParser{}
	abs := &BaseParser{
		Parser: codait,
	}
	codait.BaseParser = abs
	return codait
}

func (p *CodaitParser) getValues(bufReader *bufio.Reader) map[string]interface{} {
	var tmpValues map[string][]float64 = make(map[string][]float64)
	var values map[string]interface{} = make(map[string]interface{})
	i := 0
	tmpValues[CodaitKeys[i]] = []float64{}
	for {
		line, _, err := bufReader.ReadLine()
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Printf("Error: %v\n", err)
			return values
		}
		linestr := string(line)
		value := strings.Split(linestr, ",")
		if len(value) == 1 {
			i = i + 1
			if len(CodaitKeys) == i {
				break
			}
			bufReader.ReadLine() // column line
		} else {
			floatVal, _ := strconv.ParseFloat(value[0], 64)
			tmpValues[CodaitKeys[i]] = append(tmpValues[CodaitKeys[i]], floatVal)
		}
	}
	for key, tmpValue := range tmpValues {
		values[key] = tmpValue
	}

	return values
}

func (p *CodaitParser) ParseValue(body []byte) (map[string]interface{}, error) {
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
		if strings.Contains(linestr, "===") && strings.Contains(linestr, CodaitKeys[0]) {
			_, _, err = bufReader.ReadLine() // column line
			values := p.getValues(bufReader)
			return values, nil
		}
	}
	return nil, errors.New("KeyNotFound")
}

func (p *CodaitParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := CodaitKeys[2]
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
