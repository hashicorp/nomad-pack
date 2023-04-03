// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package creator

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	require.NotNil(t, tpl)
	err := tpl.ExecuteTemplate(os.Stdout, "pack_readme.md", map[string]string{
		"PackName": "foo",
	})
	require.NoError(t, err)
}
