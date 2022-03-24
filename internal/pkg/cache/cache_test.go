package cache

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/nomad-pack/internal/pkg/errors"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper/filesystem"
	"github.com/stretchr/testify/require"
)

var (
	// path to be used for cache building clones. Set in TestMain.
	testRegistryURL *string = new(string)
	// testRegistryRef1 is the hash of the first commit in the repo. Set in TestMain.
	testRegistryRef1 *string = new(string)
	// testRegistryRef2 is the hash of the second commit in the repo. Set in TestMain.
	testRegistryRef2 *string = new(string)
)

func testAddOpts(registryName string) *AddOpts {
	return &AddOpts{
		Source:       *testRegistryURL,
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
	require.NoError(t, err)

	for _, dirEntry := range dirEntries {
		if opts.IsTarget(dirEntry) {
			packCount += 1
		}
	}

	return packCount
}

func TestMain(m *testing.M) {
	cleanup := makeTestRegRepo()
	exitCode := m.Run()
	cleanup()
	os.Exit(exitCode)
}

func makeTestRegRepo() func() {
	dirName, err := os.MkdirTemp("", "cache-test-*")
	if err != nil {
		panic(err)
	}
	cleanup := func() { os.RemoveAll(dirName) }
	maybeFatal := func(err error) {
		if err != nil {
			var out string

			switch err := err.(type) {
			case *exec.ExitError:
				out = fmt.Sprintf("Error: %v \nStdErr:\n%s", err, string(err.Stderr))
			default:
				out = fmt.Sprintf("Error: (%T) %v ", err, err)
			}

			cleanup()
			panic(out)
		}
	}
	// fmt.Printf("Temp Dir: %s\n", dirName)

	*testRegistryURL = path.Join(dirName, "test_registry.git")
	maybeFatal(filesystem.CopyDir("../../../fixtures/test_registry", *testRegistryURL, NoopLogger{}))
	var exitErr *exec.ExitError

	formatPanic := func(res GitCommandResult) {
		var out strings.Builder
		out.WriteString(fmt.Sprintf("git err: %v running %v\n", res.err.Error(), res.cmd))
		out.WriteString(fmt.Sprintf("stdout:\n%s\nstderr:\n%s\n", res.stdout, res.stderr))
		// If these setup git commands fail, there's no use in continuing
		// because almost all of the cache tests will fail. Could these
		// be refactored into a sync.Once and a check for cache tests?
		cleanup()
		panic(out)
	}

	handleInitError := func(res GitCommandResult) {
		if res.err != nil && errors.As(res.err, &exitErr) && !strings.Contains(
			res.stdout,
			"Initialized empty Git repository",
		) {
			formatPanic(res)
		}
	}

	handleGitError := func(res GitCommandResult) {
		if res.err != nil {
			formatPanic(res)
		}
	}

	git(handleInitError, "init")
	git(handleGitError, "config", "user.email", "test@example.com")
	git(handleGitError, "config", "user.name", "Github Action Test User")
	git(handleGitError, "add", ".")
	git(handleGitError, "commit", "-m", "Initial Commit")
	res := git(handleGitError, "log", "-1", `--pretty=%H`)
	*testRegistryRef1 = strings.TrimSpace(res.stdout)

	git(handleGitError, "commit", "--allow-empty", "-m", "Second Commit")
	res = git(handleGitError, "log", "-1", `--pretty=%H`)
	*testRegistryRef2 = strings.TrimSpace(res.stdout)

	return cleanup
}

func git(errFn func(GitCommandResult), args ...string) GitCommandResult {
	git := exec.Command("git", args...)
	git.Dir = *testRegistryURL
	oB := new(bytes.Buffer)
	eB := new(bytes.Buffer)
	git.Stdout = oB
	git.Stderr = eB
	err := git.Run()
	res := GitCommandResult{
		exitCode: git.ProcessState.ExitCode(),
		cmd:      git,
		err:      err,
		stdout:   oB.String(),
		stderr:   eB.String(),
	}
	errFn(res)
	return res
}

type GitCommandResult struct {
	cmd      *exec.Cmd
	exitCode int
	err      error
	stdout   string
	stderr   string
}

func TestListRegistries(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("list-registries")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	expected := testPackCount(t, opts)
	require.Equal(t, expected, len(registry.Packs))
}

func TestAddRegistry(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("add-registry")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	expected := testPackCount(t, opts)
	require.NoError(t, err)
	require.Equal(t, expected, len(registry.Packs))
}

func TestAddRegistryPacksAtMultipleRefs(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	testOpts := testAddOpts("multiple-refs")
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "multiple-refs",
		Source:       *testRegistryURL,
		Ref:          *testRegistryRef1,
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Add at ref
	registry, err := cache.Add(addOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	// Add at latest
	registry, err = cache.Add(testOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	expected := testPackCount(t, testOpts)

	// test that registry still exists
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Equal(t, 1, len(registryEntries))

	// test that multiple refs of pack exist
	packEntries, err := os.ReadDir(path.Join(cacheDir, "multiple-refs"))
	require.NoError(t, err)

	require.Equal(t, expected, len(packEntries))
}

func TestAddRegistryWithTarget(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()

	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-target",
		PackName:     "simple_raw_exec",
		Source:       *testRegistryURL,
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Add at ref
	registry, err := cache.Add(addOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	require.Len(t, registry.Packs, 1)
}

func TestAddRegistryWithSHA(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-sha",
		Source:       *testRegistryURL,
		Ref:          *testRegistryRef1,
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Add at SHA
	registry, err := cache.Add(addOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	// expected testCount
	expected := testPackCount(t, addOpts)

	require.Equal(t, expected, len(registry.Packs))
}

func TestAddRegistryWithRefAndPackName(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	// Set opts at sha ref
	addOpts := &AddOpts{
		cachePath:    cacheDir,
		RegistryName: "with-ref-and-pack-name",
		PackName:     "simple_raw_exec",
		Source:       *testRegistryURL,
		Ref:          *testRegistryRef1,
	}

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	// Add Ref and PackName
	registry, err := cache.Add(addOpts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	require.Len(t, registry.Packs, 1)
}

func TestAddRegistryNoCacheDir(t *testing.T) {
	opts := testAddOpts("no-cache-dir")

	cache, err := NewCache(&CacheConfig{
		Path:   "",
		Logger: NewTestLogger(t),
	})
	require.Error(t, err)

	registry, err := cache.Add(opts)

	require.Error(t, err)
	require.Nil(t, registry)
	require.Equal(t, errors.ErrCachePathRequired, err)
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
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)

	require.Error(t, err)
	require.Nil(t, registry)
	require.Equal(t, errors.ErrRegistrySourceRequired, err)
}

func TestDeleteRegistry(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("delete-registry")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "",
		Ref:          "",
	}

	err = cache.Delete(deleteOpts)
	require.NoError(t, err)

	// test that registry is gone
	registryEntries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)

	for _, registryEntry := range registryEntries {
		require.NotEqual(t, registryEntry.Name(), deleteOpts.RegistryName)
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
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	packCount := testPackCount(t, opts)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "simple_raw_exec",
		Ref:          opts.Ref,
	}

	err = cache.Delete(deleteOpts)
	require.NoError(t, err)

	// test that pack is gone
	registryEntries, err := os.ReadDir(opts.RegistryPath())
	require.NoError(t, err)

	for _, packEntry := range registryEntries {
		require.NotEqual(t, packEntry.Name(), deleteOpts.RegistryName)
	}

	// test that registry still exists, and other packs are still  there.
	require.NotEqual(t, 0, packCount)
	require.Equal(t, packCount-1, len(registryEntries))
}

func TestDeletePackByRef(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	opts := testAddOpts("delete-pack-by-ref")

	cache, err := NewCache(&CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	require.NoError(t, err)
	require.NotNil(t, cache)

	registry, err := cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	// Now add at different ref
	opts.Ref = *testRegistryRef1
	registry, err = cache.Add(opts)
	require.NoError(t, err)
	require.NotNil(t, registry)

	packCount := testPackCount(t, opts)

	deleteOpts := &DeleteOpts{
		RegistryName: opts.RegistryName,
		PackName:     "simple_raw_exec",
		Ref:          opts.Ref,
	}

	err = cache.Delete(deleteOpts)
	require.NoError(t, err)

	// test that pack is gone
	registryEntries, err := os.ReadDir(opts.RegistryPath())
	require.NoError(t, err)

	for _, packEntry := range registryEntries {
		require.NotEqual(t, packEntry.Name(), deleteOpts.RegistryName)
	}

	// test that registry still exists, and other packs are still  there.
	require.NotEqual(t, 0, packCount)
	require.Equal(t, packCount-1, len(registryEntries))
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
				require.True(t, ok)
				require.Equal(t, tc.expectedResult, reg.Source)
			} else {
				require.False(t, ok)
				// If we get an error, reg.Source should be unset.
				require.True(t, strings.Contains(reg.Source, "invalid url") || reg.Source == "")
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
	l.t.Log(fmt.Sprintf("err: %s", err))
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
// for any of the methods. This can be useful for cases that require a Logger as
// a parameter.
type NoopLogger struct{}

func (_ NoopLogger) Debug(_ string)                                  {}
func (_ NoopLogger) Error(_ string)                                  {}
func (_ NoopLogger) ErrorWithContext(_ error, _ string, _ ...string) {}
func (_ NoopLogger) Info(_ string)                                   {}
func (_ NoopLogger) Trace(_ string)                                  {}
func (_ NoopLogger) Warning(_ string)                                {}
