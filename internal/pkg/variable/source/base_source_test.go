package source

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestNewBaseSource(t *testing.T) {
	vars := make(variables.PackIDKeyedVarMap)
	source := NewBaseSource("test", 10, vars)

	require.NotNil(t, source)
	require.Equal(t, "test", source.name)
	require.Equal(t, 10, source.priority)
	require.Equal(t, vars, source.vars)
}

func TestBaseSource_Name(t *testing.T) {
	source := NewBaseSource("custom-name", 10, nil)
	require.Equal(t, "custom-name", source.Name())
}

func TestBaseSource_Priority(t *testing.T) {
	source := NewBaseSource("test", 42, nil)
	require.Equal(t, 42, source.Priority())
}

func TestBaseSource_Fetch_NilVars(t *testing.T) {
	source := NewBaseSource("test", 10, nil)
	ctx := context.Background()
	packID := pack.ID("test-pack")

	vars, err := source.Fetch(ctx, packID)
	require.NoError(t, err)
	require.NotNil(t, vars)
	require.Len(t, vars, 0)
}

func TestBaseSource_Fetch_EmptyVars(t *testing.T) {
	source := NewBaseSource("test", 10, make(variables.PackIDKeyedVarMap))
	ctx := context.Background()
	packID := pack.ID("test-pack")

	vars, err := source.Fetch(ctx, packID)
	require.NoError(t, err)
	require.NotNil(t, vars)
	require.Len(t, vars, 0)
}

func TestBaseSource_Fetch_WithVars(t *testing.T) {
	packID := pack.ID("test-pack")
	testVar := &variables.Variable{
		Name:  "test_var",
		Value: cty.StringVal("test_value"),
	}

	varsMap := variables.PackIDKeyedVarMap{
		packID: []*variables.Variable{testVar},
	}

	source := NewBaseSource("test", 10, varsMap)
	ctx := context.Background()

	vars, err := source.Fetch(ctx, packID)
	require.NoError(t, err)
	require.Len(t, vars, 1)
	require.Equal(t, testVar, vars[0])
}

func TestBaseSource_Fetch_ContextCancelled(t *testing.T) {
	source := NewBaseSource("test", 10, make(variables.PackIDKeyedVarMap))
	packID := pack.ID("test-pack")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	vars, err := source.Fetch(ctx, packID)
	require.Error(t, err)
	require.Nil(t, vars)
}

func TestNewEnvSource(t *testing.T) {
	vars := make(variables.PackIDKeyedVarMap)
	source := NewEnvSource(10, vars)

	require.NotNil(t, source)
	require.Equal(t, "env", source.Name())
	require.Equal(t, 10, source.Priority())
}

func TestNewFileSource(t *testing.T) {
	vars := make(variables.PackIDKeyedVarMap)
	source := NewFileSource(20, vars)

	require.NotNil(t, source)
	require.Equal(t, "file", source.Name())
	require.Equal(t, 20, source.Priority())
}

func TestNewCLISource(t *testing.T) {
	vars := make(variables.PackIDKeyedVarMap)
	source := NewCLISource(30, vars)

	require.NotNil(t, source)
	require.Equal(t, "cli", source.Name())
	require.Equal(t, 30, source.Priority())
}
