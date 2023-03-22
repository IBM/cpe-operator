/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package controllers

///////////////////////////////////////////////////////////////////////////
//
// operator_adaptor.go
//
// This is an abstract class for defining the function for
// - CheckComplete - checking that the job is completed from the job resource's status
//   (default job resource is batch/Job)
// - GetPodList - to define matching rule from job to pod
//
////////////////////////////////////////////////////////////////////////////

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

type OperatorAdaptor interface {
	CheckComplete(jobObject map[string]interface{}) bool
	GetPodList(jobObject map[string]interface{}, clientset *kubernetes.Clientset) (*corev1.PodList, error)
	CopyJobResource(originalJob *unstructured.Unstructured) *unstructured.Unstructured
}

// Base Operartor Adaptor
type BaseOperatorAdaptor struct {
	OperatorAdaptor
}

// Default Operartor Adaptor
type DefaultAdaptor struct {
	*BaseOperatorAdaptor
}

func NewDefaultAdaptor() *DefaultAdaptor {
	defaultAdaptor := &DefaultAdaptor{}
	abs := &BaseOperatorAdaptor{
		OperatorAdaptor: defaultAdaptor,
	}
	defaultAdaptor.BaseOperatorAdaptor = abs
	return defaultAdaptor
}

func (a *DefaultAdaptor) CheckComplete(jobObject map[string]interface{}) bool {
	jobStatus := jobObject["status"].(map[string]interface{})
	if jobStatus["conditions"] == nil {
		return false
	}
	jobConditions := jobStatus["conditions"].([]interface{})
	for _, condition := range jobConditions {
		conditionMap := condition.(map[string]interface{})
		conditionType := conditionMap["type"].(string)
		if conditionType == "Complete" {
			status := conditionMap["status"].(string)
			if status == "True" {
				return true
			}
		}
	}
	return false
}

func (a *DefaultAdaptor) GetPodList(jobObject map[string]interface{}, clientset *kubernetes.Clientset) (*corev1.PodList, error) {
	jobMeta := jobObject["metadata"].(map[string]interface{})
	jobName := jobMeta["name"].(string)
	jobNamespace := jobMeta["namespace"].(string)
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
		Limit:         100,
	}

	podList, err := clientset.CoreV1().Pods(jobNamespace).List(context.TODO(), listOptions)
	return podList, err
}

func (a *DefaultAdaptor) CopyJobResource(originalJob *unstructured.Unstructured) *unstructured.Unstructured {
	jobObject := originalJob.Object
	benchmarkObj := make(map[string]interface{})

	benchmarkObj["apiVersion"] = jobObject["apiVersion"]
	benchmarkObj["kind"] = jobObject["kind"]
	benchmarkObj["spec"] = jobObject["spec"].(map[string]interface{})
	labels := jobObject["metadata"].(map[string]interface{})["labels"].(map[string]interface{})

	// delete auto-generated selector on Job
	metadata := jobObject["metadata"].(map[string]interface{})
	benchmarkObj["metadata"] = map[string]interface{}{"name": metadata["name"], "namespace": metadata["namespace"], "labels": labels}
	if _, ok := benchmarkObj["spec"].(map[string]interface{})["template"]; ok {
		if _, ok = benchmarkObj["spec"].(map[string]interface{})["template"].(map[string]interface{})["metadata"]; ok {
			delete(benchmarkObj["spec"].(map[string]interface{})["template"].(map[string]interface{}), "metadata")
		}
	}
	if _, ok := benchmarkObj["spec"].(map[string]interface{})["selector"]; ok {
		delete(benchmarkObj["spec"].(map[string]interface{}), "selector")
	}
	return &unstructured.Unstructured{
		Object: benchmarkObj,
	}
}

// Ripsaw Operartor Adaptor
type RipsawAdaptor struct {
	*BaseOperatorAdaptor
}

func NewRipsawAdaptor() *RipsawAdaptor {
	ripsawAdaptor := &RipsawAdaptor{}
	abs := &BaseOperatorAdaptor{
		OperatorAdaptor: ripsawAdaptor,
	}
	ripsawAdaptor.BaseOperatorAdaptor = abs
	return ripsawAdaptor
}

func (a *RipsawAdaptor) CheckComplete(jobObject map[string]interface{}) bool {
	jobStatus := jobObject["status"].(map[string]interface{})
	if jobStatus["state"] != nil {
		if jobStatus["state"].(string) == "Complete" {
			return true
		}
	}
	return false
}

func (a *RipsawAdaptor) GetPodList(jobObject map[string]interface{}, clientset *kubernetes.Clientset) (*corev1.PodList, error) {
	jobSpec := jobObject["spec"].(map[string]interface{})
	workload := jobSpec["workload"].(map[string]interface{})
	jobPrefix := workload["name"].(string)
	jobStatus := jobObject["status"].(map[string]interface{})
	jobUUID := jobStatus["suuid"].(string)
	jobMeta := jobObject["metadata"].(map[string]interface{})
	jobNamespace := jobMeta["namespace"].(string)

	listOptions := metav1.ListOptions{}

	podList, err := clientset.CoreV1().Pods(jobNamespace).List(context.TODO(), listOptions)
	if err != nil {
		return podList, err
	}
	var podItems []corev1.Pod
	for _, pod := range podList.Items {
		if strings.Contains(pod.GetName(), jobUUID) && strings.Contains(pod.GetName(), jobPrefix) {
			podItems = append(podItems, pod)
		}
	}
	sublist := &corev1.PodList{
		Items:    podItems,
		TypeMeta: podList.TypeMeta,
	}
	return sublist, err
}

func (a *RipsawAdaptor) CopyJobResource(originalJob *unstructured.Unstructured) *unstructured.Unstructured {
	return originalJob.DeepCopy()
}

// MPI Operartor Adaptor
type MPIAdaptor struct {
	*BaseOperatorAdaptor
}

func NewMPIAdaptor() *MPIAdaptor {
	mpiAdaptor := &MPIAdaptor{}
	abs := &BaseOperatorAdaptor{
		OperatorAdaptor: mpiAdaptor,
	}
	mpiAdaptor.BaseOperatorAdaptor = abs
	return mpiAdaptor
}

func (a *MPIAdaptor) CheckComplete(jobObject map[string]interface{}) bool {
	jobStatus := jobObject["status"].(map[string]interface{})
	if jobStatus["conditions"] == nil {
		return false
	}
	jobConditions := jobStatus["conditions"].([]interface{})
	for _, condition := range jobConditions {
		conditionMap := condition.(map[string]interface{})
		conditionType := conditionMap["reason"].(string)
		if strings.Contains(conditionType, "JobSucceeded") {
			status := conditionMap["status"].(string)
			if status == "True" {
				return true
			}
		}
	}
	return false
}

func (a *MPIAdaptor) CopyJobResource(originalJob *unstructured.Unstructured) *unstructured.Unstructured {
	return originalJob.DeepCopy()
}

func (a *MPIAdaptor) GetPodList(jobObject map[string]interface{}, clientset *kubernetes.Clientset) (*corev1.PodList, error) {
	jobMeta := jobObject["metadata"].(map[string]interface{})
	jobNamespace := jobMeta["namespace"].(string)

	jobPrefix := jobMeta["name"].(string) + "-launcher"

	listOptions := metav1.ListOptions{}

	podList, err := clientset.CoreV1().Pods(jobNamespace).List(context.TODO(), listOptions)
	if err != nil {
		return podList, err
	}
	var podItems []corev1.Pod
	for _, pod := range podList.Items {
		if strings.Contains(pod.GetName(), jobPrefix) {
			podItems = append(podItems, pod)
		}
	}
	sublist := &corev1.PodList{
		Items:    podItems,
		TypeMeta: podList.TypeMeta,
	}
	return sublist, err
}

// Kubeflow Operartor Adaptor
type KubeflowAdaptor struct {
	*BaseOperatorAdaptor
}

func NewKubeflowAdaptor() *KubeflowAdaptor {
	kubeflowAdaptor := &KubeflowAdaptor{}
	abs := &BaseOperatorAdaptor{
		OperatorAdaptor: kubeflowAdaptor,
	}
	kubeflowAdaptor.BaseOperatorAdaptor = abs
	return kubeflowAdaptor
}

func (a *KubeflowAdaptor) CheckComplete(jobObject map[string]interface{}) bool {
	jobStatus := jobObject["status"].(map[string]interface{})
	if jobStatus["conditions"] == nil {
		return false
	}
	jobConditions := jobStatus["conditions"].([]interface{})
	for _, condition := range jobConditions {
		conditionMap := condition.(map[string]interface{})
		conditionType := conditionMap["reason"].(string)
		if strings.Contains(conditionType, "JobSucceeded") {
			status := conditionMap["status"].(string)
			if status == "True" {
				return true
			}
		}
	}
	return false
}

func (a *KubeflowAdaptor) CopyJobResource(originalJob *unstructured.Unstructured) *unstructured.Unstructured {
	return originalJob.DeepCopy()
}

func (a *KubeflowAdaptor) GetPodList(jobObject map[string]interface{}, clientset *kubernetes.Clientset) (*corev1.PodList, error) {
	jobMeta := jobObject["metadata"].(map[string]interface{})
	jobNamespace := jobMeta["namespace"].(string)

	jobName := jobMeta["name"].(string)
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
		Limit:         100,
	}

	return clientset.CoreV1().Pods(jobNamespace).List(context.TODO(), listOptions)
}

var defaultAdaptor OperatorAdaptor = NewDefaultAdaptor()
var ripsawAdaptor OperatorAdaptor = NewRipsawAdaptor()
var mpiAdaptor OperatorAdaptor = NewMPIAdaptor()
var kubeflowAdaptor OperatorAdaptor = NewKubeflowAdaptor()

var OperatorAdaptorMap map[string]OperatorAdaptor = map[string]OperatorAdaptor{
	"default":  defaultAdaptor,
	"ripsaw":   ripsawAdaptor,
	"mpi":      mpiAdaptor,
	"kubeflow": kubeflowAdaptor,
}
