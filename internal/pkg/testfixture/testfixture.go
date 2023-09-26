package testfixture

import (
	"fmt"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/shoenig/test/must"
)

var RelFixtureDir = "fixtures"

// AbsPath returns the absolute path to a fixture inside the FixtureDir
func AbsPath(t *testing.T, fixtureName string) string {
	t.Helper()
	return path.Join(getRepoRoot(t), RelFixtureDir, fixtureName)
}

// Clone creates a test TempDir, copies the given Pack into it, and returns the
// absolute path to the copy.
func Clone(t *testing.T, fPath string) (dest string) {
	t.Helper()
	parts := strings.Split(fPath, "/")

	td := t.TempDir()
	p := AbsPath(t, fPath)
	cmd := exec.Command("cp", "-R", p, td)
	out, err := cmd.CombinedOutput()
	must.NoError(t, err, must.Sprintf("output: %s\n err: %v", out, err))
	return path.Join(td, parts[len(parts)-1])
}

// getRepoRoot uses git rev-parse to locate the top folder in the git repo for
// locating the fixtures folder
func getRepoRoot(t *testing.T) string {
	repoRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	must.NoError(t, err, must.Sprintf("output: %s\n err: %v", repoRoot, err))
	return strings.TrimSpace(string(repoRoot))
}

// MustAbsPath returns the absolute path to a fixture inside the FixtureDir
func MustAbsPath(fixtureName string) string {
	// mustGetRepoRoot will panic on error, so this becomes a Must func too
	return path.Join(mustGetRepoRoot(), RelFixtureDir, fixtureName)
}

// Clone creates a test TempDir, copies the given Pack into it, and returns the
// absolute path to the copy.
func MustClone(dst string, fPath string) (dest string) {
	p := MustAbsPath(fPath)

	cmd := exec.Command("cp", "-R", p, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("MustClone fatal error:\nerr:%v\nout: %s", err, out))
	}

	parts := strings.Split(fPath, "/")
	return path.Join(dst, parts[len(parts)-1])
}

// mustGetRepoRoot uses git rev-parse to locate the top folder in the git repo for
// locating the fixtures folder. If there is an error running the command, it will
// panic. This function is used in cases where we have no access to *testing.T
func mustGetRepoRoot() string {
	repoRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(repoRoot))
}
