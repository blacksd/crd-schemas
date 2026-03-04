// Package provenance generates per-schema provenance metadata files.
package provenance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/blacksd/crd-schemas/internal/source"
)

const ToolVersion = "crd-schemas v0.1.0"

// Record represents the provenance metadata for a single extracted schema.
type Record struct {
	Schema     string     `json:"schema"`
	CRD        CRDInfo    `json:"crd"`
	Source     source.Source `json:"source"`
	Extraction Extraction `json:"extraction"`
	Integrity  Integrity  `json:"integrity"`
}

type CRDInfo struct {
	Group      string `json:"group"`
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

type Extraction struct {
	Timestamp   string `json:"timestamp"`
	ToolVersion string `json:"toolVersion"`
}

type Integrity struct {
	SHA256 string `json:"sha256"`
}

// Generate creates a provenance record for a schema.
func Generate(schemaFilename string, group, kind, apiVersion, timestamp string, src source.Source, schemaData []byte) ([]byte, error) {
	hash := sha256.Sum256(schemaData)

	rec := Record{
		Schema: schemaFilename,
		CRD: CRDInfo{
			Group:      group,
			Kind:       kind,
			APIVersion: apiVersion,
		},
		Source: src,
		Extraction: Extraction{
			Timestamp:   timestamp,
			ToolVersion: ToolVersion,
		},
		Integrity: Integrity{
			SHA256: fmt.Sprintf("%x", hash),
		},
	}

	return json.MarshalIndent(rec, "", "  ")
}
