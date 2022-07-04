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

type Iperf3Parser struct {
	*BaseParser
}

/*
[  5]   0.00-60.00  sec  17.4 GBytes  2.49 Gbits/sec   44             sender
[  5]   0.00-60.04  sec  17.4 GBytes  2.49 Gbits/sec                  receiver
*/

func NewIperf3Parser() *Iperf3Parser {
	iperf3Parser := &Iperf3Parser{}
	abs := &BaseParser{
		Parser: iperf3Parser,
	}
	iperf3Parser.BaseParser = abs
	return iperf3Parser
}

func (p *Iperf3Parser) getKey(subject string, metric string, unit string, postfix string) string {
	mainKey := fmt.Sprintf("%s_%s_%s", subject, metric, unit)
	if postfix != "" {
		return mainKey + "_" + postfix
	}
	return mainKey
}

func (p *Iperf3Parser) ParseValue(body []byte) (map[string]interface{}, error) {
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
		found := false
		var subject string
		if strings.Contains(linestr, "sender") {
			found = true
			subject = "sender"
		} else if strings.Contains(linestr, "receiver") {
			found = true
			subject = "receiver"
		}

		if found {
			splited := strings.Fields(linestr)
			total_bytes, _ := strconv.ParseFloat(splited[4], 64)
			bytes_unit := splited[5]
			avg_bps, _ := strconv.ParseFloat(splited[6], 64)
			bps_unit := strings.Split(splited[7], "/")[0]
			total_bytes_key := p.getKey(subject, "total", bytes_unit, "")
			avg_bps_key := p.getKey(subject, "avg", bps_unit, "ps")
			values[total_bytes_key] = total_bytes
			values[avg_bps_key] = avg_bps
		}
	}
	return values, nil
}

func (p *Iperf3Parser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	subject := "receiver"
	performancePartialKey := p.getKey(subject, "avg", "", "")
	for performanceKey, _ := range values {
		if strings.Contains(performanceKey, performancePartialKey) {
			avg := values[performanceKey].(float64)
			if strings.Contains(performanceKey, "Gbits") {
				avg *= 1000.0
			}
			return p.getKey(subject, "avg", "Mbits", "ps"), avg
		}
	}
	return p.getKey(subject, "avg", "Mbits", "ps"), -1
}
