package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestHash(t *testing.T) {
	dir := t.TempDir()

	// No manifest files -> empty hash (but still deterministic)
	h1 := manifestHash(dir, "unknown")
	if h1 == "" {
		t.Fatal("expected non-empty hash even with no manifests")
	}

	// Create a package.json
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644)
	h2 := manifestHash(dir, "node")
	if h2 == h1 {
		t.Fatal("expected different hash after adding manifest")
	}

	// Same content -> same hash
	h3 := manifestHash(dir, "node")
	if h3 != h2 {
		t.Fatal("expected same hash for same content")
	}

	// Change content -> different hash
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"changed"}`), 0o644)
	h4 := manifestHash(dir, "node")
	if h4 == h2 {
		t.Fatal("expected different hash after content change")
	}
}

func TestWarmHash(t *testing.T) {
	dir := t.TempDir()
	scDir := filepath.Join(dir, ".sandcastles")
	os.MkdirAll(scDir, 0o755)

	// No stored hash -> not up to date
	if warmImageUpToDate(dir, "abc123", "node") {
		t.Fatal("expected not up to date with no stored hash")
	}

	// Save and verify
	saveWarmHash(dir, "abc123", "node")
	if !warmImageUpToDate(dir, "abc123", "node") {
		t.Fatal("expected up to date after saving")
	}

	// Different base image -> stale
	if warmImageUpToDate(dir, "def456", "node") {
		t.Fatal("expected stale with different base image ID")
	}

	// Change manifest -> stale
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"new"}`), 0o644)
	if warmImageUpToDate(dir, "abc123", "node") {
		t.Fatal("expected stale after manifest change")
	}
}
