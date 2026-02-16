// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/posener/complete"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad-pack/internal/pkg/caching"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/internal/pkg/logging"
	"github.com/hashicorp/nomad-pack/internal/pkg/testfixture"
)

func TestExtractFlagValue(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		flag     string
		expected string
	}{
		{
			name:     "space_format",
			args:     []string{"--registry", "default"},
			flag:     "registry",
			expected: "default",
		},
		{
			name:     "equals_format",
			args:     []string{"--registry=custom"},
			flag:     "registry",
			expected: "custom",
		},
		{
			name:     "missing_flag",
			args:     []string{"--other", "value"},
			flag:     "registry",
			expected: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expected, extractFlagValue(tc.args, tc.flag))
		})
	}
}

func TestPredictPackName_AutocompleteArgs(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CACHE_HOME", tempHome)

	cachePath := caching.DefaultCachePath()

	createRegistryWithPacks(t, cachePath, "reg-a", "latest", map[string]string{
		"simple_raw_exec": "v2/test_registry/packs/simple_raw_exec",
		"simple_docker":   "v2/test_registry/packs/simple_docker",
	})
	createRegistryWithPacks(t, cachePath, "reg-b", "latest", map[string]string{
		"simple_raw_exec": "v2/test_registry/packs/simple_raw_exec",
	})

	testCases := []struct {
		name     string
		args     complete.Args
		expected []string
	}{
		{
			name:     "all_packs",
			args:     complete.Args{All: []string{}},
			expected: []string{"simple_raw_exec", "simple_docker"},
		},
		{
			name:     "filter_registry_space",
			args:     complete.Args{All: []string{"--registry", "reg-a"}},
			expected: []string{"simple_raw_exec", "simple_docker"},
		},
		{
			name:     "filter_registry_equals",
			args:     complete.Args{All: []string{"--registry=reg-b"}},
			expected: []string{"simple_raw_exec"},
		},
		{
			name:     "missing_registry",
			args:     complete.Args{All: []string{"--registry", "missing"}},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := predictPackName.Predict(tc.args)
			assertPredictorResults(t, got, tc.expected)
		})
	}
}

func createRegistryWithPacks(t *testing.T, cachePath, name, ref string, packs map[string]string) {
	t.Helper()

	regRefDir := path.Join(cachePath, name, ref)
	must.NoError(t, os.MkdirAll(regRefDir, 0755))

	for packName, fixtureRel := range packs {
		src := testfixture.AbsPath(t, fixtureRel)
		dest := path.Join(regRefDir, packName+"@"+ref)
		must.NoError(t, filesystem.CopyDir(src, dest, false, logging.Default()))
	}

	reg := &caching.Registry{
		Name:     name,
		Source:   "example.com/" + name,
		LocalRef: ref,
		Ref:      ref,
	}

	b, err := json.Marshal(reg)
	must.NoError(t, err)
	must.NoError(t, os.WriteFile(path.Join(regRefDir, "metadata.json"), b, 0644))
}

func assertPredictorResults(t *testing.T, got, expected []string) {
	t.Helper()
	must.Eq(t, len(expected), len(got))
	must.SliceContainsAll(t, expected, got)
}
