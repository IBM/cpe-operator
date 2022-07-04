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

type GromacsParser struct {
	*BaseParser
}

var GromacsKeyMap map[string]string = map[string]string{
	"Time:":        "time",
	"Performance:": "performance",
}

/*
               Core t (s)   Wall t (s)        (%)
       Time:     3224.991      806.248      400.0
                 (ns/day)    (hour/ns)
Performance:       10.717        2.240
*/

func NewGromacsParser() *GromacsParser {
	gromacsParser := &GromacsParser{}
	abs := &BaseParser{
		Parser: gromacsParser,
	}
	gromacsParser.BaseParser = abs
	return gromacsParser
}

func (p *GromacsParser) getColumn(prevline string) []string {
	columns := []string{}
	splits := strings.Split(prevline, ")")
	for _, split := range splits {
		split = strings.TrimSpace(split)
		if split != "" {
			columns = append(columns, split)
		}
	}
	return columns
}

func (p *GromacsParser) ParseValue(body []byte) (map[string]interface{}, error) {
	tmpValues := make(map[string]float64)
	values := make(map[string]interface{})
	bytesReader := bytes.NewReader(body)
	bufReader := bufio.NewReader(bytesReader)
	prevline := ""
	for {
		line, _, err := bufReader.ReadLine()
		linestr := string(line)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		for keyword, valkey := range GromacsKeyMap {
			if strings.Contains(linestr, keyword) {
				splited := strings.Fields(linestr)
				columns := p.getColumn(prevline)
				for index, column := range columns {
					fullkey := fmt.Sprintf("%s_%s", valkey, column)
					fval, _ := strconv.ParseFloat(splited[1+index], 64)
					tmpValues[fullkey] = fval
				}
			}
		}
		prevline = linestr
	}
	for key, tmpValue := range tmpValues {
		values[key] = tmpValue
	}
	return values, nil
}

func (p *GromacsParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := ""
	var performanceValue float64 = -1
	for key, value := range values {
		if strings.Contains(key, "performance") && strings.Contains(key, "ns/") {
			performanceKey = strings.ReplaceAll(key, "performance_", "")
			performanceKey = strings.ReplaceAll(performanceKey, "(", "")
			performanceValue = value.(float64)
		}
	}
	return performanceKey, performanceValue
}
