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

type GlooParser struct {
	*BaseParser
}

/*
 # OSU MPI
 # Size          Latency (us)
*/

const (
	GLOO_PERFORMANCE_KEY = "bandwidth_GBps"
)

func NewGlooParser() *GlooParser {
	glooParser := &GlooParser{}
	abs := &BaseParser{
		Parser: glooParser,
	}
	glooParser.BaseParser = abs
	return glooParser
}

func (p *GlooParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	bytesReader := bytes.NewReader(body)
	bufReader := bufio.NewReader(bytesReader)
	var sizeLabel, eleLabel string
	var metricLabels []string
	linecount := 0
	for {
		line, _, err := bufReader.ReadLine()
		linestr := string(line)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if linecount < 2 {
			if strings.Contains(linestr, BIGLINE) {
				linecount += 1
			}
		} else {
			linestr = strings.TrimSpace(linestr)
			if strings.Contains(linestr, "size") {
				linestr = strings.ReplaceAll(linestr, " (", "_")
				linestr = strings.ReplaceAll(linestr, ")", "")
				linestr = strings.ReplaceAll(linestr, "/", "p")
				splited := strings.Fields(linestr)
				sizeLabel = splited[0]
				eleLabel = splited[1]
				metricLabels = splited[2:]
			} else {
				splited := strings.Fields(linestr)
				if len(splited) > 3 {
					labels := map[string]string{
						sizeLabel: splited[0],
						eleLabel:  splited[1],
					}
					for index, valueStr := range splited[2:] {
						value, _ := strconv.ParseFloat(valueStr, 64)
						valueWithLabels := ValueWithLabels{
							Labels: labels,
							Value:  value,
						}
						key := metricLabels[index]
						var valueWithLabelsArr []ValueWithLabels
						if _, exists := values[key]; exists {
							valueWithLabelsArr = values[key].([]ValueWithLabels)
						}
						values[key] = append(valueWithLabelsArr, valueWithLabels)
					}
				}
			}
		}
	}
	return values, nil
}

func (p *GlooParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceValues := values[GLOO_PERFORMANCE_KEY].([]ValueWithLabels)
	maxValue := float64(0)
	for _, value := range performanceValues {
		if value.Value > maxValue {
			maxValue = value.Value
		}
	}
	return GLOO_PERFORMANCE_KEY, maxValue
}
