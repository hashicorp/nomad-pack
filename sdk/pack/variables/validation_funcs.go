// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package variables

import (
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// ValidationFunctions returns the set of HCL-native functions available inside
// variable validation condition expressions. The set is intentionally kept
// small and side-effect-free, mirroring the functions commonly expected in
// HCL2 variable validation blocks (as used in Terraform and Packer).
func ValidationFunctions() map[string]function.Function {
	return map[string]function.Function{
		// Collection
		"contains":        stdlib.ContainsFunc,
		"length":          stdlib.LengthFunc,
		"keys":            stdlib.KeysFunc,
		"values":          stdlib.ValuesFunc,
		"lookup":          stdlib.LookupFunc,
		"merge":           stdlib.MergeFunc,
		"flatten":         stdlib.FlattenFunc,
		"distinct":        stdlib.DistinctFunc,
		"compact":         stdlib.CompactFunc,
		"chunklist":       stdlib.ChunklistFunc,
		"coalesce":        stdlib.CoalesceFunc,
		"coalescelist":    stdlib.CoalesceListFunc,
		"concat":          stdlib.ConcatFunc,
		"element":         stdlib.ElementFunc,
		"index":           stdlib.IndexFunc,
		"reverse":         stdlib.ReverseListFunc,
		"slice":           stdlib.SliceFunc,
		"sort":            stdlib.SortFunc,
		"zipmap":          stdlib.ZipmapFunc,
		"hasindex":        stdlib.HasIndexFunc,
		"setproduct":      stdlib.SetProductFunc,
		"setintersection": stdlib.SetIntersectionFunc,
		"setunion":        stdlib.SetUnionFunc,
		"setsubtract":     stdlib.SetSubtractFunc,

		// String
		"join":         stdlib.JoinFunc,
		"split":        stdlib.SplitFunc,
		"chomp":        stdlib.ChompFunc,
		"trimspace":    stdlib.TrimSpaceFunc,
		"trim":         stdlib.TrimFunc,
		"trimprefix":   stdlib.TrimPrefixFunc,
		"trimsuffix":   stdlib.TrimSuffixFunc,
		"indent":       stdlib.IndentFunc,
		"title":        stdlib.TitleFunc,
		"upper":        stdlib.UpperFunc,
		"lower":        stdlib.LowerFunc,
		"replace":      stdlib.ReplaceFunc,
		"substr":       stdlib.SubstrFunc,
		"strlen":       stdlib.StrlenFunc,
		"regex":        stdlib.RegexFunc,
		"regexall":     stdlib.RegexAllFunc,
		"regexreplace": stdlib.RegexReplaceFunc,
		"format":       stdlib.FormatFunc,
		"formatlist":   stdlib.FormatListFunc,

		// Numeric
		"abs":      stdlib.AbsoluteFunc,
		"ceil":     stdlib.CeilFunc,
		"floor":    stdlib.FloorFunc,
		"min":      stdlib.MinFunc,
		"max":      stdlib.MaxFunc,
		"log":      stdlib.LogFunc,
		"pow":      stdlib.PowFunc,
		"signum":   stdlib.SignumFunc,
		"parseint": stdlib.ParseIntFunc,
		"int":      stdlib.IntFunc,

		// Boolean / comparison
		"not": stdlib.NotFunc,
		"and": stdlib.AndFunc,
		"or":  stdlib.OrFunc,

		// Encoding
		"csvdecode":  stdlib.CSVDecodeFunc,
		"jsondecode": stdlib.JSONDecodeFunc,
		"jsonencode": stdlib.JSONEncodeFunc,

		// Range
		"range": stdlib.RangeFunc,
	}
}
