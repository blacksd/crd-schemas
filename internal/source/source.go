// Package source handles parsing of CRD source configuration files.
package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// Source represents a single CRD source entry from a source config file.
type Source struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Repo     string            `json:"repo,omitempty"`
	Chart    string            `json:"chart,omitempty"`
	URL      string            `json:"url,omitempty"`
	Version  string            `json:"version"`
	License  string            `json:"license"`
	Homepage string            `json:"homepage"`
	Values   map[string]string `json:"values,omitempty"`
	Include  []string          `json:"include,omitempty"`
	Exclude  []string          `json:"exclude,omitempty"`
}

// File represents the structure of a source YAML config file.
type File struct {
	Sources []Source `json:"sources"`
}

// LoadAll reads all YAML files from the given directory and returns the
// parsed sources along with the API group derived from the filename.
func LoadAll(dir string) (map[string][]Source, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading sources directory: %w", err)
	}

	result := make(map[string][]Source)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}

		var sf File
		if err := yaml.Unmarshal(data, &sf); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}

		// API group is derived from the filename (e.g. "cert-manager.io.yaml" -> "cert-manager.io")
		apiGroup := strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")

		result[apiGroup] = append(result[apiGroup], sf.Sources...)
	}

	return result, nil
}

// All returns a flat list of all sources across all API groups.
func All(grouped map[string][]Source) []Source {
	var all []Source
	for _, sources := range grouped {
		all = append(all, sources...)
	}
	return all
}
