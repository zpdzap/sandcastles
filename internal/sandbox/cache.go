package sandbox

import "fmt"

// cacheVolumeSpec defines a Docker volume mount for package manager caches.
type cacheVolumeSpec struct {
	volume    string // Docker volume name suffix (prefixed with sc-<project>-)
	mountPath string // Container mount path
}

// cacheVolumeSpecs maps language to cache volume definitions.
var cacheVolumeSpecs = map[string][]cacheVolumeSpec{
	"node": {
		{"npm-cache", "/home/sandcastle/.npm"},
	},
	"go": {
		{"go-cache", "/home/sandcastle/.cache/go-build"},
		{"gomod-cache", "/home/sandcastle/go/pkg/mod"},
	},
	"python": {
		{"pip-cache", "/home/sandcastle/.cache/pip"},
	},
}

// cacheVolumes returns Docker -v arguments for package manager cache volumes.
func cacheVolumes(project, language string) []string {
	specs := cacheVolumeSpecs[language]
	var args []string
	for _, s := range specs {
		volName := fmt.Sprintf("sc-%s-%s", project, s.volume)
		args = append(args, fmt.Sprintf("%s:%s", volName, s.mountPath))
	}
	return args
}
