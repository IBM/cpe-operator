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

type FioParser struct {
	*BaseParser
}

var FioKeyMap map[string]string = map[string]string{
	"bw (  KiB/s):": "bw_kib_per_sec",
	"iops        :": "iops",
	"slat (usec):":  "slat_usec",
	"clat (usec):":  "clat_usec",
	"lat (usec):":   "lat_usec",
}

/*
test: (g=0): rw=randread, bs=(R) 64.0KiB-64.0KiB, (W) 64.0KiB-64.0KiB, (T) 64.0KiB-64.0KiB, ioengine=libaio, iodepth=8
...
test: (groupid=0, jobs=1): err= 0: pid=8: Wed Aug 11 07:40:24 2021
  read: IOPS=753, BW=47.1MiB/s (49.3MB/s)(2048MiB/43515msec)
    slat (usec): min=3, max=700, avg=12.50, stdev= 8.69
    clat (usec): min=38, max=53357, avg=10609.04, stdev=1235.99
     lat (usec): min=43, max=53367, avg=10621.72, stdev=1235.43
    clat percentiles (usec):
     |  1.00th=[ 9372],  5.00th=[ 9765], 10.00th=[ 9896], 20.00th=[10290],
     | 30.00th=[10683], 40.00th=[10683], 50.00th=[10814], 60.00th=[10814],
     | 70.00th=[10814], 80.00th=[10945], 90.00th=[11076], 95.00th=[11207],
     | 99.00th=[11731], 99.50th=[13304], 99.90th=[16450], 99.95th=[20055],
     | 99.99th=[46924]
   bw (  KiB/s): min=43264, max=80000, per=100.00%, avg=48212.84, stdev=3510.18, samples=86
   iops        : min=  676, max= 1250, avg=753.33, stdev=54.85, samples=86
  lat (usec)   : 50=0.02%, 100=0.25%, 250=0.41%, 500=0.06%, 750=0.02%
  lat (msec)   : 2=0.03%, 4=0.02%, 10=12.30%, 20=86.83%, 50=0.04%
  lat (msec)   : 100=0.01%
  cpu          : usr=0.31%, sys=1.35%, ctx=32509, majf=0, minf=169
  IO depths    : 1=0.1%, 2=0.1%, 4=0.1%, 8=100.0%, 16=0.0%, 32=0.0%, >=64=0.0%
     submit    : 0=0.0%, 4=100.0%, 8=0.0%, 16=0.0%, 32=0.0%, 64=0.0%, >=64=0.0%
     complete  : 0=0.0%, 4=100.0%, 8=0.1%, 16=0.0%, 32=0.0%, 64=0.0%, >=64=0.0%
     issued rwts: total=32768,0,0,0 short=0,0,0,0 dropped=0,0,0,0
     latency   : target=0, window=0, percentile=100.00%, depth=8

Run status group 0 (all jobs):
   READ: bw=47.1MiB/s (49.3MB/s), 47.1MiB/s-47.1MiB/s (49.3MB/s-49.3MB/s), io=2048MiB (2147MB), run=43515-43515msec
*/

func NewFioParser() *FioParser {
	fioParser := &FioParser{}
	abs := &BaseParser{
		Parser: fioParser,
	}
	fioParser.BaseParser = abs
	return fioParser
}

func (p *FioParser) ParseValue(body []byte) (map[string]interface{}, error) {
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
		for keyword, valkey := range FioKeyMap {
			if strings.Contains(linestr, keyword) {
				linestr = strings.Replace(linestr, keyword, "", 1)
				splitedByComma := strings.Split(linestr, ",")
				for _, splitToken := range splitedByComma {
					splitedByEqual := strings.Split(splitToken, "=")
					statkey := valkey + "_" + strings.TrimSpace(splitedByEqual[0])
					value := strings.TrimSpace(splitedByEqual[1])
					fval, _ := strconv.ParseFloat(value, 64)
					values[statkey] = fval
				}
			}
		}
	}
	return values, nil
}

func (p *FioParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := "iops_avg"
	avg := values[performanceKey].(float64)
	return performanceKey, avg
}
