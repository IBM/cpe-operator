/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package controllers

// https://github.com/d4l3k/go-bayesopt/blob/master/bayesopt.go
import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"

	bo "github.com/d4l3k/go-bayesopt"
)

const (
	TUNED_MAX_QSIZE  = 100
	RANGE_MAX_LENGTH = 1000
	SET_MAX_LENGTH   = 20

	// opt params
	RANDOM_ROUND = 5
	MAX_ROUND    = 100

	CONFIG_FOLDER = "/etc/search-space"
)

var SearchSpace map[TuneType][]bo.Param
var ParamNameMap map[string]TuneType

func InitSearchSpace() error {
	searchSpace, paramNameMap, err := GetSearchSpaceConfig(CONFIG_FOLDER)
	SearchSpace = searchSpace
	ParamNameMap = paramNameMap
	return err
}

///////////////////////////////////////////////////////////
// Customized Parameters
//
//// Parameter in form of valid set ////////////////////

var _ bo.Param = SetParam{}

type SetParam struct {
	Name      string
	Values    [SET_MAX_LENGTH]string
	SetLength int
}

func (p SetParam) GetName() string {
	return p.Name
}

func (p SetParam) GetMax() float64 {
	return float64(p.SetLength)
}

func (p SetParam) GetMin() float64 {
	return 0
}

func (p SetParam) Sample() float64 {
	return float64(rand.Intn(p.SetLength))
}

func (p SetParam) GetSetValue(index float64) string {
	return p.Values[int(index)]
}

func (p SetParam) Validate(value float64) float64 {
	return float64(int(value))
}

//// Parameter in form of integer ////////////////////

var _ bo.Param = IntUniformParam{}

type IntUniformParam struct {
	Name           string
	Max, Min, Step int
}

func (p IntUniformParam) GetName() string {
	return p.Name
}

func (p IntUniformParam) GetMax() float64 {
	return float64(p.Max)
}

func (p IntUniformParam) GetMin() float64 {
	return float64(p.Min)
}

func (p IntUniformParam) Sample() float64 {
	return float64(rand.Intn(int(float64(p.Max-p.Min)/float64(p.Step)))*p.Step + p.Min)
}

func (p IntUniformParam) Validate(inValue float64) float64 {
	return float64(int(inValue/float64(p.Step)) * p.Step)
}

///////////////////////////////////////////////////////////

type TuneType string
type ParamType string

func splitByEqual(value string) ([]string, error) {
	splited := strings.Split(value, "=")
	if len(splited) != 2 {
		return splited, errors.New(fmt.Sprintf("wrong config %s", value))
	}
	return splited, nil
}

func CreateSetParam(value string) (bo.Param, error) {
	var param SetParam

	splited, err := splitByEqual(value)
	if err != nil {
		return param, err
	}

	values := strings.Split(splited[1], ",")
	if len(values) >= SET_MAX_LENGTH {
		return param, errors.New(fmt.Sprintf("wrong config %s (too large set %d >= %d)", value, len(values), SET_MAX_LENGTH))
	}
	var fixedValues [SET_MAX_LENGTH]string
	for index, value := range values {
		fixedValues[index] = value
	}

	return SetParam{
		Name:      splited[0],
		Values:    fixedValues,
		SetLength: len(values),
	}, nil
}

func CreateIntUniformParam(value string) (bo.Param, error) {
	var param IntUniformParam

	splited, err := splitByEqual(value)
	if err != nil {
		return param, err
	}

	valueSplited := strings.Split(splited[1], ",")
	if len(valueSplited) < 2 {
		return param, errors.New(fmt.Sprintf("wrong config %s", value))
	}

	min, minErr := strconv.ParseInt(valueSplited[0], 10, 64)
	max, maxErr := strconv.ParseInt(valueSplited[1], 10, 64)
	var step int64
	step = 1
	if len(valueSplited) >= 3 {
		step, err = strconv.ParseInt(valueSplited[2], 10, 64)
	}
	if minErr != nil || maxErr != nil || err != nil {
		return param, errors.New(fmt.Sprintf("wrong config %s", value))
	}
	return IntUniformParam{
		Name: splited[0],
		Max:  int(max),
		Min:  int(min),
		Step: int(step),
	}, nil
}

func CreateUniformParam(value string) (bo.Param, error) {
	var param IntUniformParam

	splited, err := splitByEqual(value)
	if err != nil {
		return param, err
	}

	valueSplited := strings.Split(splited[1], ",")
	if len(valueSplited) < 2 {
		return param, errors.New(fmt.Sprintf("wrong config %s", value))
	}

	min, minErr := strconv.ParseFloat(valueSplited[0], 64)
	max, maxErr := strconv.ParseFloat(valueSplited[1], 64)
	if minErr != nil || maxErr != nil {
		return param, errors.New(fmt.Sprintf("wrong config %s", value))
	}

	return bo.UniformParam{
		Name: splited[0],
		Max:  max,
		Min:  min,
	}, nil
}

func (t TuneType) IsValid() error {
	switch t {
	case "audio", "cpu", "disk", "eeepc_she", "modules", "mounts", "net", "scheduler", "scsi_host", "selinux", "sysctl", "sysfs", "usb", "video", "vm":
		return nil
	}
	return errors.New("Invalid TuneType")
}
func (t ParamType) GetParam(fileName string, tuneType TuneType, paramNameMap map[string]TuneType) ([]bo.Param, error) {
	var params []bo.Param
	valueFile, err := os.Open(fileName)
	defer valueFile.Close()
	if err != nil {
		return params, err
	}
	var createFunc func(string) (bo.Param, error)
	switch t {
	case "int":
		createFunc = CreateIntUniformParam
	case "set":
		createFunc = CreateSetParam
	case "float":
		createFunc = CreateUniformParam
	default:
		return params, errors.New(fmt.Sprintf("Invalid ParamType %s", t))
	}
	scanner := bufio.NewScanner(valueFile)

	for scanner.Scan() {
		param, err := createFunc(scanner.Text())
		if err != nil {
			return params, err
		}
		params = append(params, param)
		paramNameMap[param.GetName()] = tuneType
	}

	if err := scanner.Err(); err != nil {
		return params, err
	}
	return params, nil
}

func GetSearchSpaceConfig(configMapLoc string) (map[TuneType][]bo.Param, map[string]TuneType, error) {
	searchSpace := make(map[TuneType][]bo.Param)
	paramNameMap := make(map[string]TuneType)
	files, err := ioutil.ReadDir(configMapLoc)

	if err != nil {
		return searchSpace, paramNameMap, err
	}
	for _, file := range files {
		splited := strings.Split(file.Name(), ".")
		if len(splited) != 2 {
			continue
		}
		tuneType := TuneType(splited[0])
		err = tuneType.IsValid()
		if err != nil {
			return searchSpace, paramNameMap, err
		}
		paramType := ParamType(splited[1])
		params, err := paramType.GetParam(configMapLoc+"/"+file.Name(), tuneType, paramNameMap)
		if err != nil {
			return searchSpace, paramNameMap, err
		}
		if prevParams, exists := searchSpace[tuneType]; exists {
			searchSpace[tuneType] = append(prevParams, params...)
		} else {
			searchSpace[tuneType] = params
		}
	}
	return searchSpace, paramNameMap, nil
}

func getValue(param bo.Param, value float64) string {
	switch reflect.TypeOf(param) {
	case reflect.TypeOf(SetParam{}):
		return param.(SetParam).GetSetValue(value)
	case reflect.TypeOf(IntUniformParam{}):
		return fmt.Sprintf("%d", int(value))
	}
	return fmt.Sprintf("%.2f", value)
}

func convertToProfile(paramValue map[bo.Param]float64, paramNameMap map[string]TuneType) (map[TuneType]map[string]string, error) {
	profileValueMap := make(map[TuneType]map[string]string)
	for param, value := range paramValue {
		if tuneType, exists := paramNameMap[param.GetName()]; exists {
			if _, exists := profileValueMap[tuneType]; !exists {
				profileValueMap[tuneType] = make(map[string]string)
			}
			profileValueMap[tuneType][param.GetName()] = getValue(param, value)
		} else {
			return profileValueMap, errors.New(fmt.Sprintf("Not found tune type %s in %v \n results: %v", tuneType, paramNameMap, paramValue))
		}
	}
	return profileValueMap, nil
}

type BaysesOptimizer struct {
	SampleQueue chan map[TuneType]map[string]string
	ResultQueue chan float64
	*bo.Optimizer
	Minimize              bool
	FinalizedTunedProfile map[TuneType]map[string]string
	AutoTuned             bool
	FinalizedReady        bool
	FinalizedApplied      bool
	SamplingCount         int
}

func NewBayesOptimizer(minimize bool) *BaysesOptimizer {
	sampleQueue := make(chan map[TuneType]map[string]string, TUNED_MAX_QSIZE)
	resultQueue := make(chan float64, TUNED_MAX_QSIZE)
	var paramList []bo.Param

	for _, params := range SearchSpace {
		paramList = append(paramList, params...)
	}
	o := bo.New(
		paramList,
		bo.WithMinimize(minimize),
		bo.WithRandomRounds(RANDOM_ROUND),
		bo.WithRounds(MAX_ROUND),
	)

	return &BaysesOptimizer{
		SampleQueue:      sampleQueue,
		ResultQueue:      resultQueue,
		Optimizer:        o,
		Minimize:         minimize,
		AutoTuned:        false,
		FinalizedReady:   false,
		FinalizedApplied: false,
	}
}

func (b *BaysesOptimizer) AutoTune() {
	b.AutoTuned = true
	optimizedParams := b.optimize()
	finalizedTunedProfile, _ := convertToProfile(optimizedParams, ParamNameMap)
	b.FinalizedTunedProfile = finalizedTunedProfile
	b.Finalize()
}

func (b *BaysesOptimizer) SetFinalizedApplied() {
	b.Finalize()
	b.FinalizedApplied = true
}

func validateSample(params map[bo.Param]float64) (bool, map[bo.Param]float64) {
	validatedParams := make(map[bo.Param]float64)
	for param, value := range params {
		if value < param.GetMin() || value > param.GetMax() {
			return false, params
		}
		// in case of param comes from exploration
		switch reflect.TypeOf(param) {
		case reflect.TypeOf(SetParam{}):
			validatedParams[param] = param.(SetParam).Validate(value)
		case reflect.TypeOf(IntUniformParam{}):
			validatedParams[param] = param.(IntUniformParam).Validate(value)
		default:
			validatedParams[param] = value
		}
	}
	return true, validatedParams
}

func (b *BaysesOptimizer) optimize() map[bo.Param]float64 {

	x, _, err := b.Optimizer.Optimize(func(params map[bo.Param]float64) float64 {
		valid, validatedParams := validateSample(params)
		if valid {
			fmt.Println("Add new profile")
			// submit node tuning to operator
			tunedProfile, _ := convertToProfile(validatedParams, ParamNameMap)
			b.SampleQueue <- tunedProfile
			b.SamplingCount = b.SamplingCount + 1
			// wait for result to return
			performanceValue := <-b.ResultQueue
			fmt.Println("Optimize: ", validatedParams, performanceValue)
			return performanceValue
		}
		if b.Minimize {
			return math.MaxFloat64
		}
		return -1
	})
	if err != nil {
		return map[bo.Param]float64{}
	}
	return x
}

func (b *BaysesOptimizer) Finalize() {
	if !b.FinalizedReady {
		b.FinalizedReady = true
		close(b.SampleQueue)
		close(b.ResultQueue)
	}
}
