// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package creator

import (
	"os"
	"testing"

	"github.com/shoenig/test/must"
)

func TestInit(t *testing.T) {
	must.NotNil(t, tpl)
	err := tpl.ExecuteTemplate(os.Stdout, "pack_readme.md", map[string]string{
		"PackName": "foo",
	})
	must.NoError(t, err)
}
