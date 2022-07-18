package variable

import (
	"reflect"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestConvertCtyToInterface(t *testing.T) {

	// test basic type
	tables := []struct {
		val cty.Value
		t   reflect.Kind
	}{
		{cty.BoolVal(true), reflect.Bool},
		{cty.StringVal("test"), reflect.String},
		{cty.NumberIntVal(1), reflect.Int},
		{cty.MapVal(map[string]cty.Value{"test": cty.BoolVal(true)}), reflect.Map},
	}

	for _, table := range tables {
		res, err := convertCtyToInterface(table.val)
		resType := reflect.TypeOf(res).Kind()
		if resType != table.t || err != nil {
			t.Errorf("Error while converting type %v", table.t)
		}

	}

	// test list of list
	testListOfList := cty.ListVal([]cty.Value{
		cty.ListVal([]cty.Value{
			cty.BoolVal(true),
		}),
	})

	resListOfList, err := convertCtyToInterface(testListOfList)
	tempList, checkListOfList1 := resListOfList.([]interface{})
	_, checkListOfList2 := tempList[0].([]interface{})

	if err != nil || checkListOfList1 != true || checkListOfList2 != true {
		t.Errorf("Error while converting type list of list")
	}

	// test list of maps
	testListOfMaps := cty.ListVal([]cty.Value{
		cty.MapVal(map[string]cty.Value{
			"test": cty.BoolVal(true),
		}),
	})
	resListOfMaps, err := convertCtyToInterface(testListOfMaps)

	_, checkListOfMaps := resListOfMaps.([]map[string]interface{})
	if err != nil || checkListOfMaps != true {
		t.Errorf("Error while converting type list of maps")
	}

	// test map of maps
	testMapOfMaps := cty.MapVal(map[string]cty.Value{
		"test": cty.MapVal(map[string]cty.Value{"test": cty.BoolVal(true)}),
	})

	restMapOfMaps, err := convertCtyToInterface(testMapOfMaps)

	tempMapOfMaps, checkMapOfMaps1 := restMapOfMaps.(map[string]interface{})
	_, checkMapOfMaps2 := tempMapOfMaps["test"].(map[string]interface{})
	if err != nil || checkMapOfMaps1 != true || checkMapOfMaps2 != true {
		t.Errorf("Error while converting type maps of maps")
	}

}
