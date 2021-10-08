package registry

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/pkg/pack"
)

// CachedRegistry represents a registry definition from the global cache.
type CachedRegistry struct {
	Name  string
	URL   string
	Packs []*CachedPack
}

// LoadCachedPack will attempt to load a pack from a path, and then append it to the
// registry's Packs slice. If the root of the path does not contain a metadata.hcl
// file, it is not considered a valid pack, and will return an error. If the loader
// is unable to load the pack, likewise and error is returned.
func (r *CachedRegistry) LoadCachedPack(packPath string) error {
	// Guard against processing the .git folder or non-versioned directories
	if strings.Contains(packPath, ".git") || !strings.Contains(packPath, "@") {
		return nil
	}

	var err error

	// get the pack name from the packPath
	packName := path.Base(packPath)

	// Get the list of entries from the pack directory.
	packEntries, err := os.ReadDir(packPath)
	if err != nil {
		return err
	}

	var registryPack *pack.Pack
	// Iterate over directory entries to look for the metadata.hcl file.
	for _, packEntry := range packEntries {
		// If the entry is a directory, skip since we are only interested in the
		// metadata.hcl file
		if packEntry.IsDir() {
			continue
		}

		// If there is a metadata.hcl file try to load the pack.
		if packEntry.Name() == "metadata.hcl" {
			registryPack, err = loader.Load(packPath)
			// Add an invalid pack definition if it can't be loaded
			if err != nil {
				// Don't error but load an invalid sentinel pack
				registryPack = invalidPackDefinition(packName)
			}
		}
	}

	if registryPack == nil {
		return fmt.Errorf("unable to load registry pack at %s", packPath)
	}

	// Get version from packPath
	segments := strings.Split(packPath, "@")
	if len(segments) != 2 {
		return fmt.Errorf("invalid pack path: no version segment in %s", packPath)
	}

	// Load a cached pack instance from the pack.Pack
	cachedPack := &CachedPack{
		CacheName:    registryPack.Name(),
		CacheVersion: segments[1],
		Pack:         registryPack,
	}

	// Invalid packs will have versions added from the path. Make sure valid packs
	// have version appended to name.
	if !strings.Contains(cachedPack.Name(), "@") {
		cachedPack.CacheName = fmt.Sprintf("%s@%s", cachedPack.Name(), segments[1])
	}

	// Append the pack to the registry's packs field.
	r.Packs = append(r.Packs, cachedPack)

	return nil
}

// setURLFromPacks sets the URL since we don't have this stored in any sort of
// reliable way.
func (r *CachedRegistry) setURLFromPacks() {
	for _, cachedPack := range r.Packs {
		if cachedPack.Metadata.Pack.URL == "" {
			continue
		}

		// Get the pack url from the metadata
		url := cachedPack.Metadata.Pack.URL

		// Get the substring to remove any prefixes
		url = url[strings.Index(url, "github.com"):]

		// Split into a slice of segments, since this should include the pack name
		segments := strings.Split(url, "/")

		// Initialize this to the number of segments we want
		segmentCount := 3

		// Set the count to len of segments in case URL is not formatted correctly.
		if len(segments) < 3 {
			segmentCount = len(segments)
		}

		// set the URL back to a joined url.
		r.URL = strings.Join(segments[:segmentCount], "/")

		// Exit once we have a valid pack
		return
	}

	// Set meaningful message if no valid packs found.

	r.URL = "Not parsable - registry contains no valid packs"
}

// CachedPack wraps a pack.Pack add adds the local cache version. Useful for
// showing the registry in the global cache differentiated from the pack metadata.
type CachedPack struct {
	CacheVersion string
	CacheName    string
	*pack.Pack
}

func invalidPackDefinition(name string) *pack.Pack {
	return &pack.Pack{
		Metadata: &pack.Metadata{
			App: &pack.MetadataApp{
				URL:    "",
				Author: "",
			},
			Pack: &pack.MetadataPack{
				Name:        name,
				Description: "",
				URL:         "",
				Version:     "Invalid pack definition",
			},
		},
	}
}
