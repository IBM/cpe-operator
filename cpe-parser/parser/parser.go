/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 package parser

type ValueWithLabels struct {
	Labels map[string]string
	Value  float64
}
type ValuesWithLabels struct {
	Labels map[string]string
	Values []float64
}

type Parser interface {
	ParseValue([]byte) (map[string]interface{}, error)
	GetPerformanceValue(map[string]interface{}) (string, float64)
}

type BaseParser struct {
	Parser
}

func (*BaseParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	values["SimpleValue"] = []float64{1.0}
	return values, nil
}

func (*BaseParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	for performanceKey, _ := range values {
		performanceValues := values[performanceKey].([]float64)
		var sum float64
		sum = 0
		for _, val := range performanceValues {
			sum += val
		}
		return performanceKey, sum / float64(len(performanceValues))
	}
	return "NoKey", -1
}
