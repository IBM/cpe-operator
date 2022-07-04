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

type SysbenchParser struct {
	*BaseParser
}

var SysbenchKeyMap map[string]string = map[string]string{
	"events per second:": "cpu_eps",
	"reads/s:":           "read_ops",
	"writes/s:":          "write_ops",
	"fsyncs/s:":          "fsync_ops",
	"read, MiB/s:":       "read_throughput",
	"written, MiB/s:":    "written_throughput",
	"Total operations:":  "total_operations_per_sec",
	"MiB transferred":    "transfered_mib_per_sec",
}

/*
CPU speed:
    events per second: 31970.73

File operations:
    reads/s:                      2340.89
    writes/s:                     1560.59
    fsyncs/s:                     4995.49

Throughput:
    read, MiB/s:                  36.58
    written, MiB/s:               24.38

Total operations: 87790996 (8777584.35 per second)

85733.39 MiB transferred (8571.86 MiB/sec)
*/

func NewSysbenchParser() *SysbenchParser {
	sysbenchParser := &SysbenchParser{}
	abs := &BaseParser{
		Parser: sysbenchParser,
	}
	sysbenchParser.BaseParser = abs
	return sysbenchParser
}

func (p *SysbenchParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})
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
		for keyword, valkey := range SysbenchKeyMap {
			if strings.Contains(linestr, keyword) {
				splited := strings.Fields(linestr)
				value := splited[len(splited)-1]
				if valkey == "total_operations_per_sec" {
					value = splited[2]
					fval, _ := strconv.ParseFloat(value, 64)
					values[valkey] = fval
				} else if valkey == "transfered_mib_per_sec" {
					value = splited[0]
					fval, _ := strconv.ParseFloat(value, 64)
					values[valkey] = fval
				} else {
					fval, _ := strconv.ParseFloat(value, 64)
					values[valkey] = fval
				}
			}
		}
	}
	return values, nil
}

func (p *SysbenchParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := "transfered_mib_per_sec"
	if performanceValue, ok := values[performanceKey]; !ok {
		return performanceKey, -1
	} else {
		return performanceKey, performanceValue.(float64)
	}

}
