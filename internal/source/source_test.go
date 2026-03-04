package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()

	content := `sources:
  - name: test-operator
    type: helm
    repo: https://example.com/charts
    chart: test-operator
    version: v1.2.3
    license: Apache-2.0
    homepage: https://example.com
    values:
      crds.enabled: "true"
    include:
      - Foo
      - "example.com/Bar"
    exclude:
      - Baz
`
	if err := os.WriteFile(filepath.Join(dir, "example.com.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Non-YAML files should be ignored
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# ignore me"), 0644); err != nil {
		t.Fatal(err)
	}

	grouped, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	sources, ok := grouped["example.com"]
	if !ok {
		t.Fatal("expected API group 'example.com' not found")
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	src := sources[0]
	if src.Name != "test-operator" {
		t.Errorf("name = %q, want %q", src.Name, "test-operator")
	}
	if src.Type != "helm" {
		t.Errorf("type = %q, want %q", src.Type, "helm")
	}
	if src.Repo != "https://example.com/charts" {
		t.Errorf("repo = %q, want %q", src.Repo, "https://example.com/charts")
	}
	if src.Chart != "test-operator" {
		t.Errorf("chart = %q, want %q", src.Chart, "test-operator")
	}
	if src.Version != "v1.2.3" {
		t.Errorf("version = %q, want %q", src.Version, "v1.2.3")
	}
	if src.License != "Apache-2.0" {
		t.Errorf("license = %q, want %q", src.License, "Apache-2.0")
	}
	if src.Homepage != "https://example.com" {
		t.Errorf("homepage = %q, want %q", src.Homepage, "https://example.com")
	}
	if v, ok := src.Values["crds.enabled"]; !ok || v != "true" {
		t.Errorf("values[crds.enabled] = %q, want %q", v, "true")
	}
	if len(src.Include) != 2 {
		t.Fatalf("include length = %d, want 2", len(src.Include))
	}
	if src.Include[0] != "Foo" {
		t.Errorf("include[0] = %q, want %q", src.Include[0], "Foo")
	}
	if src.Include[1] != "example.com/Bar" {
		t.Errorf("include[1] = %q, want %q", src.Include[1], "example.com/Bar")
	}
	if len(src.Exclude) != 1 {
		t.Fatalf("exclude length = %d, want 1", len(src.Exclude))
	}
	if src.Exclude[0] != "Baz" {
		t.Errorf("exclude[0] = %q, want %q", src.Exclude[0], "Baz")
	}
}

func TestLoadAllMultipleSources(t *testing.T) {
	dir := t.TempDir()

	content := `sources:
  - name: source-a
    type: helm
    repo: https://a.example.com
    chart: chart-a
    version: v1.0.0
    license: MIT
    homepage: https://a.example.com
  - name: source-b
    type: url
    url: https://example.com/crds.yaml
    version: v2.0.0
    license: Apache-2.0
    homepage: https://b.example.com
`
	if err := os.WriteFile(filepath.Join(dir, "multi.example.com.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	grouped, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	sources := grouped["multi.example.com"]
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}

	if sources[0].Name != "source-a" {
		t.Errorf("sources[0].name = %q, want %q", sources[0].Name, "source-a")
	}
	if sources[1].Type != "url" {
		t.Errorf("sources[1].type = %q, want %q", sources[1].Type, "url")
	}
}

func TestLoadAllEmptyDir(t *testing.T) {
	dir := t.TempDir()

	grouped, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(grouped) != 0 {
		t.Errorf("expected empty map, got %d entries", len(grouped))
	}
}

func TestLoadAllInvalidYAML(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAll(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadAllMissingDir(t *testing.T) {
	_, err := LoadAll("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing directory, got nil")
	}
}

func TestAll(t *testing.T) {
	grouped := map[string][]Source{
		"a.example.com": {{Name: "a1"}, {Name: "a2"}},
		"b.example.com": {{Name: "b1"}},
	}

	all := All(grouped)
	if len(all) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(all))
	}

	names := map[string]bool{}
	for _, s := range all {
		names[s.Name] = true
	}
	for _, expected := range []string{"a1", "a2", "b1"} {
		if !names[expected] {
			t.Errorf("expected source %q not found in All() result", expected)
		}
	}
}
