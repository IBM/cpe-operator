/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package controllers

///////////////////////////////////////////////////////////////////////////
//
// tuned.go
//
// ApplyProfile
// - add profile label to the node selected by selector (called before start job)
// DeleteLabel
// - delete profile label from the node (called after job done)
//
////////////////////////////////////////////////////////////////////////////

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	TUNED_RESOURCE      = "tuneds.v1.tuned.openshift.io"
	RENDERED_TUNED_NAME = "rendered"
	TUNED_NAMESPACE     = "openshift-cluster-node-tuning-operator"
	PROFILE_RESOURCE    = "profiles.v1.tuned.openshift.io"
	TUNED_KIND          = "Tuned"

	RESERVED_PRIORITY_NUMBER        = 0
	RESERVED_AUTOTUNED_PROFILE_NAME = "auto-tuned"
	BASE_PROFILE                    = "openshift-default"
)

type TunedHandler struct {
	*kubernetes.Clientset
	Log logr.Logger
	DYN dynamic.Interface
}

func (t *TunedHandler) checkProfileExist(profileName string) bool {
	gvr, _ := schema.ParseResourceArg(TUNED_RESOURCE)

	rendered_item, err := t.DYN.Resource(*gvr).Namespace(TUNED_NAMESPACE).Get(context.TODO(), profileName, metav1.GetOptions{})
	if err != nil {
		t.Log.Info(fmt.Sprintf("Cannot get tuned %s/%s: %v", profileName, TUNED_NAMESPACE, err))
	} else {
		profiles := rendered_item.Object["spec"].(map[string]interface{})[NODESELECT_ITR_NAME].([]interface{})
		for _, profile := range profiles {
			if profileName == profile.(map[string]interface{})["name"].(string) {
				return true
			}
		}
	}
	return false
}

func (t *TunedHandler) checkIfProfileApplied(nodes []string, profileName string) bool {
	gvr, _ := schema.ParseResourceArg(PROFILE_RESOURCE)
	for _, nodeName := range nodes {
		nodeProfile, err := t.DYN.Resource(*gvr).Namespace(TUNED_NAMESPACE).Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			t.Log.Info(fmt.Sprintf("Cannot get profile %s/%s: %v", nodeName, TUNED_NAMESPACE, err))
			continue // to avoid loop
		} else {
			tunedProfile := nodeProfile.Object["spec"].(map[string]interface{})["config"].(map[string]interface{})["tunedProfile"].(string)
			if tunedProfile != profileName {
				t.Log.Info(fmt.Sprintf("Profile for %s hasn't update yet actual: %s expect: %s", nodeName, tunedProfile, profileName))
				return false
			}
		}
	}
	return true // all applied
}

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func (t *TunedHandler) getNodeList(nodeSelector *metav1.LabelSelector) *corev1.NodeList {
	var selectOptions metav1.ListOptions
	if nodeSelector != nil {
		labelMap, _ := metav1.LabelSelectorAsMap(nodeSelector)
		selectOptions = metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelMap).String(),
		}
	} else {
		selectOptions = metav1.ListOptions{}
	}
	nodes, _ := t.Clientset.CoreV1().Nodes().List(context.TODO(), selectOptions)
	return nodes
}

func (t *TunedHandler) ApplyProfile(nodeSelector *metav1.LabelSelector, profileName string) {
	nodes := t.getNodeList(nodeSelector)
	var labeledNodes []string
	for _, node := range nodes.Items {
		nodeName := node.ObjectMeta.Name
		payload := []patchStringValue{{
			Op:    "replace",
			Path:  fmt.Sprintf("/metadata/labels/%s", NODESELECT_ITR_NAME),
			Value: profileName,
		}}
		payloadBytes, _ := json.Marshal(payload)

		_, err := t.Clientset.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			t.Log.Info(fmt.Sprintf("Cannot patch label to node %s: %v", nodeName, err))
		} else {
			t.Log.Info(fmt.Sprintf("Label node %s: %s=%s", nodeName, NODESELECT_ITR_NAME, profileName))
			labeledNodes = append(labeledNodes, nodeName)
		}
	}
	if t.checkProfileExist(profileName) {
		// Running a loop until profile updated > risk for infinite loop
		// for {
		// 	time.Sleep(1 * time.Second)
		// 	if t.checkIfProfileApplied(labeledNodes, profileName) {
		// 		t.Log.Info(fmt.Sprintf("Node %v set %s=%s", labeledNodes, NODESELECT_ITR_NAME, profileName))
		// 		break
		// 	}
		// 	t.Log.Info(fmt.Sprintf("Wait for %v to apply %s ...", labeledNodes, profileName))
		// }
		// Running only one check
		time.Sleep(10 * time.Second)
		if t.checkIfProfileApplied(labeledNodes, profileName) {
			t.Log.Info(fmt.Sprintf("Node %v set %s=%s", labeledNodes, NODESELECT_ITR_NAME, profileName))
		} else {
			t.Log.Info(fmt.Sprintf("Node %v: %s is not applied yet", labeledNodes, profileName))
		}
	} else {
		t.Log.Info(fmt.Sprintf("Profile %s not exists", profileName))
	}
}

func (t *TunedHandler) DeleteLabel(nodeSelector *metav1.LabelSelector) {
	nodes := t.getNodeList(nodeSelector)
	for _, node := range nodes.Items {
		nodeName := node.ObjectMeta.Name
		payload := []patchStringValue{{
			Op:   "remove",
			Path: fmt.Sprintf("/metadata/labels/%s", NODESELECT_ITR_NAME),
		}}
		payloadBytes, _ := json.Marshal(payload)

		_, err := t.Clientset.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
		if err != nil {
			t.Log.Info(fmt.Sprintf("Cannot remove label of node %s: %v", nodeName, err))
		}
	}

}

func GetDataProfile(tunedProfile map[TuneType]map[string]string) string {
	data := "[main]\n"
	data += "summary=auto-generated profile\n"
	data += fmt.Sprintf("include=%s\n", BASE_PROFILE)
	for tuneType, configKV := range tunedProfile {
		data += fmt.Sprintf("[%s]\n", tuneType)
		for key, value := range configKV {
			data += fmt.Sprintf("%s=%s\n", key, value)
		}
	}
	return data
}

func GetAutoTunedProfile(tunedProfile map[TuneType]map[string]string) *unstructured.Unstructured {
	object := make(map[string]interface{})
	gvr, _ := schema.ParseResourceArg(TUNED_RESOURCE)
	object["apiVersion"] = gvr.Group + "/" + gvr.Version
	object["kind"] = TUNED_KIND
	object["metadata"] = map[string]interface{}{
		"name":      RESERVED_AUTOTUNED_PROFILE_NAME,
		"namespace": TUNED_NAMESPACE,
	}
	spec := make(map[string]interface{})
	spec["profile"] = []interface{}{
		map[string]interface{}{
			"data": GetDataProfile(tunedProfile),
			"name": RESERVED_AUTOTUNED_PROFILE_NAME,
		},
	}
	spec["recommend"] = []interface{}{
		map[string]interface{}{
			"match": []interface{}{
				map[string]interface{}{
					"label": NODESELECT_ITR_NAME,
					"value": RESERVED_AUTOTUNED_PROFILE_NAME,
				},
			},
			"priority": RESERVED_PRIORITY_NUMBER,
			"profile":  RESERVED_AUTOTUNED_PROFILE_NAME,
			"operand": map[string]interface{}{
				"debug": false,
			},
		},
	}
	object["spec"] = spec
	profile := &unstructured.Unstructured{
		Object: object,
	}
	return profile
}

func (t *TunedHandler) CreateAutoTunedProfile(tunedProfile map[TuneType]map[string]string) error {
	gvr, _ := schema.ParseResourceArg(TUNED_RESOURCE)
	profile := GetAutoTunedProfile(tunedProfile)
	t.Log.Info(fmt.Sprintf("Tuned Profile: %v", tunedProfile))
	_, err := t.DYN.Resource(*gvr).Namespace(TUNED_NAMESPACE).Create(context.TODO(), profile, metav1.CreateOptions{})
	return err
}

func (t *TunedHandler) DeleteAutoTunedProfile() error {
	gvr, _ := schema.ParseResourceArg(TUNED_RESOURCE)
	return t.DYN.Resource(*gvr).Namespace(TUNED_NAMESPACE).Delete(context.TODO(), RESERVED_AUTOTUNED_PROFILE_NAME, metav1.DeleteOptions{})
}

func (t *TunedHandler) IsSameProfile(tunedProfile map[TuneType]map[string]string, cmpTunedProfile map[TuneType]map[string]string) bool {
	return reflect.DeepEqual(tunedProfile, cmpTunedProfile)
}
