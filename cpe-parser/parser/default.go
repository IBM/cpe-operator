/*
 * Copyright 2022- IBM Inc. All rights reserved
 * SPDX-License-Identifier: Apache2.0
 */

 package parser

type DefaultParser struct {
	*BaseParser
}

func NewDefaultParser() *DefaultParser {
	defaultParser := &DefaultParser{}
	abs := &BaseParser{
		Parser: defaultParser,
	}
	defaultParser.BaseParser = abs
	return defaultParser
}
