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

type LinpackParser struct {
	*BaseParser
}

const (
	LINPACK_STARTER_KEY = "T/V"
)

func NewLinpackParser() *LinpackParser {
	linpackParser := &LinpackParser{}
	abs := &BaseParser{
		Parser: linpackParser,
	}
	linpackParser.BaseParser = abs
	return linpackParser
}

func (p *LinpackParser) isTargetLine(linestr string) bool {
	if strings.Contains(linestr, LINPACK_STARTER_KEY) {
		if !strings.Contains(linestr, ":") {
			return true
		}
	}
	return false
}

func (p *LinpackParser) ParseValue(body []byte) (map[string]interface{}, error) {
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

		if p.isTargetLine(linestr) {
			colSplits := strings.Fields(linestr)
			bufReader.ReadLine() // free line
			line, _, err = bufReader.ReadLine()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
			linestr = string(line)
			valSplits := strings.Fields(linestr)
			for index, col := range colSplits[5:len(colSplits)] {
				fval, _ := strconv.ParseFloat(valSplits[index+5], 64)
				values[col] = fval
			}
			break
		}
	}
	return values, nil
}

func (p *LinpackParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := ""
	var performanceValue float64 = -1
	for key, value := range values {
		if strings.Contains(key, "flops") {
			performanceKey = key
			performanceValue = value.(float64)
			break
		}
	}
	return performanceKey, performanceValue
}
