/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type BenchmarkOperatorMeta struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// BenchmarkSpec defines the desired state of Benchmark
type BenchmarkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Operator      BenchmarkOperatorMeta `json:"benchmarkOperator"`
	Spec          string                `json:"benchmarkSpec"`
	IterationSpec IterationSpec         `json:"iterationSpec,omitempty"`
	Repetition    int                   `json:"repetition,omitempty"`
	JobInterval   int                   `json:"interval,omitempty"`
	ParserKey     string                `json:"parserKey,omitempty"`
	BuildConfigs  []ConfigSpec          `json:"trackBuildConfigs,omitempty"`
	Sidecar       bool                  `json:"sidecar,omitempty"`
}

// BuildConfig Definition
type ConfigSpec struct {
	Name      string `json:"name"`
	Kind      string `json:"kind,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type IterationSpec struct {
	Iteration     []IterationItem    `json:"iterations,omitempty"`
	NodeSelection *NodeSelectionSpec `json:"nodeSelection,omitempty"`
	Configuration []IterationItem    `json:"configurations,omitempty"`
	Sequential    bool               `json:"sequential,omitempty"`
	Minimize      bool               `json:"minimize,omitempty"`
}

type NodeSelectionSpec struct {
	Location       string                `json:"location"`
	TunedValues    []string              `json:"values"`
	TargetSelector *metav1.LabelSelector `json:"selector,omitempty"`
}

// Iteration Definition
type IterationItem struct {
	Name     string   `json:"name"`
	Location string   `json:"location"`
	Values   []string `json:"values,omitempty"`
}

type BenchmarkResultItem struct {
	Repetition       string `json:"run"`
	JobName          string `json:"job"`
	PodName          string `json:"pod"`
	PerformanceKey   string `json:"performanceKey"`
	PerformanceValue string `json:"performanceValue"`
	PushedTime       string `json:"pushedTime"`
}

// BemchmarkIterationHash
type IterationHash struct {
	Hash       string            `json:"hash"`
	Build      string            `json:"build"`
	Iteration  map[string]string `json:"iterations"`
	Repetition string            `json:"run"`
}

// BenchmarkPerformanceResult
type BenchmarkResult struct {
	BuildID          string                `json:"build"`
	IterationID      string                `json:"scenarioID"`
	IterationMap     map[string]string     `json:"scenarios"`
	ConfigurationID  string                `json:"configID"`
	ConfigurationMap map[string]string     `json:"configurations"`
	Items            []BenchmarkResultItem `json:"repetitions"`
}

type BenchmarkBestResult struct {
	BuildID          string            `json:"build"`
	IterationID      string            `json:"scenarioID"`
	ConfigurationMap map[string]string `json:"configurations"`
	PerformanceKey   string            `json:"performanceKey"`
	PerformanceValue string            `json:"performanceValue"`
}

// BenchmarkStatus defines the observed state of Benchmark
type BenchmarkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Hash          []IterationHash       `json:"hash,omitempty"`
	Results       []BenchmarkResult     `json:"results,omitempty"`
	BestResults   []BenchmarkBestResult `json:"bestResults,omitempty"`
	TrackedBuilds []string              `json:"builds,omitempty"`
	JobCompleted  string                `json:"jobCompleted,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Benchmark is the Schema for the benchmarks API
type Benchmark struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BenchmarkSpec   `json:"spec,omitempty"`
	Status BenchmarkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BenchmarkList contains a list of Benchmark
type BenchmarkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Benchmark `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Benchmark{}, &BenchmarkList{})
}
