package main

import (
	"fmt"

	"github.com/hashicorp/nomad-pack/cli"
	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad/e2e/framework"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "community",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(CommunityTestCase),
		},
	})
}

type CommunityTestCase struct {
	framework.TC
}

func (tc *CommunityTestCase) TestCommunityRegistry(f *framework.F) {
	glache, err := cache.NewCache(&cache.CacheConfig{
		Path:   cache.DefaultCachePath(),
		Eager:  true,
		Logger: nil,
	})
	f.NoError(err)

	// Make sure to delete all packs when test is over
	defer func() {
		for _, registry := range glache.Registries() {
			if registry.Name != cache.DefaultRegistryName {
				continue
			}

			for _, pack := range registry.Packs {
				if pack.Ref != cache.DefaultRef {
					continue
				}
				exitCode := cli.DestroyCmd().Run([]string{pack.Name()})
				f.Equal(0, exitCode)
			}
		}
	}()

	f.T().Log(fmt.Sprintf("found %d registries", len(glache.Registries())))
	f.NotEqual(0, len(glache.Registries()))

	for _, registry := range glache.Registries() {
		for _, pack := range registry.Packs {
			if pack.Ref != cache.DefaultRef {
				continue
			}

			f.T().Log(fmt.Sprintf("Running pack %s\n", pack.Name()))

			exitCode := cli.RunCmd().Run([]string{pack.Name()})
			f.Equal(0, exitCode)
		}
	}

}
