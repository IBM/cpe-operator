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
	"strconv"
	"strings"
)

type TimeParser struct {
	*BaseParser
}

/*
  # OSU MPI
  # Size          Latency (us)
*/

const (
	RealTimeKey = "real"
	UserTimeKey = "user"
	SysTimeKey  = "sys"

	performanceKey = "milliseconds"
)

func NewTimeParser() *TimeParser {
	timeParser := &TimeParser{}
	abs := &BaseParser{
		Parser: timeParser,
	}
	timeParser.BaseParser = abs
	return timeParser
}

func (p *TimeParser) parseTime(key, linestr string) (bool, int) {
	match, err := regexp.MatchString(fmt.Sprintf("%s([\t\n\f\r ]+)([0-9]+(.)*([0-9])*)m([0-9]+(.)*([0-9])*)s", ""), linestr)
	if err == nil && match {
		values := strings.Fields(linestr)
		if len(values) == 2 {
			value := values[1]
			mSplit := strings.Split(value[0:len(value)-1], "m")
			minute, _ := strconv.ParseFloat(mSplit[0], 64)
			second, _ := strconv.ParseFloat(mSplit[1], 64)
			milliseconds := (second + minute*60) * 1000
			return true, int(milliseconds)
		}
	}
	return false, -1
}

func (p *TimeParser) ParseValue(body []byte) (map[string]interface{}, error) {
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
		match, value := p.parseTime(RealTimeKey, linestr)
		if match {
			realTime := value
			// next line
			line, _, err = bufReader.ReadLine()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
			linestr = string(line)
			match, value = p.parseTime(UserTimeKey, linestr)
			if !match {
				// something wrong
				continue
			}
			userTime := value
			// next line
			line, _, err = bufReader.ReadLine()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
			linestr = string(line)
			match, value = p.parseTime(SysTimeKey, linestr)
			if !match {
				// something wrong
				continue
			}
			sysTime := value
			labels := map[string]string{
				"user": fmt.Sprintf("%d", userTime),
				"sys":  fmt.Sprintf("%d", sysTime),
			}
			valueWithLabels := ValueWithLabels{
				Labels: labels,
				Value:  float64(realTime),
			}
			values[performanceKey] = valueWithLabels
			break
		}
	}
	return values, nil
}

func (p *TimeParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	if value, ok := values[performanceKey]; ok {
		return performanceKey, value.(ValueWithLabels).Value
	}
	return "NoKey", -1
}
