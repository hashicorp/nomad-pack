package cache

import (
	"github.com/hashicorp/nomad-pack/pkg/pack"
)

// PackConfig represents the common configuration required by all packs. Used primarily
// by the cli package but should
type PackConfig struct {
	Registry string
	Name     string
	Ref      string
}

// Pack wraps a pack.Pack add adds the local cache ref. Useful for
// showing the registry in the global cache differentiated from the pack metadata.
type Pack struct {
	Ref string
	*pack.Pack
}

func invalidPackDefinition(provider cacheOperationProvider) *Pack {
	return &Pack{
		Ref: provider.AtRef(),
		Pack: &pack.Pack{
			Metadata: &pack.Metadata{
				App: &pack.MetadataApp{
					URL:    "",
					Author: "",
				},
				Pack: &pack.MetadataPack{
					Name:        provider.ForPackName(),
					Description: "",
					URL:         "",
					Version:     "Invalid pack definition",
				},
			},
		},
	}
}
