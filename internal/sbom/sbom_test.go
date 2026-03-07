package sbom

import (
	"bytes"
	"encoding/json"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"

	"github.com/blacksd/crd-schemas/internal/source"
)

func decode(t *testing.T, data []byte) *cdx.BOM {
	t.Helper()
	bom := cdx.NewBOM()
	decoder := cdx.NewBOMDecoder(bytes.NewReader(data), cdx.BOMFileFormatJSON)
	if err := decoder.Decode(bom); err != nil {
		t.Fatalf("decode SBOM: %v", err)
	}
	return bom
}

func TestGenerate(t *testing.T) {
	sources := []source.Source{
		{
			Name:     "test-operator",
			Version:  "v1.0.0",
			License:  "Apache-2.0",
			Homepage: "https://example.com",
		},
		{
			Name:    "other-thing",
			Version: "v2.0.0",
		},
	}

	data, err := Generate(sources, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	bom := decode(t, data)

	if bom.BOMFormat != "CycloneDX" {
		t.Errorf("bomFormat = %q, want %q", bom.BOMFormat, "CycloneDX")
	}
	if bom.SpecVersion != cdx.SpecVersion1_5 {
		t.Errorf("specVersion = %q, want %q", bom.SpecVersion, cdx.SpecVersion1_5)
	}
	if bom.Metadata == nil || bom.Metadata.Timestamp != "2026-01-01T00:00:00Z" {
		t.Errorf("metadata.timestamp = %q, want %q", bom.Metadata.Timestamp, "2026-01-01T00:00:00Z")
	}

	// Tool is declared as a component in spec 1.5+
	if bom.Metadata.Tools == nil || bom.Metadata.Tools.Components == nil || len(*bom.Metadata.Tools.Components) != 1 {
		t.Fatal("expected 1 tool component in metadata")
	}
	toolComp := (*bom.Metadata.Tools.Components)[0]
	if toolComp.Name != "crd-schemas" {
		t.Errorf("tool name = %q, want %q", toolComp.Name, "crd-schemas")
	}

	if bom.Components == nil || len(*bom.Components) != 2 {
		t.Fatalf("expected 2 components, got %v", bom.Components)
	}
	components := *bom.Components

	// Components are sorted by name: "other-thing" before "test-operator"

	// First component (alphabetically): no license or homepage
	c0 := components[0]
	if c0.Name != "other-thing" {
		t.Errorf("components[0].name = %q, want %q", c0.Name, "other-thing")
	}
	if c0.Version != "v2.0.0" {
		t.Errorf("components[0].version = %q, want %q", c0.Version, "v2.0.0")
	}
	if c0.Type != cdx.ComponentTypeLibrary {
		t.Errorf("components[0].type = %q, want %q", c0.Type, cdx.ComponentTypeLibrary)
	}
	if c0.Licenses != nil && len(*c0.Licenses) != 0 {
		t.Errorf("expected no licenses for component without license, got %d", len(*c0.Licenses))
	}
	if c0.ExternalReferences != nil && len(*c0.ExternalReferences) != 0 {
		t.Errorf("expected no external refs for component without homepage, got %d", len(*c0.ExternalReferences))
	}

	// Second component (alphabetically): has license and homepage
	c1 := components[1]
	if c1.Name != "test-operator" {
		t.Errorf("components[1].name = %q, want %q", c1.Name, "test-operator")
	}
	if c1.Version != "v1.0.0" {
		t.Errorf("components[1].version = %q, want %q", c1.Version, "v1.0.0")
	}
	if c1.Licenses == nil || len(*c1.Licenses) != 1 || (*c1.Licenses)[0].License.ID != "Apache-2.0" {
		t.Errorf("components[1].licenses unexpected: %+v", c1.Licenses)
	}
	if c1.ExternalReferences == nil || len(*c1.ExternalReferences) != 1 || (*c1.ExternalReferences)[0].URL != "https://example.com" {
		t.Errorf("components[1].externalReferences unexpected: %+v", c1.ExternalReferences)
	}
}

func TestGenerateEmpty(t *testing.T) {
	data, err := Generate(nil, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Generate with nil sources: %v", err)
	}

	bom := decode(t, data)

	if bom.Components != nil && len(*bom.Components) != 0 {
		t.Errorf("expected nil/empty components for empty input, got %d", len(*bom.Components))
	}
}

func TestGenerateValidJSON(t *testing.T) {
	sources := []source.Source{{Name: "test", Version: "v1.0.0"}}
	data, err := Generate(sources, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !json.Valid(data) {
		t.Error("output is not valid JSON")
	}
}
