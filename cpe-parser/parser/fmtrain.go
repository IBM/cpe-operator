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
	"strings"
	"time"
)

const (
	fmtrainCheckpointStartKey  = "Started to load checkpoint"
	fmtrainCheckpointEndKey    = "Ended to load checkpoint"
	fmtrainDataPrepareStartKey = "Started to prepare data"
	fmtrainDataPrepareEndKey   = "Datasets constructed"
	fmtrainTrainStartKey       = "Beginning training"
	fmtrainTrainEndKey         = "Writing"
	fmtrainEndKey              = "Job Complete!"
)

type FMTrainParser struct {
	*BaseParser
}

/*
2023-07-27 16:31:18 Started to load checkpoint
2023-07-27 16:31:18 Ended to load checkpoint
2023-07-27 16:31:18 Started to prepare data
2023-07-27 16:31:19 Datasets constructed!
2023-27-2023 16:31:19 Beginning training! If using a large dataset w/o aggressive caching, may take ~1 min per 20GB before starting.

step = 10
trainloss = 10.766671752929687
speed = 1.0044547319412231

step = 20
trainloss = 9.861251831054688
speed = 0.6618351697921753
...

step = 1000
trainloss = 6.763322448730468
speed = 0.6619813680648804
2023-07-27 16:42:26 Writing final checkpoint
step = 1000
2023-07-27 16:42:28 Training ended.
Job Complete!
total_hours = 0.1876769094996982

*/

func NewFMTrainParser() *FMTrainParser {
	fmtrainParser := &FMTrainParser{}
	abs := &BaseParser{
		Parser: fmtrainParser,
	}
	fmtrainParser.BaseParser = abs
	return fmtrainParser
}

func (p *FMTrainParser) getTimestamp(linestr string) string {
	splits := strings.Fields(linestr)
	if len(splits) < 2 {
		return ""
	}
	dateString := fmt.Sprintf("%s %s", splits[0], splits[1])
	return dateString
}

func (p *FMTrainParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	valuesWithLabels := make(map[string][]ValueWithLabels)
	bytesReader := bytes.NewReader(body)
	bufReader := bufio.NewReader(bytesReader)
	var trainStart bool

	stepLabel := make(map[string]string)
	for {
		line, _, err := bufReader.ReadLine()
		linestr := string(line)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if strings.Contains(linestr, fmtrainCheckpointStartKey) {
			// set checkpointStart
			stepLabel["checkpointStart"] = p.getTimestamp(linestr)
		} else if strings.Contains(linestr, fmtrainCheckpointEndKey) {
			// set checkpointEnd
			stepLabel["checkpointEnd"] = p.getTimestamp(linestr)
		} else if strings.Contains(linestr, fmtrainDataPrepareStartKey) {
			// set dataPrepareStart
			stepLabel["dataPrepareStart"] = p.getTimestamp(linestr)
		} else if strings.Contains(linestr, fmtrainDataPrepareEndKey) {
			// set dataPrepareEnd
			stepLabel["dataPrepareEnd"] = p.getTimestamp(linestr)
		} else if strings.Contains(linestr, fmtrainTrainStartKey) {
			// set trainStart
			stepLabel["trainStart"] = p.getTimestamp(linestr)
			trainStart = true
		} else if strings.Contains(linestr, fmtrainEndKey) {
			// job completed
			// read next line
			line, _, err := bufReader.ReadLine()
			linestr := string(line)
			if err == io.EOF {
				break
			} else if err == nil {
				// expect total_hours key
				key, value, err := splitValue(linestr, defaultDelimit)
				if err == nil {
					values[key] = value
				}
			}
		} else if strings.Contains(linestr, fmtrainTrainEndKey) {
			dateString := p.getTimestamp(linestr)
			layout := "2006-01-02 15:04:05"
			timestamp, err := time.Parse(layout, dateString)
			if err == nil {
				values["trainEnd"] = float64(timestamp.Unix())
			}
			trainStart = false
		} else if trainStart {
			key, value, err := splitValue(linestr, defaultDelimit)
			if err == nil {
				// trainStepValues
				if strings.Contains(linestr, "step") {
					stepLabel[key] = fmt.Sprintf("%f", value)
				} else {
					copyLabels := make(map[string]string)
					for labelKey, labelValue := range stepLabel {
						copyLabels[labelKey] = labelValue
					}
					newValue := ValueWithLabels{
						Labels: copyLabels,
						Value:  value,
					}
					if valueWithLabelsArr, ok := valuesWithLabels[key]; ok {
						valuesWithLabels[key] = append(valueWithLabelsArr, newValue)
					} else {
						valuesWithLabels[key] = []ValueWithLabels{newValue}
					}
				}
			}
		}
	}
	for key, valueWithLabelsArr := range valuesWithLabels {
		values[key] = valueWithLabelsArr
	}
	return values, nil
}

func (p *FMTrainParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {
	performanceKey := "trainloss"
	if valuesWithLabelsInterface, ok := values[performanceKey]; ok {
		valuesWithLabels := valuesWithLabelsInterface.([]ValueWithLabels)
		if len(valuesWithLabels) == 0 {
			return "NoValue", -1
		}
		lastValue := valuesWithLabels[len(valuesWithLabels)-1]
		return performanceKey, lastValue.Value
	}
	return "NoKey", -1
}
