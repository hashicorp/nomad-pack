// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package variable

import (
	"fmt"
	"math"
	"math/big"

	"github.com/zclconf/go-cty/cty"
)

func convertCtyToInterface(val cty.Value) (interface{}, error) {

	if val.IsNull() {
		return nil, nil
	}

	if !val.IsKnown() {
		return nil, fmt.Errorf("value is not known")
	}

	t := val.Type()

	switch {
	case t.IsPrimitiveType():
		switch t {
		case cty.String:
			return val.AsString(), nil
		case cty.Number:
			switch {
			case val.RawEquals(cty.PositiveInfinity):
				return math.Inf(1), nil
			case val.RawEquals(cty.NegativeInfinity):
				return math.Inf(-1), nil
			default:
				return smallestNumber(val.AsBigFloat()), nil
			}
		case cty.Bool:
			return val.True(), nil
		default:
			panic("unsupported primitive type")
		}
	case isCollectionOfMaps(t):
		result := []map[string]interface{}{}

		it := val.ElementIterator()
		for it.Next() {
			_, ev := it.Element()
			evi, err := convertCtyToInterface(ev)
			if err != nil {
				return nil, err
			}
			result = append(result, evi.(map[string]interface{}))
		}
		return result, nil
	case t.IsListType(), t.IsSetType(), t.IsTupleType():
		result := []interface{}{}

		it := val.ElementIterator()
		for it.Next() {
			_, ev := it.Element()
			evi, err := convertCtyToInterface(ev)
			if err != nil {
				return nil, err
			}
			result = append(result, evi)
		}
		return result, nil
	case t.IsMapType():
		result := map[string]interface{}{}
		it := val.ElementIterator()
		for it.Next() {
			ek, ev := it.Element()

			ekv := ek.AsString()
			evv, err := convertCtyToInterface(ev)
			if err != nil {
				return nil, err
			}

			result[ekv] = evv
		}
		return result, nil
	case t.IsObjectType():
		result := map[string]interface{}{}

		for k := range t.AttributeTypes() {
			av := val.GetAttr(k)
			avv, err := convertCtyToInterface(av)
			if err != nil {
				return nil, err
			}

			result[k] = avv
		}
		return result, nil
	case t.IsCapsuleType():
		rawVal := val.EncapsulatedValue()
		return rawVal, nil
	default:
		// should never happen
		return nil, fmt.Errorf("cannot serialize %s", t.FriendlyName())
	}
}

func smallestNumber(b *big.Float) interface{} {
	if v, acc := b.Int64(); acc == big.Exact {
		// check if it fits in int
		if int64(int(v)) == v {
			return int(v)
		}
		return v
	}

	v, _ := b.Float64()
	return v
}

func isCollectionOfMaps(t cty.Type) bool {
	switch {
	// t.IsCollectionType() also match when t is type Map
	case t.IsCollectionType() && !t.IsMapType():
		et := t.ElementType()
		return et.IsMapType() || et.IsObjectType()
	case t.IsTupleType():
		ets := t.TupleElementTypes()
		for _, et := range ets {
			if !et.IsMapType() && !et.IsObjectType() {
				return false
			}
		}

		return len(ets) > 0
	default:
		return false
	}
}
