package cli

import (
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/registry"
	"github.com/stretchr/testify/require"
)

func TestListRegistries(t *testing.T) {
	t.Parallel()

	globalCache, err := globalCacheDir()
	require.NoError(t, err)

	registries, err := registry.LoadAllFromCache(globalCache)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(registries))

}
