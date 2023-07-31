/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

package parser

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	BIGLINE        = "===================================="
	defaultDelimit = "="
)

func splitValue(linestr, delimit string) (key string, value float64, err error) {
	splits := strings.Split(linestr, delimit)
	if len(splits) != 2 {
		err = fmt.Errorf("cannot split value %s with %s", linestr, delimit)
		return
	}
	key = strings.TrimSpace(splits[0])
	valueStr := strings.TrimSpace(splits[1])
	value, err = strconv.ParseFloat(valueStr, 64)
	return
}
