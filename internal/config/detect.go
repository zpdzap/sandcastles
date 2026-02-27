package config

import (
	"os"
	"path/filepath"
)

type Detection struct {
	Language string
	Packages []string
	Ports    []int
}

// Detect inspects the project directory and returns language, suggested packages, and ports.
func Detect(projectDir string) Detection {
	checks := []struct {
		file     string
		language string
		packages []string
		ports    []int
	}{
		{"go.mod", "go", []string{"golang-go", "git", "curl"}, []int{8080}},
		{"package.json", "node", []string{"nodejs", "npm", "git", "curl"}, []int{3000}},
		{"requirements.txt", "python", []string{"python3", "python3-pip", "git", "curl"}, []int{8000}},
		{"Cargo.toml", "rust", []string{"rustc", "cargo", "git", "curl"}, []int{8080}},
		{"pyproject.toml", "python", []string{"python3", "python3-pip", "git", "curl"}, []int{8000}},
	}

	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(projectDir, c.file)); err == nil {
			return Detection{
				Language: c.language,
				Packages: c.packages,
				Ports:    c.ports,
			}
		}
	}

	return Detection{
		Language: "unknown",
		Packages: []string{"git", "curl"},
		Ports:    nil,
	}
}
