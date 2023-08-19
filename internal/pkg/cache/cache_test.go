// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cache

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/shoenig/test/must"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/hashicorp/nomad-pack/internal/pkg/testfixture"
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

	expected := len(listAllTestPacks(t, cacheDir))
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

	expected := len(listAllTestPacks(t, cacheDir))
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

	// test that registry still exists
	pts := listAllTestPacks(t, cacheDir)
	must.Eq(t, 1, len(pts.RegistriesUnique()))

	// test that multiple refs of pack exist
	must.Eq(t, 2, len(pts.RefsUnique()))
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

	must.Eq(t, 1, len(registry.Packs))
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
	must.Eq(t, len(listAllTestPacks(t, cacheDir)), len(registry.Packs))

	// Make sure the metadata file is there and that it contains what we want
	f, err := os.ReadFile(path.Join(cacheDir, registry.Name, registry.Ref, "metadata.json"))
	must.NoError(t, err)
	r := &Registry{}
	must.NoError(t, json.Unmarshal(f, r))
	expectedRegistryMetadata := &Registry{
		Name:     "with-sha",
		Source:   tReg.SourceURL(),
		Ref:      tReg.Ref1(),
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

	must.Eq(t, 1, len(registry.Packs))
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

	packTuplesBefore := listAllTestPacks(t, cacheDir)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "simple_raw_exec",
		Ref:          opts.Ref,
	}

	err = cache.Delete(deleteOpts)
	must.NoError(t, err)

	packTuplesAfter := listAllTestPacks(t, cacheDir)

	// test that pack is gone
	for _, pt := range packTuplesAfter {
		must.NotEq(t, deleteOpts.RegistryName+"@"+deleteOpts.Ref, pt.name)
	}

	// test that registry still exists, and other packs are still there.
	must.Eq(t, len(packTuplesBefore.RegistriesUnique()), len(packTuplesAfter.RegistriesUnique()))

	// test that all of the other counts are as expected
	must.Eq(t, len(packTuplesBefore)-1, len(packTuplesAfter))
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

	packTuplesBefore := listAllTestPacks(t, cacheDir)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "simple_raw_exec",
		Ref:          opts.Ref,
	}

	err = cache.Delete(deleteOpts)
	must.NoError(t, err)

	packTuplesAfter := listAllTestPacks(t, cacheDir)

	// test that pack is gone
	for _, pt := range packTuplesAfter {
		must.NotEq(t, deleteOpts.RegistryName+"@"+deleteOpts.Ref, pt.name)
	}

	// test that registry still exists, and other packs are still there.
	must.Eq(t, len(packTuplesBefore.RegistriesUnique()), len(packTuplesAfter.RegistriesUnique()))

	// test that all of the other counts are as expected
	must.Eq(t, len(packTuplesBefore)-1, len(packTuplesAfter))
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

func (NoopLogger) Trace(string)                              {}
func (NoopLogger) Debug(string)                              {}
func (NoopLogger) Info(string)                               {}
func (NoopLogger) Warning(string)                            {}
func (NoopLogger) Error(string)                              {}
func (NoopLogger) ErrorWithContext(error, string, ...string) {}

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
	err = filesystem.CopyDir(testfixture.MustAbsPath("v2/test_registry"), tReg.SourceURL(), false, NoopLogger{})
	if err != nil {
		tReg.Cleanup()
		panic(fmt.Errorf("unable to copy test fixtures to test git repo: %v", err))
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

func dirEntries(t *testing.T, p string) []fs.DirEntry {
	t.Helper()
	dirEntries, err := os.ReadDir(p)
	if err != nil {
		t.Fatalf("error in dirEntries: p: %s err:%s", p, err)
	}
	return dirEntries
}

// packtuple describes each pack found in an entire cache
// directory. The listAllTestPacks produces a packtuples,
// which is an alias for []packtuple
type packtuple struct {
	reg  string
	ref  string
	name string
}

// packtuples is a []packtuple with method receivers on
// them for filtering and query purposes in test cases
type packtuples []packtuple

// RefsUnique produces a sorted, unique value list of
// references in the packtuples
func (p packtuples) RefsUnique() []string {
	out := p.Refs()
	slices.Sort(out)
	return slices.Compact(out)
}

// Refs produces a list of references in this packtuples.
// They are returned in slice order.
func (p packtuples) Refs() []string {
	out := make([]string, len(p))
	for i, p := range p {
		out[i] = p.ref
	}
	return out
}

// RegistriesUnique produces a sorted, unique value list of
// registries in the packtuples
func (p packtuples) RegistriesUnique() []string {
	out := p.Registries()
	slices.Sort(out)
	return slices.Compact(out)
}

// Registries produces a list of references in this packtuples.
// They are returned in slice order.
func (p packtuples) Registries() []string {
	out := make([]string, len(p))
	for i, p := range p {
		out[i] = p.reg
	}
	return out
}

// PacksWithRefs produces a list of packs in this packtuples.
// They are returned in slice order and include their
// `@ref` suffix.
func (p packtuples) PacksWithRefs() []string {
	out := make([]string, len(p))
	for i, p := range p {
		out[i] = p.name
	}
	return out
}

// Packs produces a list of packs in this packtuples.
// They are returned in slice order without their `@ref`
// suffix.
func (p packtuples) Packs() []string {
	out := make([]string, len(p))
	for i, p := range p {
		name := p.name
		atIdx := strings.Index(name, "@")
		if atIdx >= 0 {
			name = name[:atIdx]
		}
		out[i] = name
	}
	return out
}

// String returns a packtuple in `«reg»@«ref»/«packname»` form
func (p packtuple) String() string {
	name := p.name
	atIdx := strings.Index(name, "@")
	if atIdx >= 0 {
		name = name[:atIdx]
	}
	return fmt.Sprintf("%s@%s/%s", p.reg, p.ref, name)
}

// listAllTestPacks is a test helper that uses the filesystem
// to discover and count the registries, refs, and packs in a
// given cachePath
func listAllTestPacks(t *testing.T, cachePath string) packtuples {
	acc := make([]packtuple, 0, 10)
	dirFS := os.DirFS(cachePath)
	fs.WalkDir(dirFS, ".", func(p string, d fs.DirEntry, err error) error {
		// If there is an error opening the initial path, WalkDir
		// calls this function again with an error set.
		if err != nil {
			t.Fatalf("listAllTestPacks: WalkDir error: %v", err)
		}

		if testing.Verbose() && d.IsDir() {
			t.Logf("walking %q...", p)
		}

		pts := strings.Split(p, "/")
		if len(pts) > 3 {
			// If we haven't reached a three-element directory, it can't
			// be a correctly placed pack
			return fs.SkipDir
		}

		if len(pts) == 3 && d.IsDir() && isPack(t, path.Join(cachePath, p), d) {
			// Found a pack; add it to the accumulator
			acc = append(acc, packtuple{reg: pts[0], ref: pts[1], name: pts[2]})
			// We don't need to descend into the pack itself.
			return fs.SkipDir
		}

		if len(pts) == 3 && d.IsDir() {
			// Don't descend into non-pack directories.
			return fs.SkipDir
		}

		// All other cases do nothing
		return nil
	})
	return acc
}

func isPack(t *testing.T, p string, d fs.DirEntry) bool {
	dirEntries := dirEntries(t, path.Join(p))
	return slices.ContainsFunc(dirEntries, hasDir("templates")) &&
		slices.ContainsFunc(dirEntries, hasFile("metadata.hcl"))
}

func hasDir(dir string) func(d fs.DirEntry) bool {
	return func(d fs.DirEntry) bool {
		return d.IsDir() && d.Name() == dir
	}
}

func hasFile(name string) func(d fs.DirEntry) bool {
	return func(d fs.DirEntry) bool {
		return d.Type().IsRegular() && d.Name() == name
	}
}
