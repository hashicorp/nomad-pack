package deps

import (
	"bytes"
	"context"
	"os"
	"path"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad-pack/internal/pkg/cache"
	"github.com/hashicorp/nomad-pack/internal/pkg/helper"
	"github.com/hashicorp/nomad-pack/internal/testui"
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

func TestVendor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cacheDir := t.TempDir()
	globalCache, err := cache.NewCache(&cache.CacheConfig{
		Path:   cacheDir,
		Logger: NewTestLogger(t),
	})
	must.NoError(t, err)
	must.NotNil(t, globalCache)

	uiStdout := new(bytes.Buffer)
	uiStderr := new(bytes.Buffer)
	uiCtx, cancel := helper.WithInterrupt(context.Background())
	ui := testui.NonInteractiveTestUI(uiCtx, uiStdout, uiStderr)

	// first run against an empty directory
	tmpDir1 := t.TempDir()
	err = Vendor(ctx, ui, globalCache, tmpDir1, false)
	must.NotNil(t, err)
	must.ErrorContains(t, err, "does not exist")

	// run against a metadata file with empty dependencies
	tmpDir2 := t.TempDir()
	f, err := os.Create(path.Join(tmpDir2, "metadata.hcl"))
	if err != nil {
		panic(err)
	}

	fw := hclwrite.NewEmptyFile()
	badMetadata := pack.Metadata{
		App:          &pack.MetadataApp{},
		Pack:         &pack.MetadataPack{},
		Integration:  &pack.MetadataIntegration{},
		Dependencies: []*pack.Dependency{},
	}
	gohcl.EncodeIntoBody(&badMetadata, fw.Body())
	_, err = fw.WriteTo(f)
	if err != nil {
		panic(err)
	}

	err = Vendor(ctx, ui, globalCache, tmpDir2, false)
	must.NotNil(t, err)
	must.ErrorContains(t, err, "does not contain any dependencies")

	// test adding to cache
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
