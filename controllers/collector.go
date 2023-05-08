/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	cpe_result_metric_name   = "cpe_result_val"
	cpe_result_metric_lables = []string{
		"benchmark", "build", "config", "scenario", "job", "pod", "key", "attrbs",
	}
)

type ValueWithLabels struct {
	Labels map[string]string
	Value  float64
}
type ValuesWithLabels struct {
	Labels map[string]string
	Values []float64
}

type ResultCollector struct {
	client.Client
	Log           logr.Logger
	resultVectors *prometheus.GaugeVec
}

func (c *ResultCollector) relabelKey(key string) string {
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, " ", "_")
	key = strings.ReplaceAll(key, "(", "_")
	key = strings.ReplaceAll(key, "/", "_per_")
	key = strings.ReplaceAll(key, ")", "")
	key = strings.ReplaceAll(key, "%", "_percent")
	key = strings.ReplaceAll(key, "__", "_")
	return key
}

func (c *ResultCollector) labelMapToStr(labelMap map[string]string) string {
	keys := []string{}
	for k := range labelMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	str := ""
	for index, k := range keys {
		if index > 0 {
			str += "_"
		}
		str += fmt.Sprintf("%s_%s ", k, labelMap[k])
	}
	return strings.ToLower(str)
}

func NewCollector(client client.Client, logger logr.Logger) {
	collector := &ResultCollector{
		Client: client,
		Log:    logger,
		resultVectors: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: cpe_result_metric_name,
			Help: "CPE Results with parsed key and index if applicable",
		}, cpe_result_metric_lables),
	}
	// register prometheus
	metrics.Registry.MustRegister(collector)
}

// Describe implements the prometheus.Collector interface
func (c *ResultCollector) Describe(ch chan<- *prometheus.Desc) {
	c.resultVectors.Describe(ch)
}

func (c *ResultCollector) getStat(vals []float64) (minVal, maxVal, avgVal float64) {
	if len(vals) == 0 {
		// no values
		return -1, -1, -1
	}
	minVal = vals[0]
	maxVal = vals[0]
	var sumVal float64 = 0
	for _, val := range vals {
		if val > maxVal {
			maxVal = val
		} else if val < minVal {
			minVal = val
		}
		sumVal += val
	}
	avgVal = sumVal / float64(len(vals))
	return
}

func (c *ResultCollector) getCommonLabels(benchmarkName, build, configID, scenarioID, jobName, podName string) prometheus.Labels {
	labels := make(prometheus.Labels)
	labels["benchmark"] = benchmarkName
	labels["build"] = build
	labels["config"] = configID
	labels["scenario"] = scenarioID
	labels["job"] = jobName
	labels["pod"] = podName
	return labels
}

func (c *ResultCollector) updateGaugeVec(benchmarkName, build, configID, scenarioID, jobName, podName string, values map[string]interface{}) {
	for key, vals := range values {
		relabeledKey := c.relabelKey(key)
		if reflect.TypeOf(vals).Kind() == reflect.Float64 {
			labels := c.getCommonLabels(benchmarkName, build, configID, scenarioID, jobName, podName)
			labels["key"] = relabeledKey
			labels["attrbs"] = ""
			c.resultVectors.With(labels).Set(vals.(float64))
		} else if valueWithLabelsArr, ok := vals.([]ValueWithLabels); ok {
			for _, valueWithLabels := range valueWithLabelsArr {
				labels := c.getCommonLabels(benchmarkName, build, configID, scenarioID, jobName, podName)
				labels["key"] = relabeledKey
				labels["attrbs"] = c.labelMapToStr(valueWithLabels.Labels)
				c.resultVectors.With(labels).Set(valueWithLabels.Value)
			}
		} else if valuesWithLabelsArr, ok := vals.([]ValuesWithLabels); ok {
			for _, valuesWithLabels := range valuesWithLabelsArr {
				minVal, maxVal, avgVal := c.getStat(valuesWithLabels.Values)
				minLabels := c.getCommonLabels(benchmarkName, build, configID, scenarioID, jobName, podName)
				maxLables := c.getCommonLabels(benchmarkName, build, configID, scenarioID, jobName, podName)
				avgLables := c.getCommonLabels(benchmarkName, build, configID, scenarioID, jobName, podName)
				minLabels["key"] = relabeledKey
				maxLables["key"] = relabeledKey
				avgLables["key"] = relabeledKey
				minLabels["attrbs"] = c.labelMapToStr(valuesWithLabels.Labels) + "_min"
				maxLables["attrbs"] = c.labelMapToStr(valuesWithLabels.Labels) + "_max"
				avgLables["attrbs"] = c.labelMapToStr(valuesWithLabels.Labels) + "_avg"
				c.resultVectors.With(minLabels).Set(minVal)
				c.resultVectors.With(maxLables).Set(maxVal)
				c.resultVectors.With(avgLables).Set(avgVal)
			}
		} else if reflect.TypeOf(vals).Kind() == reflect.Slice {
			for index, val := range vals.([]interface{}) {
				if reflect.TypeOf(val).Kind() == reflect.Float64 {
					labels := c.getCommonLabels(benchmarkName, build, configID, scenarioID, jobName, podName)
					labels["key"] = relabeledKey
					labels["attrbs"] = fmt.Sprintf("%d", index)
					c.resultVectors.With(labels).Set(val.(float64))
				}
			}
		} else {
			c.Log.Info(fmt.Sprintf("Wrong key type: %v", vals))
		}
	}
}

// Collect implements the prometheus.Collector interface
// "benchmark", "build", "configID", "scenarioID", "job", "pod", "key", "index"
func (c *ResultCollector) Collect(ch chan<- prometheus.Metric) {
	benchmarks := &cpev1.BenchmarkList{}
	c.Client.List(context.TODO(), benchmarks, &client.ListOptions{
		Namespace: metav1.NamespaceAll,
	})
	c.resultVectors.Reset()
	for _, benchmark := range benchmarks.Items {
		benchmarkName := benchmark.Name
		for _, result := range benchmark.Status.Results {
			build := result.BuildID
			configID := result.ConfigurationID
			scenarioID := result.IterationID
			for _, item := range result.Items {
				values := make(map[string]interface{})
				err := json.Unmarshal([]byte(item.Result), &values)
				if err != nil {
					c.Log.Info(fmt.Sprintf("Cannot parse values of %s from respone: %s", benchmarkName, item.Result))
					continue
				}
				jobName := item.JobName
				podName := item.PodName
				c.updateGaugeVec(benchmarkName, build, configID, scenarioID, jobName, podName, values)
			}
		}
	}
	c.resultVectors.Collect(ch)
}
