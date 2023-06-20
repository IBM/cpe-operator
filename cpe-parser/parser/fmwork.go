/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type FMWorkParser struct {
	*BaseParser
}

/*
Tokens per iteration: 2

               s/iter           seqs/s         tokens/s         ms/token
MIN       0.031715076     31.530745819     63.061491639     15.857538000
MAX       1.158101110      0.863482464      1.726964928    579.050555000
AVG       0.034738420     28.786571393     57.573142786     17.369209871
MED       0.031836234     31.410750898     62.821501796     15.918116750
P95       0.032091481     31.160917510     62.321835020     16.045740625
STD       0.056241701
*/

const (
	FMWorkPerfKey = "Tokens per iteration"
)

func NewFMWorkParser() *FMWorkParser {
	fmworkParser := &FMWorkParser{}
	abs := &BaseParser{
		Parser: fmworkParser,
	}
	fmworkParser.BaseParser = abs
	return fmworkParser
}

func (p *FMWorkParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	bytesReader := bytes.NewReader(body)
	bufReader := bufio.NewReader(bytesReader)
	for {
		line, _, err := bufReader.ReadLine()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		linestr := string(line)
		if strings.Contains(linestr, FMWorkPerfKey) {
			// start from here // Tokens per iteration
			splitedLine := strings.Split(linestr, ":")
			if len(splitedLine) == 2 {
				valueStr := strings.TrimSpace(splitedLine[1])
				key := strings.TrimSpace(splitedLine[0])
				value, err := strconv.ParseFloat(valueStr, 64)
				if err == nil {
					values[key] = value
				}
			}
			// skip one line
			_, _, err := bufReader.ReadLine()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
			line, _, err := bufReader.ReadLine()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
			headers_line := string(line)
			headers := strings.Fields(headers_line)
			for {
				line, _, err := bufReader.ReadLine()
				if err == io.EOF {
					break
				} else if err != nil {
					return nil, err
				}
				linestr := string(line)
				valueStrs := strings.Fields(linestr)
				if len(valueStrs) > 1 {
					for index, valueStr := range valueStrs[1:] {
						key := fmt.Sprintf("%s %s", strings.ToLower(valueStrs[0]), headers[index])
						value, err := strconv.ParseFloat(valueStr, 64)
						if err == nil {
							values[key] = value
						}
					}
				}
			}
			// end
			break
		}
	}
	return values, nil
}

func (p *FMWorkParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	candidatePerformanceStatKeys := []string{"p95", "p90", "p99", "avg"}
	for _, performanceStatKey := range candidatePerformanceStatKeys {
		performanceKey := fmt.Sprintf("%s s/iter", performanceStatKey)
		if value, ok := values[performanceKey]; ok {
			return performanceKey, value.(float64)
		}
	}

	return "NoKey", -1
}
