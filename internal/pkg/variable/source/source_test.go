package source

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad-pack/sdk/pack"
	"github.com/hashicorp/nomad-pack/sdk/pack/variables"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// mockSource is a test implementation of VariableSource
type mockSource struct {
	name     string
	priority int
	vars     []*variables.Variable
	err      error
}

func (m *mockSource) Name() string {
	return m.name
}

func (m *mockSource) Priority() int {
	return m.priority
}

func (m *mockSource) Fetch(ctx context.Context, packID pack.ID) ([]*variables.Variable, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.vars, nil
}

func TestVariableSource_Interface(t *testing.T) {
	// Verify that our mock implements the interface
	var _ VariableSource = (*mockSource)(nil)

	ctx := context.Background()
	packID := pack.ID("test-pack")

	testVar := &variables.Variable{
		Name:  "test_var",
		Value: cty.StringVal("test_value"),
	}

	source := &mockSource{
		name:     "mock",
		priority: 10,
		vars:     []*variables.Variable{testVar},
	}

	require.Equal(t, "mock", source.Name())
	require.Equal(t, 10, source.Priority())

	vars, err := source.Fetch(ctx, packID)
	require.NoError(t, err)
	require.Len(t, vars, 1)
	require.Equal(t, testVar, vars[0])
}
