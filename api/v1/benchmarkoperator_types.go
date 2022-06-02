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

type HelmSpec struct {
	Entity     string `json:"entity"`
	Release    string `json:"release,omitempty"`
	RepoName   string `json:"repoName"`
	URL        string `json:"url"`
	Username   string `json:"user,omitempty"`
	Password   string `json:"password,omitempty"`
	ValuesYaml string `json:"valuesYaml,omitempty"`
}

type YAMLSpec struct {
	Host  string   `json:"host"`
	Paths []string `json:"paths,omitempty"`
}

type DeploymentSpec struct {
	Namespace string   `json:"namespace,omitempty"`
	YAML      YAMLSpec `json:"yaml,omitempty"`
	Helm      HelmSpec `json:"helm,omitempty"`
}

// BenchmarkOperatorSpec defines the desired state of BenchmarkOperator
type BenchmarkOperatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Adaptor    string         `json:"adaptor,omitempty"`
	CRD        YAMLSpec       `json:"crd,omitempty"`
	DeploySpec DeploymentSpec `json:"deploySpec"`
}

// BenchmarkOperatorStatus defines the observed state of BenchmarkOperator
type BenchmarkOperatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BenchmarkOperator is the Schema for the benchmarkoperators API
type BenchmarkOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BenchmarkOperatorSpec   `json:"spec,omitempty"`
	Status BenchmarkOperatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BenchmarkOperatorList contains a list of BenchmarkOperator
type BenchmarkOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BenchmarkOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BenchmarkOperator{}, &BenchmarkOperatorList{})
}
