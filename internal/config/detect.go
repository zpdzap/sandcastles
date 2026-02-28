package config

import (
	"os"
	"path/filepath"
)

type Detection struct {
	Language     string
	Packages     []string
	Ports        []int
	DockerSocket bool
	Setup        []string
}

// Detect inspects the project directory and returns language, suggested packages, and ports.
func Detect(projectDir string) Detection {
	checks := []struct {
		file     string
		language string
		packages []string
		ports    []int
		setup    []string
	}{
		{"go.mod", "go", []string{"golang-go", "git", "curl", "make", "lsof"}, []int{8080}, nil},
		{"package.json", "node", []string{"nodejs", "npm", "git", "curl", "make", "lsof"}, []int{3000}, []string{"cd /workspace && npm install"}},
		{"requirements.txt", "python", []string{"python3", "python3-pip", "git", "curl", "make", "lsof"}, []int{8000}, []string{"cd /workspace && pip install -r requirements.txt"}},
		{"Cargo.toml", "rust", []string{"rustc", "cargo", "git", "curl", "make", "lsof"}, []int{8080}, nil},
		{"pyproject.toml", "python", []string{"python3", "python3-pip", "git", "curl", "make", "lsof"}, []int{8000}, []string{"cd /workspace && pip install -e ."}},
	}

	var det Detection
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(projectDir, c.file)); err == nil {
			det = Detection{
				Language: c.language,
				Packages: c.packages,
				Ports:    c.ports,
				Setup:    c.setup,
			}
			break
		}
	}

	if det.Language == "" {
		det = Detection{
			Language: "unknown",
			Packages: []string{"git", "curl", "make", "lsof"},
			Ports:    nil,
		}
	}

	// Detect docker-compose files
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"docker-compose.test.yml",
		"compose.yml",
		"compose.yaml",
	}
	for _, f := range composeFiles {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err == nil {
			det.DockerSocket = true
			break
		}
	}

	return det
}
