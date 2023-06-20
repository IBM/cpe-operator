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

type MlPerfParser struct {
	*BaseParser
}

/*
================================================
MLPerf Results Summary
================================================
SUT name : PySUT
Scenario : Offline
Mode : PerformanceOnly
Samples per second: 1196.99
Result is : INVALID
Min duration satisfied : NO
Min queries satisfied : Yes
Early stopping satisfied: Yes
Recommendations:
* Increase expected QPS so the loadgen pre-generates a larger (coalesced) query.

*/

const (
	MlPerfSummarySession  = "MLPerf Results Summary"
	AdditionalStatSession = "Additional Stats"
)

func NewMlPerfParser() *MlPerfParser {
	mlperfParser := &MlPerfParser{}
	abs := &BaseParser{
		Parser: mlperfParser,
	}
	mlperfParser.BaseParser = abs
	return mlperfParser
}

func (p *MlPerfParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	session := ""

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
		if session == "" {
			if strings.Contains(linestr, MlPerfSummarySession) {
				session = MlPerfSummarySession
			} else if strings.Contains(linestr, AdditionalStatSession) {
				session = AdditionalStatSession
			}
			if session != "" {
				_, _, err := bufReader.ReadLine()
				if err == io.EOF {
					break
				} else if err != nil {
					return nil, err
				}
			}
		} else {
			if strings.Contains(linestr, BIGLINE) {
				session = ""
				continue
			}
			splitedLine := strings.Split(linestr, ":")
			if len(splitedLine) != 2 {
				continue
			}
			valueStr := strings.TrimSpace(splitedLine[1])
			key := strings.TrimSpace(splitedLine[0])
			value, err := strconv.ParseFloat(valueStr, 64)
			if err == nil {
				values[key] = value
			}
		}
	}
	return values, nil
}

func (p *MlPerfParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := "Samples per second"
	if value, ok := values[performanceKey]; ok {
		return performanceKey, value.(float64)
	}
	return "NoKey", -1
}
