package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeBuild(t *testing.T, dir, version string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyStagedCfT(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the cache lives under LOCALAPPDATA on Windows")
	}
	t.Setenv("HOME", t.TempDir())

	if v, err := ApplyStagedCfT("stable"); err != nil || v != "" {
		t.Fatalf("without a staged build: %q, %v", v, err)
	}

	dir := ChromeForTestingDir("stable")
	writeBuild(t, dir, "1.0")
	writeBuild(t, CfTStagingDir("stable"), "2.0")

	v, err := ApplyStagedCfT("stable")
	if err != nil || v != "2.0" {
		t.Fatalf("ApplyStagedCfT = %q, %v, want 2.0", v, err)
	}
	if got := CfTVersion(dir); got != "2.0" {
		t.Errorf("installed version = %q, want 2.0", got)
	}
	for _, gone := range []string{CfTStagingDir("stable"), dir + ".old"} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s is left behind", gone)
		}
	}
}

func TestApplyStagedCfTWithoutInstalledBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the cache lives under LOCALAPPDATA on Windows")
	}
	t.Setenv("HOME", t.TempDir())
	writeBuild(t, CfTStagingDir("beta"), "3.0")

	if v, err := ApplyStagedCfT("beta"); err != nil || v != "3.0" {
		t.Fatalf("ApplyStagedCfT = %q, %v, want 3.0", v, err)
	}
	if got := CfTVersion(ChromeForTestingDir("beta")); got != "3.0" {
		t.Errorf("installed version = %q, want 3.0", got)
	}
}

func TestCfTVersionMissing(t *testing.T) {
	if got := CfTVersion(t.TempDir()); got != "" {
		t.Errorf("CfTVersion of an empty dir = %q", got)
	}
}
