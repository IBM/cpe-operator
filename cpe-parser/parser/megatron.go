/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package parser

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

type MegatronParser struct {
	*BaseParser
}

/*
2023-10-26 21:31:16,318 [Rank 15]:  iteration       20/   20 | consumed samples:          20 | elapsed time per iteration (ms): 3874.0 | learning rate: 6.000E-06 | global batch size:    16 | lm loss: 8.777018E+00 | loss scale: 1.0 | grad norm: 12.959 | number of skipped iterations:   0 | number of nan iterations:   0 | TFLOPs: 320.33 | tokens-per-second-per-gpu: 2114.61 |
*/

const (
	MEGATRON_PERFORMANCE_KEY = "tokens-per-second-per-gpu"
)

func NewMegatronParser() *MegatronParser {
	megatronParser := &MegatronParser{}
	abs := &BaseParser{
		Parser: megatronParser,
	}
	megatronParser.BaseParser = abs
	return megatronParser
}

func (p *MegatronParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	valuesWithLabels := make(map[string][]ValueWithLabels)

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
		if strings.Contains(linestr, MEGATRON_PERFORMANCE_KEY) {
			labels := make(map[string]string)
			splited := strings.Split(linestr, "|")
			for _, col := range splited {
				if strings.Contains(col, "Rank") && strings.Contains(col, "iteration") {
					iterationFields := strings.Fields(col)
					for _, field := range iterationFields {
						if strings.Contains(field, "/") {
							iter := strings.Split(field, "/")[0]
							labels["iteration"] = iter
						}
						if strings.Contains(field, "]") {
							rank := strings.Split(field, "]")[0]
							labels["rank"] = rank
						}
					}
				} else {
					key, value, err := splitValue(col, ":")
					if err == nil {
						// trim key
						keyFields := strings.Fields(key)
						key = strings.Join(keyFields, " ")
						newValue := ValueWithLabels{
							Labels: labels,
							Value:  value,
						}
						if valueWithLabelsArr, ok := valuesWithLabels[key]; ok {
							valuesWithLabels[key] = append(valueWithLabelsArr, newValue)
						} else {
							valuesWithLabels[key] = []ValueWithLabels{newValue}
						}
					}
				}
			}

		}
	}
	for key, valueWithLabelsArr := range valuesWithLabels {
		values[key] = valueWithLabelsArr
	}
	return values, nil
}

func (p *MegatronParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	if valuesWithLabelsInterface, ok := values[MEGATRON_PERFORMANCE_KEY]; ok {
		valuesWithLabels := valuesWithLabelsInterface.([]ValueWithLabels)
		if len(valuesWithLabels) == 0 {
			return "NoValue", -1
		}
		avgValue := float64(0)
		for _, valueWithLabel := range valuesWithLabels {
			avgValue += valueWithLabel.Value
		}
		if len(valuesWithLabels) > 0 {
			avgValue /= float64(len(valuesWithLabels))
		}
		return MEGATRON_PERFORMANCE_KEY, avgValue
	}
	return "NoKey", -1
}
