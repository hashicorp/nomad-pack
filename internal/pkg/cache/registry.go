package cache

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/hashicorp/nomad-pack/internal/pkg/loader"
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

// Registry represents a registry definition from the global cache.
type Registry struct {
	Name   string
	Source string
	Ref    string
	Packs  []*Pack
}

// get will attempt to load the specified packs from a path, and then append them
// to the registry's Packs slice. If no packs specified, it will get them all.
// If the root of the path does not contain a metadata.hcl file, it is not
// considered a valid pack, and will return an invalid cached pack.
// If the loader is unable to load the pack, likewise an invalid cached pack is
// returned. This function is not exported, to enforce clients using the cache functions.
// It will attempt resolve any errors so that it can continue loading potentially
// valid packs.
func (r *Registry) get(opts *GetOpts, cache *Cache) (err error) {
	var packEntries []os.DirEntry
	// Get the list of entries from the registry directory.
	packEntries, err = os.ReadDir(opts.RegistryPath())
	if err != nil {
		// If we can't read the directory, return error.
		return
	}

	// Iterate over the packs in the registry and load each pack so that
	// we can extract information from the metadata.
	for _, packEntry := range packEntries {
		// Skip any entries not targeted.
		if !opts.IsTarget(packEntry) {
			continue
		}

		var loadedPack *pack.Pack
		var cachedPack *Pack

		if _, err = os.Stat(path.Join(opts.RegistryPath(), packEntry.Name(), "metadata.hcl")); os.IsNotExist(err) {
			cache.cfg.Logger.ErrorWithContext(errors.New("error loading pack"),
				fmt.Sprintf("no metadata.hcl found in pack %s", packEntry.Name()), cache.ErrorContext.GetAll()...)

			// Add an invalid pack if no metadata.hcl exists
			invalidOpts := &GetOpts{
				cachePath:    opts.cachePath,
				RegistryName: opts.RegistryName,
				PackName:     packEntry.Name(),
				Ref:          opts.Ref,
			}
			cachedPack = invalidPackDefinition(invalidOpts)
			// Append the pack to the registry's packs field.
			r.add(cachedPack)
			continue
		} else if err != nil {
			// If some other error, log and continue to next pack.
			cache.cfg.Logger.ErrorWithContext(err,
				fmt.Sprintf("error checking metadata.hcl for pack %s", packEntry.Name()), cache.ErrorContext.GetAll()...)
			continue
		}

		// Attempt to load the pack.
		loadedPack, err = loader.Load(opts.toPackDir(packEntry))
		if err != nil {
			cache.cfg.Logger.Debug(fmt.Sprintf("failed to load pack %s", packEntry.Name()))
			// Add an invalid pack definition if it couldn't be loaded
			invalidOpts := &GetOpts{
				cachePath:    opts.cachePath,
				RegistryName: opts.RegistryName,
				PackName:     packEntry.Name(),
				Ref:          refFromPackEntry(packEntry),
			}
			cachedPack = invalidPackDefinition(invalidOpts)
		} else {
			cachedPack = &Pack{
				Ref:  refFromPackEntry(packEntry),
				Pack: loadedPack,
			}
		}

		// Append the pack to the registry's packs field.
		r.add(cachedPack)

		// reset err to nil in case we handled a recoverable err
		err = nil
	}

	// Set the registry URL from the first pack's URL if a pack exists
	r.setURLFromPacks()

	return
}

// setURLFromPacks sets the Source since we don't have this stored in any sort of
// reliable way.
func (r *Registry) setURLFromPacks() {
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
		r.Source = strings.Join(segments[:segmentCount], "/")

		// Exit once we have a valid pack
		return
	}

	// Set meaningful message if no valid packs found.
	r.Source = "not parsable - registry contains no valid packs"
}

func (r *Registry) add(pack *Pack) {
	r.Packs = append(r.Packs, pack)
}
