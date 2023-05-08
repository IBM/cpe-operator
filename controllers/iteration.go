/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package controllers

///////////////////////////////////////////////////////////////////
//
// iteration.go
// This module is to facilitate on generating a list of spec object
//
// There are three main function.
// - GetInitCombination: return combination of base spec object
// - GetAllCombination: return all combinations of all iteration items in a list form
// - UpdateValue: return new modified spec object regarding a new value at a specified location
//
///////////////////////////////////////////////////////////////////

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	cpev1 "github.com/IBM/cpe-operator/api/v1"
)

type IterationKeyType int

const (
	VALUE IterationKeyType = iota
	LIST
	MAP
	NULL_VALUE_STR = "nil"
)

type IterationHandler struct{}

func (it *IterationHandler) getKeyType(key string) IterationKeyType {
	if !strings.Contains(key, "[") {
		return VALUE
	}
	if strings.Contains(key, "=") {
		return MAP
	}
	return LIST
}

func (it *IterationHandler) splitBracket(key string) (string, string) {
	start := strings.Index(key, "[")
	end := strings.Index(key, "]")
	return key[0:start], key[start+1 : end]
}

func (it *IterationHandler) getValue(nextKey string, object map[string]interface{}) (IterationKeyType, interface{}, string, int) {
	keyType := it.getKeyType(nextKey)
	var nextObject interface{}
	var keyName string
	var indexStr string
	var ok bool
	indexInt := -1
	switch keyType {
	case VALUE:
		keyName = nextKey
		if nextObject, ok = object[nextKey]; !ok {
			nextObject = nil
		}
	case LIST:
		keyName, indexStr = it.splitBracket(nextKey)
		if _, ok = object[keyName]; !ok {
			object[keyName] = []interface{}{}
		}
		objectList := object[keyName].([]interface{})
		indexInt, _ = strconv.Atoi(indexStr)
		if indexInt < len(objectList) {
			nextObject = objectList[indexInt]
		} else {
			nextObject = nil
		}

	case MAP:
		keyName, indexStr = it.splitBracket(nextKey)
		subMapIndex := strings.Split(indexStr, "=")
		if _, ok = object[keyName]; !ok {
			object[keyName] = []interface{}{}
		}
		subMaps := object[keyName].([]interface{})
		nextObject = nil
		indexInt = 0
		for _, subMap := range subMaps {
			key := subMapIndex[0]
			if value, ok := subMap.(map[string]interface{})[key]; ok && value.(string) == subMapIndex[1] {
				nextObject = subMap
				break
			}
			indexInt += 1
		}
	default:
		nextObject = nil
	}
	return keyType, nextObject, keyName, indexInt
}

// Update Value

func (it *IterationHandler) getValueInterface(reflectType reflect.Type, value string) interface{} {
	if reflectType == nil {
		return value
	}
	var valueInterface interface{}

	if reflectType.Kind() == reflect.Int64 {
		valueInterface, _ = strconv.ParseInt(value, 10, 64)
	} else if reflectType.Kind() == reflect.Float64 {
		valueInterface, _ = strconv.ParseFloat(value, 64)
	} else if reflectType.Kind() == reflect.Bool {
		valueInterface, _ = strconv.ParseBool(value)
	} else {
		valueInterface = value
	}
	return valueInterface
}

func (it *IterationHandler) updateNextValue(object map[string]interface{}, keys []string, curIndex int, valueToSet string) map[string]interface{} {

	curKey := keys[curIndex]
	keyType, nextObject, keyName, indexInt := it.getValue(curKey, object)
	if nextObject == nil {
		// last valueToSet
		if curIndex == len(keys)-1 {
			var objectList []interface{}
			switch keyType {
			case VALUE:
				reflectType := reflect.TypeOf(object[curKey])
				valueInterface := it.getValueInterface(reflectType, valueToSet)
				object[curKey] = valueInterface
			case LIST:
				objectList = object[keyName].([]interface{})
				reflectType := reflect.TypeOf(objectList).Elem()
				valueInterface := it.getValueInterface(reflectType, valueToSet)
				object[keyName] = append(objectList, valueInterface)
			case MAP:
				objectList = object[keyName].([]interface{})
				newObject := make(map[string]interface{})
				_, indexStr := it.splitBracket(curKey)
				subMapIndex := strings.Split(indexStr, "=")
				newObject[subMapIndex[0]] = subMapIndex[1]
				valueKey := keys[curIndex+1]
				reflectType := reflect.TypeOf(newObject[valueKey])
				valueInterface := it.getValueInterface(reflectType, valueToSet)
				newObject[valueKey] = valueInterface
				object[keyName] = append(objectList, newObject)
			}
		} else {
			// not last value but no value before
			switch keyType {
			case VALUE:
				nextObject = make(map[string]interface{})
			case LIST:
				nextObject = []interface{}{}
			case MAP:
				nextObject = make(map[string]interface{})
				_, indexStr := it.splitBracket(curKey)
				subMapIndex := strings.Split(indexStr, "=")
				reflectType := reflect.TypeOf(subMapIndex[1])
				valueInterface := it.getValueInterface(reflectType, subMapIndex[1])
				nextObject.(map[string]interface{})[subMapIndex[0]] = valueInterface
			}
		}
	}

	if nextObject != nil {

		if reflect.TypeOf(nextObject).Kind() == reflect.Map {
			subObject := it.updateNextValue(nextObject.(map[string]interface{}), keys, curIndex+1, valueToSet)
			switch keyType {
			case VALUE:
				object[keyName] = subObject
			case LIST:
				objectArr := object[keyName].([]interface{})
				if indexInt == len(objectArr) {
					object[keyName] = []interface{}{subObject}
				} else {
					object[keyName].([]interface{})[indexInt] = subObject
				}
			case MAP:
				objectArr := object[keyName].([]interface{})
				if indexInt >= len(objectArr) {
					object[keyName] = append(objectArr, subObject)
				} else {
					object[keyName].([]interface{})[indexInt] = subObject
				}
			}
		} else {
			reflectType := reflect.TypeOf(object[keyName])
			if reflectType == reflect.TypeOf([]interface{}{}) {
				objectArr := object[keyName].([]interface{})
				valueInterface := it.getValueInterface(reflect.TypeOf(objectArr[0]), valueToSet)
				if indexInt >= len(objectArr) {
					object[keyName] = append(objectArr, valueInterface)
				} else {
					object[keyName].([]interface{})[indexInt] = valueInterface
				}
			} else {
				valueInterface := it.getValueInterface(reflectType, valueToSet)
				object[keyName] = valueInterface
			}
		}
	}
	return object
}

func (it *IterationHandler) getTokens(location string) [][]string {
	var tokens [][]string
	compositeSplit := strings.Split(location, ";")
	for _, split := range compositeSplit {
		token := it.getToken(split[1:])
		tokens = append(tokens, token)
	}
	return tokens
}

func (it *IterationHandler) getToken(location string) []string {
	var token []string
	simpleSplit := strings.Split(location, ".")
	opened := false
	quoteValue := ""
	for _, split := range simpleSplit {
		if !opened && split[0] == '(' {
			opened = true
			split = split[1:len(split)]
			quoteValue = split
		} else if opened {
			if split[len(split)-1] == ')' {
				opened = false
				split = split[0 : len(split)-1]
				quoteValue += "." + split
				token = append(token, quoteValue)
			} else {
				quoteValue += "." + split
			}
		} else {
			token = append(token, split)
		}
	}
	return token
}

func (it *IterationHandler) UpdateValue(baseObject map[string]interface{}, location string, value string) map[string]interface{} {
	if value == NULL_VALUE_STR {
		return baseObject
	}
	//locationSplits := strings.Split(location[1:], ".")
	locationSplitsArr := it.getTokens(location)
	values := strings.Split(value, ";")

	var modifiedObject map[string]interface{}
	for index, locationSplits := range locationSplitsArr {
		if index < len(values) {
			modifiedObject = it.updateNextValue(baseObject, locationSplits, 0, values[index])
		}
	}
	return modifiedObject
}

// Get All Combination
func (it *IterationHandler) nextCombination(itr []cpev1.IterationItem, curLayer int, prevList []map[string]string) []map[string]string {
	if curLayer == len(itr) {
		return prevList
	}
	var newList []map[string]string
	for _, value := range itr[curLayer].Values {
		if len(prevList) == 0 {
			newItem := make(map[string]string)
			newItem[itr[curLayer].Name] = value
			newList = append(newList, newItem)
		} else {
			for _, prevItem := range prevList {
				newItem := make(map[string]string)
				for prevK, prevV := range prevItem {
					newItem[prevK] = prevV
				}
				newItem[itr[curLayer].Name] = value
				newList = append(newList, newItem)
			}
		}
	}
	return it.nextCombination(itr, curLayer+1, newList)
}

func (it *IterationHandler) GetAllCombination(itr []cpev1.IterationItem) []map[string]string {
	var prevList []map[string]string
	return it.nextCombination(itr, 0, prevList)
}

// Get Init Combination

func (it *IterationHandler) deeperValue(object map[string]interface{}, keys []string, curIndex int) interface{} {
	curKey := keys[curIndex]
	_, nextObject, keyName, indexInt := it.getValue(curKey, object)
	if nextObject == nil {
		return NULL_VALUE_STR
	}

	if reflect.TypeOf(nextObject).Kind() == reflect.Map {
		return it.deeperValue(nextObject.(map[string]interface{}), keys, curIndex+1)
	}

	reflectType := reflect.TypeOf(object[keyName])
	if reflectType == reflect.TypeOf([]interface{}{}) {
		objectArr := object[keyName].([]interface{})
		return objectArr[indexInt]
	}
	return object[keyName]
}

func (it *IterationHandler) GetValue(initObject map[string]interface{}, location string) []string {
	// locationSplits := strings.Split(location[1:], ".")
	locationSplitsArr := it.getTokens(location)
	var values []string
	for _, locationSplits := range locationSplitsArr {
		value := it.deeperValue(initObject, locationSplits, 0)
		if value == "" { // cannot find
			return nil
		}
		values = append(values, fmt.Sprintf("%v", value))
	}
	return values
}
