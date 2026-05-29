package source

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	require.NotNil(t, reg)
	require.NotNil(t, reg.sources)
	require.Len(t, reg.sources, 0)
}

func TestRegistry_Register(t *testing.T) {
	reg := NewRegistry()

	source1 := &mockSource{name: "source1", priority: 10}
	source2 := &mockSource{name: "source2", priority: 20}

	reg.Register(source1)
	require.Len(t, reg.sources, 1)

	reg.Register(source2)
	require.Len(t, reg.sources, 2)
}

func TestRegistry_Resolve_EmptyRegistry(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	packID := pack.ID("test-pack")

	vars, err := reg.Resolve(ctx, packID)
	require.NoError(t, err)
	require.NotNil(t, vars)
	require.Len(t, vars, 0)
}

func TestRegistry_Resolve_SingleSource(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	packID := pack.ID("test-pack")

	testVar := &variables.Variable{
		Name:  "test_var",
		Value: cty.StringVal("test_value"),
	}

	source := &mockSource{
		name:     "test",
		priority: 10,
		vars:     []*variables.Variable{testVar},
	}

	reg.Register(source)

	vars, err := reg.Resolve(ctx, packID)
	require.NoError(t, err)
	require.Len(t, vars, 1)
	require.Equal(t, testVar, vars[0])
}

func TestRegistry_Resolve_PriorityOverride(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	packID := pack.ID("test-pack")

	lowPriorityVar := &variables.Variable{
		Name:  "test_var",
		Value: cty.StringVal("low_priority"),
	}

	highPriorityVar := &variables.Variable{
		Name:  "test_var",
		Value: cty.StringVal("high_priority"),
	}

	lowSource := &mockSource{
		name:     "low",
		priority: 10,
		vars:     []*variables.Variable{lowPriorityVar},
	}

	highSource := &mockSource{
		name:     "high",
		priority: 20,
		vars:     []*variables.Variable{highPriorityVar},
	}

	// Register in reverse order to test sorting
	reg.Register(highSource)
	reg.Register(lowSource)

	vars, err := reg.Resolve(ctx, packID)
	require.NoError(t, err)
	require.Len(t, vars, 1)
	// High priority should win
	require.Equal(t, "high_priority", vars[0].Value.AsString())
}

func TestRegistry_Resolve_MultipleSources(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	packID := pack.ID("test-pack")

	source1 := &mockSource{
		name:     "source1",
		priority: 10,
		vars: []*variables.Variable{
			{Name: "var1", Value: cty.StringVal("value1")},
			{Name: "var2", Value: cty.StringVal("value2_low")},
		},
	}

	source2 := &mockSource{
		name:     "source2",
		priority: 20,
		vars: []*variables.Variable{
			{Name: "var2", Value: cty.StringVal("value2_high")},
			{Name: "var3", Value: cty.StringVal("value3")},
		},
	}

	reg.Register(source1)
	reg.Register(source2)

	vars, err := reg.Resolve(ctx, packID)
	require.NoError(t, err)
	require.Len(t, vars, 3)

	// Convert to map for easier testing
	varMap := make(map[variables.ID]*variables.Variable)
	for _, v := range vars {
		varMap[v.Name] = v
	}

	require.Equal(t, "value1", varMap["var1"].Value.AsString())
	require.Equal(t, "value2_high", varMap["var2"].Value.AsString()) // Higher priority wins
	require.Equal(t, "value3", varMap["var3"].Value.AsString())
}

func TestRegistry_Resolve_SourceError(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	packID := pack.ID("test-pack")

	expectedErr := errors.New("fetch failed")

	source := &mockSource{
		name:     "failing",
		priority: 10,
		err:      expectedErr,
	}

	reg.Register(source)

	vars, err := reg.Resolve(ctx, packID)
	require.Error(t, err)
	require.Nil(t, vars)
	require.Contains(t, err.Error(), "failed to fetch from failing")
	require.Contains(t, err.Error(), "fetch failed")
}

func TestRegistry_Register_NilSource(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot register nil source")
}

func TestRegistry_Register_DuplicateName(t *testing.T) {
	reg := NewRegistry()

	source1 := &mockSource{name: "duplicate", priority: 10}
	source2 := &mockSource{name: "duplicate", priority: 20}

	err := reg.Register(source1)
	require.NoError(t, err)

	err = reg.Register(source2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Resolve_ContextCancelled(t *testing.T) {
	reg := NewRegistry()
	packID := pack.ID("test-pack")

	source := &mockSource{
		name:     "test",
		priority: 10,
		vars:     []*variables.Variable{},
	}

	reg.Register(source)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	vars, err := reg.Resolve(ctx, packID)
	require.Error(t, err)
	require.Nil(t, vars)
	require.Contains(t, err.Error(), "context cancelled")
}

func TestRegistry_Sources(t *testing.T) {
	reg := NewRegistry()

	source1 := &mockSource{name: "source1", priority: 10}
	source2 := &mockSource{name: "source2", priority: 20}

	reg.Register(source1)
	reg.Register(source2)

	sources := reg.Sources()
	require.Len(t, sources, 2)

	// Verify it's a copy (modifying returned slice doesn't affect registry)
	sources[0] = nil
	require.Len(t, reg.sources, 2)
	require.NotNil(t, reg.sources[0])
}

func TestRegistry_Clear(t *testing.T) {
	reg := NewRegistry()

	source1 := &mockSource{name: "source1", priority: 10}
	source2 := &mockSource{name: "source2", priority: 20}

	reg.Register(source1)
	reg.Register(source2)
	require.Len(t, reg.sources, 2)

	reg.Clear()
	require.Len(t, reg.sources, 0)
}
