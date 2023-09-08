package variable

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser"
	"github.com/hashicorp/nomad-pack/internal/pkg/variable/parser/config"
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

type Parser interface {
	Parse() (*parser.ParsedVariables, hcl.Diagnostics)
}

type PackTemplateContexter interface {
	GetPackTemplateContext(p pack.ID) any
}

func NewParser(cfg *config.ParserConfig) (Parser, error) {
	return parser.NewParser(cfg)
}
