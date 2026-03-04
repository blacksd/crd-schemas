// Command extract reads CRD source configs, fetches upstream CRDs, extracts
// JSON schemas, and writes them to the output directory with provenance and SBOM.
package main

import (
	"crypto/sha256"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/blacksd/crd-schemas/internal/extractor"
	"github.com/blacksd/crd-schemas/internal/provenance"
	"github.com/blacksd/crd-schemas/internal/sbom"
	"github.com/blacksd/crd-schemas/internal/source"
)

func main() {
	sourcesDir := flag.String("sources", "sources", "Directory containing source YAML config files")
	outputDir := flag.String("output", "schemas", "Output directory for extracted schemas")
	filterSource := flag.String("source", "", "Process only the named source (default: all)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Timestamp().Logger()

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if err := run(log, *sourcesDir, *outputDir, *filterSource); err != nil {
		log.Fatal().Err(err).Msg("extraction failed")
	}
}

// schemaEntry tracks an extracted schema alongside its source metadata.
type schemaEntry struct {
	schema extractor.CRDSchema
	src    source.Source
}

func run(log zerolog.Logger, sourcesDir, outputDir, filterSource string) error {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	grouped, err := source.LoadAll(sourcesDir)
	if err != nil {
		return err
	}

	log.Debug().Str("dir", sourcesDir).Msg("loaded source configs")

	// Phase 1: Extract and filter all schemas
	var allEntries []schemaEntry
	var allSchemas []extractor.CRDSchema
	totalSources := 0

	for _, sources := range grouped {
		for _, src := range sources {
			if filterSource != "" && src.Name != filterSource {
				continue
			}

			totalSources++
			srcLog := log.With().Str("source", src.Name).Str("type", src.Type).Str("version", src.Version).Logger()
			srcLog.Info().Msg("processing")

			schemas, err := extractor.Extract(srcLog, src)
			if err != nil {
				srcLog.Warn().Err(err).Msg("extraction failed")
				continue
			}

			// Tag schemas with their source name
			for i := range schemas {
				schemas[i].SourceName = src.Name
			}

			// Apply include/exclude filters
			schemas = extractor.Filter(schemas, src)

			srcLog.Info().Int("count", len(schemas)).Msg("schemas extracted")

			for _, s := range schemas {
				allEntries = append(allEntries, schemaEntry{schema: s, src: src})
				allSchemas = append(allSchemas, s)
			}
		}
	}

	// Phase 2: Detect conflicts
	conflicts := extractor.DetectConflicts(log, allSchemas)
	if len(conflicts) > 0 {
		for _, c := range conflicts {
			log.Error().
				Str("group", c.Group).
				Str("kind", c.Kind).
				Str("apiVersion", c.APIVersion).
				Strs("sources", c.Sources).
				Msg("schema conflict")
		}
		log.Fatal().Int("count", len(conflicts)).Msg("resolve conflicts with include/exclude filters in source configs")
	}

	// Phase 3: Deduplicate identical schemas (keep first occurrence)
	seen := make(map[string]bool)
	var dedupedEntries []schemaEntry
	for _, e := range allEntries {
		key := extractor.SchemaKey(e.schema)
		if seen[key] {
			continue
		}
		seen[key] = true
		dedupedEntries = append(dedupedEntries, e)
	}

	// Phase 4: Write schemas to disk
	written, skipped := 0, 0
	for _, e := range dedupedEntries {
		schema := e.schema
		src := e.src

		kindLower := strings.ToLower(schema.Kind)
		schemaFilename := kindLower + ".json"
		provFilename := kindLower + ".provenance.json"
		group := schema.Group

		// Write schemas to: <group>/<apiVersion>/<kind>.json
		schemaDir := filepath.Join(outputDir, group, schema.APIVersion)
		schemaPath := filepath.Join(schemaDir, schemaFilename)
		changed := !fileContentEqual(schemaPath, schema.Schema)

		if changed {
			provJSON, err := provenance.Generate(
				schemaFilename, group, schema.Kind, schema.APIVersion,
				timestamp, src, schema.Schema,
			)
			if err != nil {
				log.Warn().Err(err).Str("group", group).Str("kind", schema.Kind).Msg("generating provenance")
				continue
			}

			if err := os.MkdirAll(schemaDir, 0755); err != nil {
				return err
			}
			if err := os.WriteFile(schemaPath, schema.Schema, 0644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(schemaDir, provFilename), provJSON, 0644); err != nil {
				return err
			}

			log.Debug().Str("group", group).Str("kind", schema.Kind).Str("apiVersion", schema.APIVersion).Msg("written schema")
			written++
		} else {
			log.Debug().Str("group", group).Str("kind", schema.Kind).Str("apiVersion", schema.APIVersion).Msg("unchanged, skipping")
			skipped++
		}
	}

	// Generate SBOM
	allSources := source.All(grouped)
	sbomJSON, err := sbom.Generate(allSources, timestamp)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	sbomPath := filepath.Join(outputDir, "sbom.cdx.json")
	if err := os.WriteFile(sbomPath, sbomJSON, 0644); err != nil {
		return err
	}

	log.Info().
		Int("sources", totalSources).
		Int("schemas", len(dedupedEntries)).
		Int("written", written).
		Int("unchanged", skipped).
		Msg("extraction complete")
	return nil
}

// fileContentEqual returns true if the file at path exists and its content
// has the same SHA-256 hash as data.
func fileContentEqual(path string, data []byte) bool {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return sha256.Sum256(existing) == sha256.Sum256(data)
}
