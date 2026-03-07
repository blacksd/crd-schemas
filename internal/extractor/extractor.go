// Package extractor handles fetching CRDs from upstream sources and
// extracting their openAPIV3Schema as standalone JSON Schema files.
package extractor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/blacksd/crd-schemas/internal/source"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	httpTimeout   = 60 * time.Second
	maxRetries    = 3
	retryInterval = 2 * time.Second
)

// CRDSchema represents an extracted JSON Schema for a single CRD version.
type CRDSchema struct {
	Group      string
	Kind       string
	APIVersion string
	Schema     json.RawMessage
	SourceName string // tracks which source produced this schema
}

// Conflict represents a schema key produced by multiple sources with different content.
type Conflict struct {
	Group      string
	Kind       string
	APIVersion string
	Sources    []string
}

// Extract fetches CRDs from the given source and returns extracted schemas.
func Extract(log zerolog.Logger, src source.Source) ([]CRDSchema, error) {
	switch src.Type {
	case "helm":
		return extractFromHelm(log, src)
	case "url":
		return extractFromURL(log, src)
	default:
		return nil, fmt.Errorf("unknown source type: %s", src.Type)
	}
}

func extractFromHelm(log zerolog.Logger, src source.Source) ([]CRDSchema, error) {
	tmpDir, err := os.MkdirTemp("", "crd-schemas-helm-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	chartDir := filepath.Join(tmpDir, "chart")
	isOCI := strings.HasPrefix(src.Repo, "oci://")

	if isOCI {
		// OCI registries: pull directly, no repo add/update needed.
		// The chart ref is the full OCI URI with the chart name appended.
		chartRef := strings.TrimSuffix(src.Repo, "/") + "/" + src.Chart
		log.Debug().Str("chart", chartRef).Msg("pulling chart from OCI registry")

		helmVersion := strings.TrimPrefix(src.Version, "v")
		err = runHelm(log, "pull", chartRef, "--version", helmVersion, "--untar", "--untardir", chartDir)
		if err != nil {
			log.Debug().Str("version", src.Version).Msg("retrying pull with original version string")
			err = runHelm(log, "pull", chartRef, "--version", src.Version, "--untar", "--untardir", chartDir)
			if err != nil {
				return nil, fmt.Errorf("helm pull %s@%s: %w", chartRef, src.Version, err)
			}
		}
	} else {
		// HTTP repositories: add repo, update, then pull via alias.
		repoAlias := "crd-schemas-" + src.Name

		log.Debug().Str("repo", src.Repo).Str("alias", repoAlias).Msg("adding helm repo")
		if err := runHelm(log, "repo", "add", repoAlias, src.Repo, "--force-update"); err != nil {
			return nil, fmt.Errorf("helm repo add: %w", err)
		}
		log.Debug().Str("alias", repoAlias).Msg("updating helm repo")
		if err := runHelm(log, "repo", "update", repoAlias); err != nil {
			return nil, fmt.Errorf("helm repo update: %w", err)
		}

		helmVersion := strings.TrimPrefix(src.Version, "v")
		chartRef := repoAlias + "/" + src.Chart

		log.Debug().Str("chart", chartRef).Str("version", helmVersion).Msg("pulling chart")
		err = runHelm(log, "pull", chartRef, "--version", helmVersion, "--untar", "--untardir", chartDir)
		if err != nil {
			log.Debug().Str("version", src.Version).Msg("retrying pull with original version string")
			err = runHelm(log, "pull", chartRef, "--version", src.Version, "--untar", "--untardir", chartDir)
			if err != nil {
				return nil, fmt.Errorf("helm pull %s@%s: %w", src.Chart, src.Version, err)
			}
		}
	}

	var allSchemas []CRDSchema

	// Extract CRDs from the crds/ directory
	crdsDir := filepath.Join(chartDir, src.Chart, "crds")
	if entries, err := os.ReadDir(crdsDir); err == nil {
		log.Debug().Str("dir", crdsDir).Int("files", len(entries)).Msg("scanning crds directory")
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(crdsDir, name))
			if err != nil {
				continue
			}
			schemas, err := parseCRDs(log, data)
			if err != nil {
				log.Warn().Err(err).Str("source", src.Name).Str("file", name).Msg("parsing CRDs failed")
				continue
			}
			allSchemas = append(allSchemas, schemas...)
		}
	} else {
		log.Debug().Str("dir", crdsDir).Msg("no crds directory found, skipping")
	}

	// Also try helm template for charts that generate CRDs via templates
	templateArgs := []string{"template", "release",
		filepath.Join(chartDir, src.Chart),
		"--include-crds", "--no-hooks"}
	for k, v := range src.Values {
		templateArgs = append(templateArgs, "--set", k+"="+v)
	}
	log.Debug().Strs("args", templateArgs).Msg("running helm template")
	templateCmd := exec.Command("helm", templateArgs...)
	out, err := templateCmd.Output()
	if err == nil {
		schemas, err := parseCRDs(log, out)
		if err == nil {
			allSchemas = append(allSchemas, schemas...)
		}
	} else {
		log.Debug().Err(err).Msg("helm template produced no output")
	}

	return dedup(allSchemas), nil
}

func extractFromURL(log zerolog.Logger, src source.Source) ([]CRDSchema, error) {
	client := &http.Client{Timeout: httpTimeout}

	var resp *http.Response
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Debug().Str("url", src.URL).Int("attempt", attempt).Msg("fetching manifest")

		var err error
		resp, err = client.Get(src.URL)
		if err != nil {
			lastErr = fmt.Errorf("downloading %s: %w", src.URL, err)
			log.Warn().Err(err).Int("attempt", attempt).Int("max", maxRetries).Msg("fetch failed")
			if attempt < maxRetries {
				time.Sleep(retryInterval)
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("downloading %s: HTTP %d", src.URL, resp.StatusCode)
			log.Warn().Int("status", resp.StatusCode).Int("attempt", attempt).Int("max", maxRetries).Msg("fetch returned non-200")
			if attempt < maxRetries {
				time.Sleep(retryInterval)
			}
			continue
		}

		lastErr = nil
		break
	}

	if lastErr != nil {
		return nil, lastErr
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", src.URL, err)
	}

	log.Debug().Str("url", src.URL).Int("bytes", len(data)).Msg("manifest downloaded")
	return parseCRDs(log, data)
}

// parseCRDs parses a multi-document YAML byte slice and extracts CRD schemas.
// It first decodes each document as a generic object to check the kind, then
// re-marshals CRD documents into the typed struct for proper schema extraction.
func parseCRDs(log zerolog.Logger, data []byte) ([]CRDSchema, error) {
	var schemas []CRDSchema

	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)

	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		// Quick check: is this a CRD?
		var meta struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil || meta.Kind != "CustomResourceDefinition" {
			continue
		}

		// Full decode into the typed CRD struct
		var crd apiextensionsv1.CustomResourceDefinition
		if err := json.Unmarshal(raw, &crd); err != nil {
			log.Warn().Err(err).Msg("decoding CRD")
			continue
		}

		if crd.Spec.Group == "" {
			continue
		}

		for _, version := range crd.Spec.Versions {
			if !version.Served {
				log.Debug().
					Str("group", crd.Spec.Group).
					Str("kind", crd.Spec.Names.Kind).
					Str("version", version.Name).
					Msg("skipping unserved version")
				continue
			}

			if version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
				log.Debug().
					Str("group", crd.Spec.Group).
					Str("kind", crd.Spec.Names.Kind).
					Str("version", version.Name).
					Msg("skipping version without openAPIV3Schema")
				continue
			}

			schemaJSON, err := json.MarshalIndent(version.Schema.OpenAPIV3Schema, "", "  ")
			if err != nil {
				log.Warn().Err(err).
					Str("group", crd.Spec.Group).
					Str("kind", crd.Spec.Names.Kind).
					Str("version", version.Name).
					Msg("marshaling schema")
				continue
			}

			log.Debug().
				Str("group", crd.Spec.Group).
				Str("kind", crd.Spec.Names.Kind).
				Str("version", version.Name).
				Int("bytes", len(schemaJSON)).
				Msg("extracted schema")

			schemas = append(schemas, CRDSchema{
				Group:      crd.Spec.Group,
				Kind:       crd.Spec.Names.Kind,
				APIVersion: version.Name,
				Schema:     schemaJSON,
			})
		}
	}

	return schemas, nil
}

func runHelm(log zerolog.Logger, args ...string) error {
	log.Debug().Strs("args", args).Msg("exec helm")
	cmd := exec.Command("helm", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dedup(schemas []CRDSchema) []CRDSchema {
	seen := make(map[string]bool)
	var result []CRDSchema
	for _, s := range schemas {
		key := fmt.Sprintf("%s/%s/%s", s.Group, s.Kind, s.APIVersion)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, s)
	}
	return result
}

// SchemaKey returns the unique identifier for a schema: "group/Kind/apiVersion".
func SchemaKey(s CRDSchema) string {
	return fmt.Sprintf("%s/%s/%s", s.Group, s.Kind, s.APIVersion)
}

// Filter applies include/exclude rules from the source config to a set of
// extracted schemas. Rules can match by:
//   - Kind alone: "GatewayClass"
//   - group/Kind: "gateway.networking.k8s.io/GatewayClass"
//   - group wildcard: "gateway.networking.k8s.io/*"
//
// If Include is non-empty, only matching schemas are kept (allowlist).
// If Exclude is non-empty, matching schemas are removed (denylist).
// Include takes precedence: if both are set, only Include is evaluated.
func Filter(schemas []CRDSchema, src source.Source) []CRDSchema {
	if len(src.Include) == 0 && len(src.Exclude) == 0 {
		return schemas
	}

	if len(src.Include) > 0 {
		var result []CRDSchema
		for _, s := range schemas {
			if matchesAny(s, src.Include) {
				result = append(result, s)
			}
		}
		return result
	}

	// Exclude mode
	var result []CRDSchema
	for _, s := range schemas {
		if !matchesAny(s, src.Exclude) {
			result = append(result, s)
		}
	}
	return result
}

// matchesAny checks if a schema matches any of the given filter patterns.
func matchesAny(s CRDSchema, patterns []string) bool {
	for _, p := range patterns {
		if matchPattern(s, p) {
			return true
		}
	}
	return false
}

// matchPattern checks if a schema matches a single filter pattern.
func matchPattern(s CRDSchema, pattern string) bool {
	if idx := strings.Index(pattern, "/"); idx >= 0 {
		group := pattern[:idx]
		kind := pattern[idx+1:]
		if s.Group != group {
			return false
		}
		return kind == "*" || strings.EqualFold(s.Kind, kind)
	}
	// Kind-only match (case-insensitive)
	return strings.EqualFold(s.Kind, pattern)
}

// DetectConflicts checks a collection of schemas from multiple sources for
// conflicting entries: same group/kind/version but different schema content.
// Returns nil if no conflicts are found. Identical duplicates are logged
// as informational messages but not treated as conflicts.
func DetectConflicts(log zerolog.Logger, allSchemas []CRDSchema) []Conflict {
	type entry struct {
		schema     json.RawMessage
		sourceName string
	}

	seen := make(map[string][]entry)
	for _, s := range allSchemas {
		key := SchemaKey(s)
		seen[key] = append(seen[key], entry{schema: s.Schema, sourceName: s.SourceName})
	}

	var conflicts []Conflict
	for key, entries := range seen {
		if len(entries) < 2 {
			continue
		}

		// Check if all entries are identical
		allIdentical := true
		for i := 1; i < len(entries); i++ {
			if !jsonEqual(entries[0].schema, entries[i].schema) {
				allIdentical = false
				break
			}
		}

		parts := strings.SplitN(key, "/", 3)
		sources := make([]string, len(entries))
		for i, e := range entries {
			sources[i] = e.sourceName
		}

		if allIdentical {
			log.Info().Str("schema", key).Strs("sources", sources).Msg("duplicate schema with identical content")
			continue
		}

		conflicts = append(conflicts, Conflict{
			Group:      parts[0],
			Kind:       parts[1],
			APIVersion: parts[2],
			Sources:    sources,
		})
	}

	return conflicts
}

// jsonEqual compares two JSON byte slices for semantic equality
// (ignoring formatting differences).
func jsonEqual(a, b json.RawMessage) bool {
	var va, vb interface{}
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	// Re-marshal to canonical form for comparison
	ca, err := json.Marshal(va)
	if err != nil {
		return false
	}
	cb, err := json.Marshal(vb)
	if err != nil {
		return false
	}
	return string(ca) == string(cb)
}
