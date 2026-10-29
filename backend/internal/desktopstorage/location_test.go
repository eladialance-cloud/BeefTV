package desktopstorage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRootMigratesLegacyDirectory(t *testing.T) {
	configRoot := isolatedConfigRoot(t)
	legacy := filepath.Join(configRoot, legacyRootName)
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "open_ai_canvas.db"), []byte("legacy-workspace"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(configRoot, currentRootName)
	if got != want {
		t.Fatalf("DefaultRoot() = %q, want %q", got, want)
	}
	if body, err := os.ReadFile(filepath.Join(got, "open_ai_canvas.db")); err != nil || string(body) != "legacy-workspace" {
		t.Fatalf("legacy workspace was not moved: body=%q err=%v", body, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy directory still exists after migration: %v", err)
	}
}

func TestDefaultRootKeepsExistingCurrentDirectory(t *testing.T) {
	configRoot := isolatedConfigRoot(t)
	current := filepath.Join(configRoot, currentRootName)
	legacy := filepath.Join(configRoot, legacyRootName)
	if err := os.MkdirAll(current, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "open_ai_canvas.db"), []byte("current-workspace"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "open_ai_canvas.db"), []byte("legacy-workspace"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got != current {
		t.Fatalf("DefaultRoot() = %q, want current %q", got, current)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy directory should remain when current workspace exists: %v", err)
	}
}

func isolatedConfigRoot(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
