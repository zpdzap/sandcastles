package sandbox

import "testing"

func TestCacheVolumes(t *testing.T) {
	tests := []struct {
		language string
		expected int
	}{
		{"node", 1},
		{"go", 2},
		{"python", 1},
		{"rust", 0},
		{"unknown", 0},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			vols := cacheVolumes("myproject", tt.language)
			if len(vols) != tt.expected {
				t.Errorf("expected %d volumes for %s, got %d: %v", tt.expected, tt.language, len(vols), vols)
			}
		})
	}
}
