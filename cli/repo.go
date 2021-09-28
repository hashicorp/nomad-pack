package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type RepoListCommand struct {
	*baseCommand
	packs []string
}

var (
	linuxCachePath   = "~/.cache/nom"
	macCachePath     = "~/Library/Caches/nom"
	windowsCachePath = "%TEMP%/nom"
)

func (c *RepoListCommand) Run(args []string) int {
	// TODO: WithArgs
	if err := c.Init(); err != nil {
		return 1
	}

	c.cmdKey = "repo list"
	var err error

	if c.packs, err = c.ListPacks(); err != nil {
		c.ui.Output(fmt.Sprintf("NOM.RepoListCommand.Run.ListPacks: %#v", err))
		return 1
	}

	if len(c.packs) < 1 {
		c.ui.Output("No packs found")
		return 0
	}

	stepGroup := c.ui.StepGroup()
	for _, pack := range c.packs {
		stepGroup.Add(pack)
	}

	return 0
}

func (c *RepoListCommand) CachePath() string {
	if c.IsLinux() {
		return linuxCachePath
	}

	if c.IsMac() {
		return macCachePath
	}

	if c.IsWindows() {
		return windowsCachePath
	}

	return "./"
}

func (c *RepoListCommand) ListPacks() ([]string, error) {
	var pattern = ".nomad"
	var matches []string

	if _, err := os.Stat(c.CachePath()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err = os.Mkdir(c.CachePath(), 0644); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	err := filepath.Walk(c.CachePath(), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}
