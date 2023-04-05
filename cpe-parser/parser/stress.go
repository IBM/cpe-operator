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
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type StressParser struct {
	*BaseParser
}

/*
// stress-ng: info:  [1] stressor       bogo ops real time  usr time  sys time   bogo ops/s   bogo ops/s
// stress-ng: info:  [1]                           (secs)    (secs)    (secs)   (real time) (usr+sys time)
// stress-ng: info:  [1] cpu                4597      4.00     16.00      0.00      1147.95       287.31
*/

const (
	StressKey = "stressor"
)

func NewStressParser() *StressParser {
	stressParser := &StressParser{}
	abs := &BaseParser{
		Parser: stressParser,
	}
	stressParser.BaseParser = abs
	return stressParser
}

func (p *StressParser) parseColumn(columnLine1, columnLine2 string) []string {
	names := strings.Fields(columnLine1)
	re := regexp.MustCompile(`\([^(\n]+\)`)
	units := re.FindAllString(columnLine2, -1)
	columns := []string{}
	column := ""
	unitIndex := 0
	for _, name := range names {
		if strings.Contains(name, "ops") || strings.Contains(name, "time") {
			column = fmt.Sprintf("%s %s", column, name)
			if name != "ops" && unitIndex < len(units) {
				// has unit
				unit := units[unitIndex]
				unit = strings.ReplaceAll(unit, "+", "/")
				if strings.Contains(unit, "time") {
					// add in front
					column = fmt.Sprintf("%s %s", unit, column)
				} else {
					// add in back
					column = fmt.Sprintf("%s %s", column, unit)
				}
				unitIndex += 1
			}
			columns = append(columns, column)
		} else {
			column = name
		}
	}
	return columns
}

func (p *StressParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	bytesReader := bytes.NewReader(body)
	bufReader := bufio.NewReader(bytesReader)
	started := false
	prefix := ""
	var columns []string
	for {
		line, _, err := bufReader.ReadLine()
		linestr := string(line)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if !started {
			if strings.Contains(linestr, StressKey) {
				started = true
				splited := strings.Split(linestr, StressKey)
				prefix = splited[0]
				columnLine1 := splited[1]
				line, _, err := bufReader.ReadLine()
				if err == io.EOF {
					break
				} else if err != nil {
					return nil, err
				}
				// next coming line
				columnLine2 := string(line)
				columns = p.parseColumn(columnLine1, columnLine2)
			}
		} else {
			if strings.Contains(linestr, prefix) {
				splited := strings.Fields(linestr[len(prefix) : len(linestr)-1])
				if len(splited) == len(columns)+1 { // first split is stressor
					for index, column := range columns {
						value, err := strconv.ParseFloat(splited[index+1], 64)
						if err == nil {
							key := fmt.Sprintf("%s %s", splited[0], column)
							values[key] = value
						}
					}
				}
			}
		}
	}
	return values, nil
}

func (p *StressParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	keys := make([]string, len(values))
	i := 0
	for key := range values {
		keys[i] = key
		i += 1
	}
	fmt.Println(keys)
	if len(keys) > 0 {
		sort.Strings(keys)
		return keys[0], values[keys[0]].(float64)
	}
	return "NoKey", -1
}
