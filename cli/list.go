package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

type ListCommand struct {
	*baseCommand

	fromProject string
	into        string
	update      bool
	from        string
}

func (c *ListCommand) Run(args []string) int {
	// TODO placeholder for actual repo path
	// This whole section with repo path should be deleted in the next phase
	wd, err := os.Getwd()
	if err != nil {
		c.ui.Error(Humanize(err))
		return 1
	}
	// TODO placeholder to get project root; hacky thing to get tests to work
	for path.Base(wd) != "nom" {
		wd = path.Dir(wd)
	}
	//tempRepoPath := path.Join(wd, "repo", "packs")

	fmt.Println("Listing packs...")
	packs, _ := ioutil.ReadDir("./repo/packs")
	for _, pack := range packs {
		//if pack.IsDir() {
			fmt.Println(pack.Name())
		//}
	}
	return 0
}

func (c *ListCommand) Help() string {
	return ""
}

func (c *ListCommand) Synopsis() string {
	return ""
}
