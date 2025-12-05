// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
)

type Parser interface {
	Parse() (*ParsedVariables, hcl.Diagnostics)
}

func NewParser(cfg *config.ParserConfig) (Parser, error) {
	if cfg.Version == config.V1 {
		return NewParserV1(cfg)
	}
	return NewParserV2(cfg)
}
