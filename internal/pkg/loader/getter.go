package loader


import (
	"fmt"
	"path"
	"context"
	"net/http"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/nomad-pack/sdk/pack"
	getter "github.com/hashicorp/go-getter"
)

func GetDependecy(dependency *pack.Dependency, depsPath string) error {

	var httpClient = &http.Client{
		Transport: cleanhttp.DefaultPooledTransport(),
	}

	httpGetter := &getter.HttpGetter{
		Netrc:  true,
		Client: httpClient,
	}

	client := &getter.Client{
		Ctx: context.Background(),
		//define the destination to where the directory will be stored. This will create the directory if it doesnt exist
		Dst: path.Join(depsPath, dependency.Name),
		Dir: true,
		//the repository with a subdirectory I would like to clone only
		Src:  dependency.Source,
		Mode: getter.ClientModeDir,
		//define the type of detectors go getter should use, in this case only github is needed
		Detectors: []getter.Detector{
			&getter.GitHubDetector{},
			&getter.GitDetector{},
			&getter.FileDetector{},

		},
		//provide the getter needed to download the files
		Getters: map[string]getter.Getter{
			"git":   new(getter.GitGetter),
			"file":  &getter.FileGetter{Copy: true},
			"http":  httpGetter,
			"https": httpGetter,
		},
	}

	//download the files
	if err := client.Get(); err != nil {
		return fmt.Errorf("Failed to get source pack: %v", err)
	}

	return nil
}

