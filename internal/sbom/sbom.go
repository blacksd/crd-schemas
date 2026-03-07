// Package sbom generates a CycloneDX SBOM from the extracted sources.
package sbom

import (
	"bytes"
	"sort"

	cdx "github.com/CycloneDX/cyclonedx-go"

	"github.com/blacksd/crd-schemas/internal/provenance"
	"github.com/blacksd/crd-schemas/internal/source"
)

// Generate creates a CycloneDX 1.5 SBOM from a list of sources.
func Generate(sources []source.Source, timestamp string) ([]byte, error) {
	bom := cdx.NewBOM()

	bom.Metadata = &cdx.Metadata{
		Timestamp: timestamp,
		Tools: &cdx.ToolsChoice{
			Components: &[]cdx.Component{
				{
					Type:    cdx.ComponentTypeApplication,
					Name:    "crd-schemas",
					Version: provenance.ToolVersion,
				},
			},
		},
	}

	var components []cdx.Component
	for _, src := range sources {
		c := cdx.Component{
			Type:    cdx.ComponentTypeLibrary,
			Name:    src.Name,
			Version: src.Version,
		}
		if src.License != "" {
			c.Licenses = &cdx.Licenses{
				cdx.LicenseChoice{
					License: &cdx.License{ID: src.License},
				},
			}
		}
		if src.Homepage != "" {
			c.ExternalReferences = &[]cdx.ExternalReference{
				{Type: cdx.ERTypeWebsite, URL: src.Homepage},
			}
		}
		components = append(components, c)
	}

	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	if len(components) > 0 {
		bom.Components = &components
	}

	var buf bytes.Buffer
	encoder := cdx.NewBOMEncoder(&buf, cdx.BOMFileFormatJSON)
	encoder.SetPretty(true)
	if err := encoder.EncodeVersion(bom, cdx.SpecVersion1_5); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
