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

type OSUParser struct {
	*BaseParser
}

/*
# OSU MPI
# Size          Latency (us)
*/

const (
	OSUKey          = "# OSU MPI"
	LabelName       = "Size"
	PerformanceSize = "1048576" // 1MB
)

func NewOSUParser() *OSUParser {
	osuParser := &OSUParser{}
	abs := &BaseParser{
		Parser: osuParser,
	}
	osuParser.BaseParser = abs
	return osuParser
}

func (p *OSUParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	var valueWithLabelsArr []ValueWithLabels
	bytesReader := bytes.NewReader(body)
	bufReader := bufio.NewReader(bytesReader)
	started := false
	var labelName, key string

	for {
		line, _, err := bufReader.ReadLine()
		linestr := string(line)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if !started {
			if strings.Contains(linestr, OSUKey) {
				started = true
			}
		} else {
			splited := strings.Fields(linestr)
			if strings.Contains(linestr, "#") {
				labelName = splited[1]
				key = splited[2]
				for _, append_key := range splited[3:len(splited)] {
					key += append_key
				}
			} else {
				if len(splited) == 2 {
					value, _ := strconv.ParseFloat(splited[1], 64)
					labels := map[string]string{
						labelName: splited[0],
					}
					valueWithLabels := ValueWithLabels{
						Labels: labels,
						Value:  value,
					}
					valueWithLabelsArr = append(valueWithLabelsArr, valueWithLabels)
				}
			}
		}
	}
	values[key] = valueWithLabelsArr
	return values, nil
}

func (p *OSUParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	for performanceKey, _ := range values {
		performanceValues := values[performanceKey].([]ValueWithLabels)
		for _, performanceValue := range performanceValues {
			if performanceValue.Labels[LabelName] == PerformanceSize {
				return performanceKey, performanceValue.Value
			}
		}
		return performanceKey, -1
	}
	return "NoKey", -1
}
