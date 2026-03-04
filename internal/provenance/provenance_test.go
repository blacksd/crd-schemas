package provenance

import (
	"encoding/json"
	"testing"

	"github.com/blacksd/crd-schemas/internal/source"
)

func TestGenerate(t *testing.T) {
	src := source.Source{
		Name:     "test-operator",
		Type:     "helm",
		Repo:     "https://example.com/charts",
		Chart:    "test-operator",
		Version:  "v1.0.0",
		License:  "Apache-2.0",
		Homepage: "https://example.com",
	}

	schemaData := []byte(`{"type": "object"}`)

	data, err := Generate("foo_v1.json", "example.com", "Foo", "v1", "2026-01-01T00:00:00Z", src, schemaData)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal provenance: %v", err)
	}

	if rec.Schema != "foo_v1.json" {
		t.Errorf("schema = %q, want %q", rec.Schema, "foo_v1.json")
	}
	if rec.CRD.Group != "example.com" {
		t.Errorf("crd.group = %q, want %q", rec.CRD.Group, "example.com")
	}
	if rec.CRD.Kind != "Foo" {
		t.Errorf("crd.kind = %q, want %q", rec.CRD.Kind, "Foo")
	}
	if rec.CRD.APIVersion != "v1" {
		t.Errorf("crd.apiVersion = %q, want %q", rec.CRD.APIVersion, "v1")
	}
	if rec.Source.Name != "test-operator" {
		t.Errorf("source.name = %q, want %q", rec.Source.Name, "test-operator")
	}
	if rec.Source.Version != "v1.0.0" {
		t.Errorf("source.version = %q, want %q", rec.Source.Version, "v1.0.0")
	}
	if rec.Extraction.Timestamp != "2026-01-01T00:00:00Z" {
		t.Errorf("extraction.timestamp = %q, want %q", rec.Extraction.Timestamp, "2026-01-01T00:00:00Z")
	}
	if rec.Extraction.ToolVersion != ToolVersion {
		t.Errorf("extraction.toolVersion = %q, want %q", rec.Extraction.ToolVersion, ToolVersion)
	}
	if rec.Integrity.SHA256 == "" {
		t.Error("integrity.sha256 is empty")
	}
}

func TestGenerateDeterministicHash(t *testing.T) {
	src := source.Source{Name: "test", Version: "v1.0.0"}
	schemaData := []byte(`{"type": "object"}`)

	data1, _ := Generate("f.json", "g", "K", "v1", "t", src, schemaData)
	data2, _ := Generate("f.json", "g", "K", "v1", "t", src, schemaData)

	var rec1, rec2 Record
	json.Unmarshal(data1, &rec1)
	json.Unmarshal(data2, &rec2)

	if rec1.Integrity.SHA256 != rec2.Integrity.SHA256 {
		t.Error("same input should produce same hash")
	}
}

func TestGenerateDifferentHashForDifferentInput(t *testing.T) {
	src := source.Source{Name: "test", Version: "v1.0.0"}

	data1, _ := Generate("f.json", "g", "K", "v1", "t", src, []byte(`{"type": "object"}`))
	data2, _ := Generate("f.json", "g", "K", "v1", "t", src, []byte(`{"type": "string"}`))

	var rec1, rec2 Record
	json.Unmarshal(data1, &rec1)
	json.Unmarshal(data2, &rec2)

	if rec1.Integrity.SHA256 == rec2.Integrity.SHA256 {
		t.Error("different input should produce different hash")
	}
}
