package sandbox

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zpdzap/sandcastles/internal/config"
)

// manifestFiles maps language to the dependency manifest files to track.
var manifestFiles = map[string][]string{
	"node":   {"package.json", "package-lock.json"},
	"go":     {"go.mod", "go.sum"},
	"python": {"requirements.txt", "pyproject.toml"},
	"rust":   {"Cargo.toml", "Cargo.lock"},
}

// manifestHash computes a SHA256 hash of all relevant manifest file contents
// for the given language. Files that don't exist are skipped.
func manifestHash(projectDir, language string) string {
	files := manifestFiles[language]
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	h := sha256.New()
	for _, f := range sorted {
		data, err := os.ReadFile(filepath.Join(projectDir, f))
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s:%x\n", f, sha256.Sum256(data))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// warmImageName returns the warm image tag for the project.
func warmImageName(project string) string {
	return fmt.Sprintf("sc-%s:warm", project)
}

// warmHashPath returns the path to the .warm-hash file.
func warmHashPath(projectDir string) string {
	return filepath.Join(projectDir, config.Dir, ".warm-hash")
}

// computeWarmHash combines the base image ID and manifest hash into a single hash.
func computeWarmHash(projectDir, baseImageID, language string) string {
	mh := manifestHash(projectDir, language)
	combined := fmt.Sprintf("%s:%s", baseImageID, mh)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(combined)))
}

// warmImageUpToDate checks if the warm image hash matches the current state.
func warmImageUpToDate(projectDir, baseImageID, language string) bool {
	stored, err := os.ReadFile(warmHashPath(projectDir))
	if err != nil {
		return false
	}
	current := computeWarmHash(projectDir, baseImageID, language)
	return strings.TrimSpace(string(stored)) == current
}

// saveWarmHash writes the current warm hash to disk.
func saveWarmHash(projectDir, baseImageID, language string) {
	hash := computeWarmHash(projectDir, baseImageID, language)
	os.WriteFile(warmHashPath(projectDir), []byte(hash+"\n"), 0o644)
}

// removeWarmHash deletes the warm hash file.
func removeWarmHash(projectDir string) {
	os.Remove(warmHashPath(projectDir))
}
