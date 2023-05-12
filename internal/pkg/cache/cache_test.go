// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
)

var (
	tReg         *TestGithubRegistry
	tRegInitOnce sync.Once
)

func TestMain(m *testing.M) {
	tReg = GetTestGithubRegistry()
	exitCode := m.Run()
	tReg.Cleanup()
	os.Exit(exitCode)
}

func TestListRegistries(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("list-registries")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	registry, err := cache.Add(opts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	expected := testPackCount(t, opts)
	must.Eq(t, expected, len(registry.Packs))
}

func TestAddRegistry(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("add-registry")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	registry, err := cache.Add(opts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	expected := testPackCount(t, opts)
	must.NoError(t, err)
	must.Eq(t, expected, len(registry.Packs))
}

func TestAddRegistryPacksAtMultipleRefs(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	testOpts := testAddOpts("multiple-refs")
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "multiple-refs",
		Source:       tReg.SourceURL(),
		Ref:          tReg.Ref1(),
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	// Add at ref
	registry, err := cache.Add(addOpts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	// Add at latest
	registry, err = cache.Add(testOpts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	expected := testPackCount(t, testOpts) + 1 // to account for top-level metadata.json

	// test that registry still exists
	registryEntries, err := os.ReadDir(cacheDir)
	must.NoError(t, err)
	must.Eq(t, 1, len(registryEntries))

	// test that multiple refs of pack exist
	packEntries, err := os.ReadDir(path.Join(cacheDir, "multiple-refs"))
	must.NoError(t, err)

	must.Eq(t, expected, len(packEntries))
}

func TestAddRegistryWithTarget(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()

	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-target",
		PackName:     "simple_raw_exec",
		Source:       tReg.SourceURL(),
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	// Add at ref
	registry, err := cache.Add(addOpts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	must.Eq(t, len(registry.Packs), 1)
}

func TestAddRegistryWithSHA(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-sha",
		Source:       tReg.SourceURL(),
		Ref:          tReg.Ref1(),
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	// Add at SHA
	registry, err := cache.Add(addOpts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	// expected testCount
	expected := testPackCount(t, addOpts)
	must.Eq(t, expected, len(registry.Packs))

	// Make sure the metadata file is there and that it contains what we want
	f, err := os.ReadFile(path.Join(cacheDir + "/" + registry.Name + "/metadata.json"))
	must.NoError(t, err)
	r := &Registry{}
	must.NoError(t, json.Unmarshal(f, r))
	expectedRegistryMetadata := &Registry{
		Name:     "with-sha",
		Source:   "github.com/hashicorp/nomad-pack/fixtures/test_registry",
		LocalRef: tReg.Ref1(),
	}
	must.Eq(t, expectedRegistryMetadata, r)

}

func TestAddRegistryWithRefAndPackName(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-ref-and-pack-name",
		PackName:     "simple_raw_exec",
		Source:       tReg.SourceURL(),
		Ref:          tReg.Ref1(),
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	// Add Ref and PackName
	registry, err := cache.Add(addOpts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	must.Eq(t, len(registry.Packs), 1)
}

func TestAddRegistryNoCacheDir(t *testing.T) {
	opts := testAddOpts("no-cache-dir")

	cache, err := NewCache(&CacheConfig{
		Path:   "",
		Logger: NewTestLogger(t),
	})
	must.Error(t, err)

	registry, err := cache.Add(opts)
	must.Error(t, err)
	must.Nil(t, registry)
	must.Eq(t, errors.ErrCachePathRequired, err)
}

func TestAddRegistryNoSource(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("no-source")
	opts.Source = ""

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	registry, err := cache.Add(opts)

	must.Error(t, err)
	must.Nil(t, registry)
	must.Eq(t, errors.ErrRegistrySourceRequired, err)
}

func TestDeleteRegistry(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("delete-registry")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	registry, err := cache.Add(opts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "",
		Ref:          "",
	}

	err = cache.Delete(deleteOpts)
	must.NoError(t, err)

	// test that registry is gone
	registryEntries, err := os.ReadDir(cacheDir)
	must.NoError(t, err)

	for _, registryEntry := range registryEntries {
		must.NotEq(t, registryEntry.Name(), deleteOpts.RegistryName)
	}
}

func TestDeletePack(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("delete-pack")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	registry, err := cache.Add(opts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	packCount := testPackCount(t, opts)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "simple_raw_exec",
		Ref:          opts.Ref,
	}

	err = cache.Delete(deleteOpts)
	must.NoError(t, err)

	// test that pack is gone
	registryEntries, err := os.ReadDir(opts.RegistryPath())
	must.NoError(t, err)

	for _, packEntry := range registryEntries {
		must.NotEq(t, packEntry.Name(), deleteOpts.RegistryName)
	}

	// test that registry still exists, and other packs are still  there.
	must.NotEq(t, 0, packCount)
	must.Eq(t, packCount-1, len(registryEntries)-1)
}

func TestDeletePackByRef(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("delete-pack-by-ref")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, cache)

	registry, err := cache.Add(opts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	// Now add at different ref
	opts.Ref = tReg.Ref1()
	registry, err = cache.Add(opts)
	must.NoError(t, err)
	must.NotNil(t, registry)

	packCount := testPackCount(t, opts)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "simple_raw_exec",
		Ref:          opts.Ref,
	}

	err = cache.Delete(deleteOpts)
	must.NoError(t, err)

	// test that pack is gone
	registryEntries, err := os.ReadDir(opts.RegistryPath())
	must.NoError(t, err)

	for _, packEntry := range registryEntries {
		must.NotEq(t, packEntry.Name(), deleteOpts.RegistryName)
	}

	// test that registry still exists, and other packs are still  there.
	must.NotEq(t, 0, packCount)
	must.Eq(t, packCount-1, len(registryEntries)-1)
}

func TestParsePackURL(t *testing.T) {
	t.Parallel()
	reg := &Registry{}

	testCases := []struct {
		name           string
		path           string
		expectedResult string
		expectOk       bool
	}{
		{
			name:           "empty string",
			path:           "",
			expectedResult: "",
			expectOk:       false,
		},
		{
			name:           "default",
			path:           "https://github.com/hashicorp/nomad-pack-community-registry/packs/simple_service",
			expectedResult: "github.com/hashicorp/nomad-pack-community-registry",
			expectOk:       true,
		},
		{
			name:           "filepath",
			path:           "/Users/voiselle/debugging/path-to-a-registry/packs/simple_service",
			expectedResult: "/Users/voiselle/debugging/path-to-a-registry",
			expectOk:       true,
		},
		{
			name:           "nested-repo",
			path:           "https://gitlab.com/a6281/nomad/my-pack-registry.git/packs/simple_service",
			expectedResult: "gitlab.com/a6281/nomad/my-pack-registry",
			expectOk:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := reg.parsePackURL(tc.path)
			t.Logf("  path: %s\nsource: %s\n    ok: %v\n\n", tc.path, reg.Source, ok)
			if tc.expectOk {
				must.True(t, ok)
				must.Eq(t, tc.expectedResult, reg.Source)
			} else {
				must.False(t, ok)
				// If we get an error, reg.Source should be unset.
				must.True(t, strings.Contains(reg.Source, "invalid url") || reg.Source == "")
			}
		})
	}
}

type TestLogger struct {
	t *testing.T
}

// Debug logs at the DEBUG log level
func (l *TestLogger) Debug(message string) {
	l.t.Helper()
	l.t.Log(message)
}

// Error logs at the ERROR log level
func (l *TestLogger) Error(message string) {
	l.t.Helper()
	l.t.Log(message)
}

// ErrorWithContext logs at the ERROR log level including additional context so
// users can easily identify issues.
func (l *TestLogger) ErrorWithContext(err error, sub string, ctx ...string) {
	l.t.Helper()
	l.t.Logf("err: %s", err)
	l.t.Log(sub)
	for _, entry := range ctx {
		l.t.Log(entry)
	}
}

// Info logs at the INFO log level
func (l *TestLogger) Info(message string) {
	l.t.Helper()
	l.t.Log(message)
}

// Trace logs at the TRACE log level
func (l *TestLogger) Trace(message string) {
	l.t.Helper()
	l.t.Log(message)
}

// Warning logs at the WARN log level
func (l *TestLogger) Warning(message string) {
	l.t.Helper()
	l.t.Log(message)
}

// NewTestLogger returns a test logger suitable for use with the go testing.T log function.
func NewTestLogger(t *testing.T) *TestLogger {
	return &TestLogger{
		t: t,
	}
}

// NoopLogger returns a logger that meets the Logger interface, but does nothing
// for any of the methods. This can be useful for cases that must a Logger as
// a parameter.
type NoopLogger struct{}

func (_ NoopLogger) Debug(_ string)                                  {}
func (_ NoopLogger) Error(_ string)                                  {}
func (_ NoopLogger) ErrorWithContext(_ error, _ string, _ ...string) {}
func (_ NoopLogger) Info(_ string)                                   {}
func (_ NoopLogger) Trace(_ string)                                  {}
func (_ NoopLogger) Warning(_ string)                                {}

type TestGithubRegistry struct {
	sourceURL string
	ref1      string
	ref2      string
	cleanupFn func()
	tmpDir    string
}

// initialize is protected from being called more than once by a sync.Once.
func initialize() {
	tRegInitOnce.Do(func() {
		tReg = new(TestGithubRegistry)
		makeTestRegRepo(tReg)
	})
}

// GetTestGithubRegistry will initialize a cloneable local git registry containing
// the contents of test_registry in the fixtures folder of the project. This local
// git repo is then used to reduce network impacts of registry clone actions in
// the cache test suite.
func GetTestGithubRegistry() *TestGithubRegistry {
	initialize()
	return tReg
}

// SourceURL returns the TestGithubRegistry's SourceURL. Used to make AddOpts
func (t *TestGithubRegistry) SourceURL() string {
	return t.sourceURL
}

// Ref1 returns the TestGithubRegistry's SourceURL. Used to make AddOpts
// for tests that add registries at a reference.
func (t *TestGithubRegistry) Ref1() string {
	return t.ref1
}

// Cleanup is the proper way to run the TestGithubRegistry's internal cleanup
// function.
func (t *TestGithubRegistry) Cleanup() {
	t.cleanupFn()
}

func makeTestRegRepo(tReg *TestGithubRegistry) {
	var err error
	tReg.tmpDir, err = os.MkdirTemp("", "cache-test-*")
	if err != nil {
		panic(fmt.Errorf("unable to create temp dir for test git repo: %w", err))
	}
	tReg.cleanupFn = func() { os.RemoveAll(tReg.tmpDir) }

	tReg.sourceURL = path.Join(tReg.tmpDir, "test_registry.git")
	err = filesystem.CopyDir("../../../fixtures/test_registry", tReg.SourceURL(), NoopLogger{})
	if err != nil {
		tReg.Cleanup()
		panic(fmt.Errorf("unable to copy test fixtures to test gir repo: %v", err))
	}

	// initialize git repo...
	r, err := git.PlainInit(tReg.SourceURL(), false)
	if err != nil {
		tReg.Cleanup()
		panic(fmt.Errorf("unable to initialize test git repo: %v", err))
	}
	// ...and worktree
	w, err := r.Worktree()
	if err != nil {
		tReg.Cleanup()
		panic(fmt.Errorf("unable to initialize worktree for test git repo: %v", err))
	}

	_, err = w.Add(".")
	if err != nil {
		tReg.Cleanup()
		panic(fmt.Errorf("unable to stage test files to test git repo: %v", err))
	}

	commitOptions := &git.CommitOptions{Author: &object.Signature{
		Name:  "Github Action Test User",
		Email: "test@example.com",
		When:  time.Now(),
	}}

	_, err = w.Commit("Initial Commit", commitOptions)
	if err != nil {
		tReg.Cleanup()
		panic(fmt.Errorf("unable to commit test files to test git repo: %v", err))
	}
	head, err := r.Head()
	if err != nil {
		panic(fmt.Errorf("could not get ref of test git repo: %v", err))
	}
	tReg.ref1 = head.Hash().String()

	commitOptions.AllowEmptyCommits = true
	_, err = w.Commit("Second Commit", commitOptions)
	if err != nil {
		tReg.Cleanup()
		panic(fmt.Errorf("unable to commit test files to test git repo: %v", err))
	}
	head, err = r.Head()
	if err != nil {
		panic(fmt.Errorf("could not get ref of test git repo: %v", err))
	}
	tReg.ref2 = head.Hash().String()
}

func testAddOpts(registryName string) *AddOpts {
	return &AddOpts{
		Source:       tReg.SourceURL(),
		RegistryName: registryName,
		PackName:     "",
		Ref:          "",
		Username:     "",
		Password:     "",
	}
}

func testPackCount(t *testing.T, opts cacheOperationProvider) int {
	packCount := 0

	dirEntries, err := os.ReadDir(opts.RegistryPath())
	must.NoError(t, err)

	for _, dirEntry := range dirEntries {
		if opts.IsTarget(dirEntry) {
			packCount += 1
		}
	}

	return packCount
}
