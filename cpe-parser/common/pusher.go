/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package common

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/IBM/cpe-operator/cpe-parser/parser"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var pushgatewayURL string = os.Getenv("PUSHGATEWAY_URL")

func relabelKey(key string) string {
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, " ", "_")
	key = strings.ReplaceAll(key, "(", "_")
	key = strings.ReplaceAll(key, "/", "_per_")
	key = strings.ReplaceAll(key, ")", "")
	key = strings.ReplaceAll(key, "%", "_percent")
	key = strings.ReplaceAll(key, "__", "_")
	return key
}

func addSingleGauge(parserKey string, key string, value float64, labels map[string]string, gauges []prometheus.Gauge) []prometheus.Gauge {
	key = relabelKey(key)
	constLabels := make(prometheus.Labels)
	for k, v := range labels {
		constLabels[k] = v
	}

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        fmt.Sprintf("%s_%s_val", parserKey, key),
		Help:        fmt.Sprintf("value of %s", key),
		ConstLabels: constLabels,
	})
	gauge.Set(value)

	gauges = append(gauges, gauge)
	return gauges
}

func addStatGauge(parserKey string, key string, vals []float64, labels map[string]string, gauges []prometheus.Gauge) []prometheus.Gauge {
	key = relabelKey(key)
	var constLabels prometheus.Labels
	for k, v := range labels {
		constLabels[k] = v
	}

	maxVal := vals[0]
	minVal := vals[0]
	var sumVal float64 = 0
	for _, val := range vals {
		if val > maxVal {
			maxVal = val
		} else if val < minVal {
			minVal = val
		}
		sumVal += val
	}
	avgVal := sumVal / float64(len(vals))

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        fmt.Sprintf("%s_%s_min_val", parserKey, key),
		Help:        fmt.Sprintf("Minimum value of %s", key),
		ConstLabels: constLabels,
	})
	value := minVal
	gauge.Set(value)
	gauges = append(gauges, gauge)

	gauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        fmt.Sprintf("%s_%s_max_val", parserKey, key),
		Help:        fmt.Sprintf("Maximum value of %s", key),
		ConstLabels: constLabels,
	})
	value = maxVal
	gauge.Set(value)
	gauges = append(gauges, gauge)

	gauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        fmt.Sprintf("%s_%s_avg_val", parserKey, key),
		Help:        fmt.Sprintf("Average value of %s", key),
		ConstLabels: constLabels,
	})
	value = avgVal
	gauge.Set(value)
	gauges = append(gauges, gauge)
	return gauges
}

func GetGauges(parserKey string, instance string, podName string, values map[string]interface{}) []prometheus.Gauge {
	var gauges []prometheus.Gauge
	emptyMap := make(map[string]string)
	for key, vals := range values {
		if reflect.TypeOf(vals).Kind() == reflect.Float64 {
			gauges = addSingleGauge(parserKey, key, vals.(float64), emptyMap, gauges)
		} else if valueWithLabelsArr, ok := vals.([]parser.ValueWithLabels); ok {
			for _, valueWithLabels := range valueWithLabelsArr {
				gauges = addSingleGauge(parserKey, key, valueWithLabels.Value, valueWithLabels.Labels, gauges)
			}
		} else if valuesWithLabelsArr, ok := vals.([]parser.ValuesWithLabels); ok {
			for _, valuesWithLabels := range valuesWithLabelsArr {
				gauges = addStatGauge(parserKey, key, valuesWithLabels.Values, valuesWithLabels.Labels, gauges)
			}
		} else if reflect.TypeOf(vals).Kind() == reflect.Slice {
			if floatVals, ok := vals.([]float64); ok {
				gauges = addStatGauge(parserKey, key, floatVals, emptyMap, gauges)
			} else {
				fmt.Println("Cannot convert slice: ", vals)
			}
		} else {
			fmt.Println("Wrong key type: ", vals)
		}
	}
	return gauges
}

func PushValues(parserKey string, clusterID string, instance string, benchmarkName string, jobName string, podName string, const_labels map[string]string, values map[string]interface{}) error {
	if pushgatewayURL == "" {
		return fmt.Errorf("No PUSHGATEWAY_URL set")
	}

	gauges := GetGauges(parserKey, instance, podName, values)
	completionTime := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "completion_timestamp_seconds",
		Help: "The timestamp of the last completion.",
	})
	completionTime.SetToCurrentTime()

	pusher := push.New(pushgatewayURL, jobName)
	for _, gauge := range gauges {
		pusher.Collector(gauge)
	}

	pusher.Collector(completionTime)

	pusher = pusher.Grouping("benchmark", benchmarkName)
	pusher = pusher.Grouping("cluster", clusterID)
	// pusher = pusher.Grouping("instance", instance)
	// pusher = pusher.Grouping("sourcepod", podName)

	for key, value := range const_labels {
		fmt.Printf("Label %s:%s\n", key, value)
		pusher = pusher.Grouping(key, value)
	}

	err := pusher.Push()

	return err
}
