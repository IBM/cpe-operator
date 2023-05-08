/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package controllers

///////////////////////////////////////////////////////////////////////////
//
// job_tracker.go
//
// JobTrackManager manages (add/delete) JobTracker component for each job resource
//
// JobTracker watch update on job resource to check completeness (refers to adaptor)
//	- putLog - put the log of completed pods to the COS
//  - parseAndPush - call parser to parse and push the prometheus-format metric to push gateway
//  - updateBenchmarkStatus - update results to benchmark and find best result
//  - deployWaitingResource - deploy iterated job resource in the waiting list
//
////////////////////////////////////////////////////////////////////////////

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type JobTrackManager struct {
	client.Client
	*kubernetes.Clientset
	JobTrackers map[string]*JobTracker
	Cos         COSObject
	GlobalQuit  chan struct{}
	Log         logr.Logger
	DC          *discovery.DiscoveryClient
	DYN         dynamic.Interface
	*TunedHandler
}

const JOB_MAX_QSIZE = 100

func (m *JobTrackManager) NewTracker(jobGVK schema.GroupVersionKind, benchmarkName string, waitingJob []*unstructured.Unstructured, dr dynamic.ResourceInterface, adaptor OperatorAdaptor, jobOptMap map[string]*BaysesOptimizer) {
	jobGVKString := jobGVK.String()
	quit := make(chan struct{})
	if _, exist := m.JobTrackers[jobGVKString]; !exist || m.JobTrackers[jobGVKString] == nil {
		m.Log.Info(fmt.Sprintf("New Job Tracker %s", jobGVKString))
		jobQueue := make(chan *unstructured.Unstructured, JOB_MAX_QSIZE)
		newJobMap := make(map[string][]*unstructured.Unstructured)
		newDRMap := make(map[string]dynamic.ResourceInterface)
		var subscribers []string

		m.JobTrackers[jobGVKString] = &JobTracker{
			Client:         m.Client,
			Clientset:      m.Clientset,
			Log:            m.Log,
			DC:             m.DC,
			DYN:            m.DYN,
			JobQueue:       jobQueue,
			Quit:           quit,
			JobGVK:         jobGVK,
			Cos:            m.Cos,
			WaitingJobMap:  newJobMap,
			DRMap:          newDRMap,
			Adaptor:        adaptor,
			TunedHandler:   m.TunedHandler,
			Subscribers:    subscribers,
			JobOptMap:      make(map[string]*BaysesOptimizer),
			BestPodNameMap: make(map[string]string),
		}

		m.JobTrackers[jobGVKString].Init()
		go m.JobTrackers[jobGVKString].Run()
	}
	m.JobTrackers[jobGVKString].Subscribe(benchmarkName, waitingJob, dr, jobOptMap)
}

func (m *JobTrackManager) IsExist(jobGVK schema.GroupVersionKind, benchmarkName string) bool {
	jobGVKString := jobGVK.String()
	if _, exist := m.JobTrackers[jobGVKString]; !exist || m.JobTrackers[jobGVKString] == nil {
		return false
	}
	return m.JobTrackers[jobGVKString].IsExist(benchmarkName)
}

func (m *JobTrackManager) DeleteTracker(jobGVK schema.GroupVersionKind, benchmarkName string) {
	jobGVKString := jobGVK.String()

	if _, exist := m.JobTrackers[jobGVKString]; exist {
		isEmpty := true
		if m.JobTrackers[jobGVKString] != nil {
			m.JobTrackers[jobGVKString].Unsubscribe(benchmarkName)
			isEmpty = m.JobTrackers[jobGVKString].IsEmpty()
			if isEmpty {
				m.JobTrackers[jobGVKString].End()
				m.Log.Info(fmt.Sprintf("End %s", jobGVKString))
			}
		}
		if isEmpty {
			delete(m.JobTrackers, jobGVKString)
		}
	}

}

func (m *JobTrackManager) Run() {
	<-m.GlobalQuit
	for jobGVKString, tracker := range m.JobTrackers {
		tracker.End()
		delete(m.JobTrackers, jobGVKString)
		m.Log.Info(fmt.Sprintf("End %s", jobGVKString))
	}
}

type JobTracker struct {
	client.Client
	*kubernetes.Clientset
	Log            logr.Logger
	DC             *discovery.DiscoveryClient
	DYN            dynamic.Interface
	JobQueue       chan *unstructured.Unstructured
	Quit           chan struct{}
	JobGVK         schema.GroupVersionKind
	Cos            COSObject
	Subscribers    []string
	WaitingJobMap  map[string][]*unstructured.Unstructured
	DRMap          map[string]dynamic.ResourceInterface
	Adaptor        OperatorAdaptor
	JobOptMap      map[string]*BaysesOptimizer
	BestPodNameMap map[string]string
	*TunedHandler
}

func (r *JobTracker) Run() {
	wait.Until(r.ProcessJobQueue, 0, r.Quit)
	close(r.JobQueue)
}

func (r *JobTracker) IsExist(benchmarkName string) bool {
	index := r.indexOf(benchmarkName)
	return index != -1 && index < len(r.Subscribers)
}

func (r *JobTracker) copyInstance(finishedInstance *unstructured.Unstructured) *unstructured.Unstructured {
	existObj := finishedInstance.DeepCopy().Object
	existMetadata := existObj["metadata"].(map[string]interface{})

	newObj := make(map[string]interface{})
	newObj["apiVersion"] = existObj["apiVersion"]
	newObj["kind"] = existObj["kind"]
	newObj["spec"] = existObj["spec"]
	newObj["metadata"] = map[string]interface{}{"name": existMetadata["name"], "namespace": existMetadata["namespace"], "labels": existMetadata["labels"]}

	return &unstructured.Unstructured{
		Object: newObj,
	}
}

func (r *JobTracker) deployWaitingResource(finishedInstance *unstructured.Unstructured, benchmark *cpev1.Benchmark) {
	benchmarkName := benchmark.GetName()
	dr := r.DRMap[benchmarkName]

	// try deploy from auto-tuning first
	if nodeTunedOptimizer, ok := r.JobOptMap[finishedInstance.GetName()]; ok {

		if benchmark.Spec.JobInterval > 0 {
			r.Log.Info(fmt.Sprintf("Wait %d seconds before creating next job of %s.", benchmark.Spec.JobInterval, benchmarkName))
			time.Sleep(time.Duration(benchmark.Spec.JobInterval) * time.Second)
		}

		if !nodeTunedOptimizer.FinalizedApplied {
			copiedInstance := r.copyInstance(finishedInstance)
			err, isNew := CreateIfNotExists(dr, benchmark, copiedInstance, r.Adaptor, r.TunedHandler, nodeTunedOptimizer)
			if err == nil && isNew {
				r.Log.Info(fmt.Sprintf("Continue auto-tuning for %s", finishedInstance.GetName()))
				return
			} else {
				r.Log.Info(fmt.Sprintf("Cannot create auto-tuned job %s: %v", finishedInstance.GetName(), err))
			}
		}

		if nodeTunedOptimizer.FinalizedApplied {
			r.Log.Info(fmt.Sprintf("Delete optimizer for %s", finishedInstance.GetName()))
			delete(r.JobOptMap, finishedInstance.GetName())
		}

		if _, ok := r.WaitingJobMap[benchmarkName]; ok {
			var nextInstance *unstructured.Unstructured
			nextInstance, r.WaitingJobMap[benchmarkName] = r.WaitingJobMap[benchmarkName][0], r.WaitingJobMap[benchmarkName][1:]
			r.Log.Info(fmt.Sprintf("Deploy resource: %s (%d waiting)", nextInstance.GetName(), len(r.WaitingJobMap[benchmarkName])))

			if nodeTunedOptimizer, ok = r.JobOptMap[nextInstance.GetName()]; ok {
				err, _ := CreateIfNotExists(dr, benchmark, nextInstance, r.Adaptor, r.TunedHandler, nodeTunedOptimizer)
				if err != nil {
					r.Log.Info(fmt.Sprintf("Cannot create #%v: %s", err, nextInstance.GetName()))
				}
				if len(r.WaitingJobMap[benchmarkName]) == 0 {
					delete(r.WaitingJobMap, benchmarkName)
					if nodeTunedOptimizer.FinalizedApplied {
						delete(r.DRMap, benchmarkName)
						r.Log.Info(fmt.Sprintf("Delete dynamic interface of %s", benchmarkName))
					}
					r.Log.Info(fmt.Sprintf("No more in waiting list: %s", benchmarkName))
				}
			} else {
				r.Log.Info(fmt.Sprintf("No job %s in the map %v", nextInstance.GetName(), r.JobOptMap))
			}
		} else {
			delete(r.DRMap, benchmarkName)
			r.Log.Info(fmt.Sprintf("No more in waiting list: %s", benchmarkName))
		}
	} else {
		r.Log.Info(fmt.Sprintf("No job %s in the map %v", finishedInstance.GetName(), r.JobOptMap))
	}
}

func (r *JobTracker) ProcessJobQueue() {
	job := <-r.JobQueue
	jobObject := job.Object

	jobMeta := jobObject["metadata"].(map[string]interface{})
	jobLabels := jobMeta["labels"].(map[string]interface{})

	benchmarkName := jobLabels[BENCHMARK_LABEL].(string)

	benchmark := &cpev1.Benchmark{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Name: benchmarkName, Namespace: job.GetNamespace()}, benchmark)
	if err != nil {
		r.Log.Info(fmt.Sprintf("Cannot get benchmark #%v ", err))
		return
	}

	parserKey := benchmark.Spec.ParserKey
	constLabels := make(map[string]string)
	for _, item := range benchmark.Spec.IterationSpec.Iteration {
		constLabels[item.Name] = jobLabels[item.Name].(string)
	}
	for _, item := range benchmark.Spec.IterationSpec.Configuration {
		constLabels[item.Name] = jobLabels[item.Name].(string)
	}
	if _, ok := jobLabels[NODESELECT_ITR_NAME]; ok {
		constLabels[NODESELECT_ITR_NAME] = jobLabels[NODESELECT_ITR_NAME].(string)
	}

	jobName := jobMeta["name"].(string)
	jobNamespace := jobMeta["namespace"].(string)

	podLogOpts := corev1.PodLogOptions{}

	valid := false
	podList, err := r.Adaptor.GetPodList(jobObject, r.Clientset)
	if err != nil {
		r.Log.Info(fmt.Sprintf("Cannot list pod from selector #%v ", err))
	}

	for index, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodSucceeded {
			continue
		}
		valid = true
		req := r.Clientset.CoreV1().Pods(jobNamespace).GetLogs(pod.Name, &podLogOpts)
		podLogs, err := req.Stream(context.TODO())
		if err != nil {
			r.Log.Info(fmt.Sprintf("Cannot stream log #%v ", err))
		}
		defer podLogs.Close()
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		if err != nil {
			r.Log.Info(fmt.Sprintf("Cannot copy I/O #%v ", err))
		} else {
			podName := pod.Name
			keyName := fmt.Sprintf("%s/%s/%s/%s.log", benchmarkName, CLUSTER_ID, jobName, podName)
			instance := pod.Status.HostIP

			// keep previous log for auto-tuned
			var previousExist bool
			var bestPodName string
			var prevResponse Response
			var bestLogErr error
			var prevValue float64

			if bestPodName, previousExist = r.BestPodNameMap[jobName]; previousExist {
				prevResponse, bestLogErr = parseLog(benchmarkName, jobName, bestPodName, parserKey)
				if prevResponse.Status == "OK" {
					prevValue = prevResponse.PerformanceValue
				}
				r.Log.Info(fmt.Sprintf("Previous Response %s:  %v, %v, %.2f", bestPodName, prevResponse, err, prevValue))
			}

			r.Log.Info(fmt.Sprintf("PutLog: %s: %s", benchmarkName, keyName))
			logBytes, _ := ioutil.ReadAll(buf)
			putLogErr := putLog(r.Cos, keyName, logBytes)

			if parserKey != "" {
				r.Log.Info(fmt.Sprintf("Job: %s Call Parser: %s", jobName, parserKey))
				var response Response
				if putLogErr != nil {
					r.Log.Info(fmt.Sprintf("PutLog Error #%v, parse raw log", err))
					response, err = parseRawLog(parserKey, logBytes)
				} else {
					r.Log.Info("Parse remote put log")
					response, err = parseAndPushLog(instance, benchmarkName, jobName, podName, parserKey, constLabels)
				}
				r.Log.Info(fmt.Sprintf("Response: %v", response))

				if index == 0 {
					// return result to job queue
					if nodeTunedOptimizer, ok := r.JobOptMap[jobName]; ok {
						if !nodeTunedOptimizer.FinalizedReady {
							nodeTunedOptimizer.ResultQueue <- response.PerformanceValue
						}
						if previousExist && bestLogErr == nil && !r.isBetterResult(benchmark, prevValue, response.PerformanceValue) {
							// if not better, use best response and keep BestPodNameMap as it is
							r.Log.Info(fmt.Sprintf("Replace with previous result %s (%.2f) -> %s (%.2f)", podName, response.PerformanceValue, bestPodName, prevResponse.PerformanceValue))
							response = prevResponse
							podName = bestPodName
						} else {
							// else record new best pod
							r.BestPodNameMap[jobName] = pod.Name
						}
					}

					if err != nil {
						r.Log.Info(fmt.Sprintf("ParseAndPushLog Error #%v ", err))
					} else {
						if nodeTunedOptimizer, ok := r.JobOptMap[jobName]; ok {
							if nodeTunedOptimizer.FinalizedApplied {
								r.updateBenchmarkStatus(benchmark, jobName, podName, response)
								delete(r.BestPodNameMap, jobName)
							}
						} else {
							r.updateBenchmarkStatus(benchmark, jobName, podName, response)
							delete(r.BestPodNameMap, jobName)
						}
					}
				}
			}

		}
	}
	if valid {
		// delete all pod if got result
		for _, pod := range podList.Items {
			r.Clientset.CoreV1().Pods(jobNamespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		}

		// handler tuned profile
		// try deploy auto-tuned sample queue first
		nodeSelectionSpec := benchmark.Spec.IterationSpec.NodeSelection
		if nodeSelectionSpec != nil && r.TunedHandler != nil {
			r.TunedHandler.DeleteLabel(nodeSelectionSpec.TargetSelector)
		}

		r.deployWaitingResource(r.Adaptor.CopyJobResource(job), benchmark)
	}
}

func (r *JobTracker) updateBenchmarkStatus(benchmark *cpev1.Benchmark, jobName string, podName string, response Response) {
	performanceKey := response.PerformanceKey
	pushedTime := time.Now().String()
	pvalInString := fmt.Sprintf("%f", response.PerformanceValue)

	_, iterationMap, configurationMap, repetition, buildID, iterationID, configurationID := GetDetailFromJobName(jobName, benchmark)

	if nodeTunedOptimizer, ok := r.JobOptMap[jobName]; ok {
		tunedData := GetDataProfile(nodeTunedOptimizer.FinalizedTunedProfile)
		if nodeTunedOptimizer.AutoTuned {
			labeledTunedStr := fmt.Sprintf("[job]\n%s\n[samples]\n%d\n%s", jobName, nodeTunedOptimizer.SamplingCount, tunedData)
			configurationMap[RESERVED_AUTOTUNED_PROFILE_NAME] = labeledTunedStr
		}
	} else {
		r.Log.Info(fmt.Sprintf("Cannot find optimizer for %s", jobName))
	}

	resultItem := cpev1.BenchmarkResultItem{
		Repetition:       repetition,
		PerformanceKey:   performanceKey,
		PerformanceValue: pvalInString,
		Result:           response.Message,
		JobName:          jobName,
		PodName:          podName,
		PushedTime:       pushedTime,
	}

	results := benchmark.Status.Results
	found := false
	avgPerformanceValue := response.PerformanceValue
	for index, existResult := range results {
		if existResult.BuildID == buildID && existResult.IterationID == iterationID && existResult.ConfigurationID == configurationID {
			found = true
			for _, existResultItem := range existResult.Items {
				existValue, _ := strconv.ParseFloat(existResultItem.PerformanceValue, 64)
				avgPerformanceValue = avgPerformanceValue + existValue
			}
			avgPerformanceValue = avgPerformanceValue / float64(len(existResult.Items)+1)
			if prevTunedData, tuneExists := benchmark.Status.Results[index].ConfigurationMap[RESERVED_AUTOTUNED_PROFILE_NAME]; tuneExists {
				benchmark.Status.Results[index].ConfigurationMap[RESERVED_AUTOTUNED_PROFILE_NAME] = fmt.Sprintf("%s\n%s", prevTunedData, configurationMap[RESERVED_AUTOTUNED_PROFILE_NAME])
			}
			benchmark.Status.Results[index].Items = append(existResult.Items, resultItem)
			break
		}
	}
	if !found {
		newResult := cpev1.BenchmarkResult{
			BuildID:          buildID,
			IterationID:      iterationID,
			IterationMap:     iterationMap,
			ConfigurationID:  configurationID,
			ConfigurationMap: configurationMap,
			Items:            []cpev1.BenchmarkResultItem{resultItem},
		}
		benchmark.Status.Results = append(benchmark.Status.Results, newResult)
	}

	bestResults := benchmark.Status.BestResults
	avgPvalInString := fmt.Sprintf("%f", avgPerformanceValue)
	candidateBestResult := cpev1.BenchmarkBestResult{
		BuildID:          buildID,
		IterationID:      iterationID,
		ConfigurationMap: configurationMap,
		PerformanceKey:   performanceKey,
		PerformanceValue: avgPvalInString,
	}

	// compare best result
	isBetter := true
	matchIndex := -1

	for index, oldBestResult := range bestResults {
		if iterationID == oldBestResult.IterationID && buildID == oldBestResult.BuildID && performanceKey == oldBestResult.PerformanceKey {
			matchIndex = index
			oldValue, _ := strconv.ParseFloat(oldBestResult.PerformanceValue, 64)
			isBetter = r.isBetterResult(benchmark, oldValue, avgPerformanceValue)
			break
		}
	}
	if isBetter {
		if matchIndex < 0 {
			// new value
			bestResults = append(bestResults, candidateBestResult)
		} else {
			bestResults[matchIndex] = candidateBestResult
		}
	}

	benchmark.Status.BestResults = bestResults
	benchmark.Status.JobCompleted = GetJobCompletedStatus(benchmark)
	err := r.Client.Status().Update(context.Background(), benchmark)

	if err != nil {
		r.Log.Info(fmt.Sprintf("Cannot update #%v ", err))
	}

}

// is val2 better than val1
func (r *JobTracker) isBetterResult(benchmark *cpev1.Benchmark, val1 float64, val2 float64) bool {
	if (!benchmark.Spec.IterationSpec.Minimize && val2 <= val1) || (benchmark.Spec.IterationSpec.Minimize && val2 >= val1) {
		return false
	}
	return true
}

func (r *JobTracker) checkComplete(jobObject map[string]interface{}) bool {
	return r.Adaptor.CheckComplete(jobObject)
}

func (r *JobTracker) Init() {

	s, factory := GetInformerFromGVK(r.DC, r.DYN, r.JobGVK)

	handlers := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldInstance, instance interface{}) {
			job := instance.(*unstructured.Unstructured)
			jobObject := job.Object
			jobMeta := jobObject["metadata"].(map[string]interface{})
			jobLabels := jobMeta["labels"].(map[string]interface{})

			jobName := jobMeta["name"].(string)

			if _, exist := jobLabels[BENCHMARK_LABEL]; exist {
				r.Log.Info(fmt.Sprintf("Job on update %s - %v %v", jobName, r.checkComplete(jobObject), jobObject["status"]))
				if r.checkComplete(jobObject) {
					oldJobObject := oldInstance.(*unstructured.Unstructured).Object
					if !r.checkComplete(oldJobObject) {
						r.Log.Info(fmt.Sprintf("Add %s to job queue", jobName))
						r.JobQueue <- job
					}
				}
			}
		},
	}

	s.AddEventHandler(handlers)
	factory.Start(r.Quit)

}

func (r *JobTracker) indexOf(benchmarkName string) int {
	index := -1
	for index = 0; index < len(r.Subscribers); index++ {
		if r.Subscribers[index] == benchmarkName {
			break
		}
	}
	return index
}

func (r *JobTracker) Subscribe(benchmarkName string, waitingJob []*unstructured.Unstructured, dr dynamic.ResourceInterface, jobOptMap map[string]*BaysesOptimizer) {
	index := r.indexOf(benchmarkName)
	if index == -1 || index == len(r.Subscribers) {
		r.Subscribers = append(r.Subscribers, benchmarkName)
		r.Log.Info(fmt.Sprintf("%s subscribed: %d subscribing, (%d wait)", benchmarkName, len(r.Subscribers), len(waitingJob)))
		if len(waitingJob) > 0 {
			r.WaitingJobMap[benchmarkName] = waitingJob
			r.DRMap[benchmarkName] = dr
			for k, v := range jobOptMap {
				r.JobOptMap[k] = v
			}
		}
	} else {
		r.Log.Info(fmt.Sprintf("%s already subscribed", benchmarkName))
	}
}

func (r *JobTracker) Unsubscribe(benchmarkName string) {
	index := r.indexOf(benchmarkName)
	if index < len(r.Subscribers) {
		if index == len(r.Subscribers)-1 {
			if len(r.Subscribers) == 1 {
				r.Subscribers = []string{}
			} else {
				r.Subscribers = r.Subscribers[:index]
			}
		} else if index == 0 {
			r.Subscribers = r.Subscribers[index+1:]
		} else {
			r.Subscribers = append(r.Subscribers[:index], r.Subscribers[index+1:]...)
		}
		r.Log.Info(fmt.Sprintf("%s unsubscribed: %d left", benchmarkName, len(r.Subscribers)))
		if _, found := r.WaitingJobMap[benchmarkName]; found {
			r.Log.Info(fmt.Sprintf("%d waiting job of %s will never deploy", len(r.WaitingJobMap[benchmarkName]), benchmarkName))
			delete(r.WaitingJobMap, benchmarkName)
			delete(r.DRMap, benchmarkName)
		}
	} else {
		r.Log.Info(fmt.Sprintf("%s cannot found", benchmarkName))
	}
}

func (r *JobTracker) IsEmpty() bool {
	return len(r.Subscribers) == 0
}

func (r *JobTracker) End() {
	close(r.Quit)
}
