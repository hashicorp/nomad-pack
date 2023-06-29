package deps

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	gg "github.com/hashicorp/go-getter"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/nomad-pack/sdk/pack"
)

// at this point we only support git repositories
var validURLPrefixes []string = []string{
	"https:",
	"http:",
	"git::",
	"git:",
}

// Vendor reads the metadata.hcl in the current directory, downloads each of the
// dependencies, and adds them to a "vendor" registry.
func Vendor() error {
	// attempt to read metadata.hcl
	metadata := &pack.Metadata{}
	err := hclsimple.DecodeFile("metadata.hcl", nil, metadata)
	if err != nil {
		return err
	}

	if len(metadata.Dependencies) == 0 {
		return fmt.Errorf("metadata.hcl file does not contain any dependencies")
	}

	// // Get the global cache dir
	// globalCache, err := cache.NewCache(&cache.CacheConfig{
	// 	Path:   cache.DefaultCachePath(),
	// 	Logger: nil,
	// })
	// if err != nil {
	// 	return fmt.Errorf("unable to locate cache: %v", err)
	// }

	// // Load the list of registries.
	// err = globalCache.Load()
	// if err != nil {
	// 	return fmt.Errorf("unable to load global cache: %v", err)
	// }

	var gitGetter = &gg.GitGetter{
		// Set a reasonable timeout for git operations
		Timeout: 30 * time.Second,
	}

	// download each dependency
	for _, d := range metadata.Dependencies {
		// _, err := globalCache.Add(&cache.AddOpts{
		// 	RegistryName: "vendor",
		// 	Source:       d.Source,
		// 	PackName:     d.Name,
		// })
		// if err != nil {
		// 	return err
		// }
		var targetDir string
		if d.Name == "" {
			var err error
			targetDir, err = sourceToPath(d.Source)
			return fmt.Errorf("invalid dependency source URL: %v", err)
		} else {
			targetDir = path.Join("vendor", d.Name)
		}

		u, err := url.Parse(d.Source)
		if err != nil {
			return fmt.Errorf("invalid dependency source URL: %v", err)
		}
		if err := gitGetter.Get(targetDir, u); err != nil {
			return fmt.Errorf("error downloading dependency: %v", err)
		}

	}
	return nil
}

func sourceToPath(source string) (string, error) {
	s := strings.Split(source, "/")
	if len(s) < 3 {
		return "", fmt.Errorf("invalid dependency source URL: %v", source)
	}
	if !slices.Contains(validURLPrefixes, s[0]) {
		return "", errors.New("")
	}
	target := s[len(s)-1]
	return strings.TrimSuffix(target, ".git"), nil
}
