# crd-schemas

Automated Kubernetes CRD JSON Schema catalog. Extracts `openAPIV3Schema` definitions from upstream operator Helm charts and release manifests, producing standalone JSON Schema files suitable for client-side validation (e.g., `kubectl`, `kubeconform`, IDE plugins).

The `main` branch always contains the latest schemas. Historical versions are accessible via git tags (`{source-name}/{version}`). Renovate keeps source versions current via automated PRs.

## Schema directory layout

```
schemas/
  {api-group}/{apiversion}/{kind}.json
  {api-group}/{apiversion}/{kind}.provenance.json
  sbom.cdx.json                       # CycloneDX 1.5 SBOM
```

For example, cert-manager v1.17.2 produces:

```
schemas/
  cert-manager.io/v1/certificate.json
  cert-manager.io/v1/certificate.provenance.json
  acme.cert-manager.io/v1/challenge.json
  acme.cert-manager.io/v1/order.json
```

## Using the schemas

### With kubeconform

Point kubeconform at the schema tree for direct lookup:

```bash
kubeconform \
  -schema-location default \
  -schema-location 'schemas/{{.Group}}/{{.ResourceAPIVersion}}/{{.ResourceKind}}.json' \
  my-manifests/
```

Or use the GitHub raw URL for remote consumption:

```bash
kubeconform \
  -schema-location default \
  -schema-location 'https://raw.githubusercontent.com/blacksd/crd-schemas/main/schemas/{{.Group}}/{{.ResourceAPIVersion}}/{{.ResourceKind}}.json' \
  my-manifests/
```

To pin schemas to a specific upstream source version, replace `main` with a tag:

```bash
kubeconform \
  -schema-location default \
  -schema-location 'https://raw.githubusercontent.com/blacksd/crd-schemas/cert-manager/v1.17.2/schemas/{{.Group}}/{{.ResourceAPIVersion}}/{{.ResourceKind}}.json' \
  my-manifests/
```

### With kubectl (client-side validation)

Use the schemas as an OpenAPI overlay for offline validation tooling, or feed them into custom admission scripts.

### In IDEs

Editors with YAML/JSON Schema support (VS Code with the YAML extension, JetBrains IDEs) can reference individual schema files for autocompletion and validation of custom resource manifests:

```jsonc
// .vscode/settings.json
{
  "yaml.schemas": {
    "schemas/cert-manager.io/v1/certificate.json": "manifests/**/certificate*.yaml"
  }
}
```

## Version history

Historical schemas are accessible via git tags. Each source version bump creates an annotated tag:

```bash
# List all versions of a source
git tag -l 'cert-manager/*'

# View a specific historical schema
git show cert-manager/v1.17.2:schemas/cert-manager.io/v1/certificate.json
```

## Adding a new source

Create a YAML file in `sources/` named after the API group (e.g., `sources/example.io.yaml`):

```yaml
sources:
  - name: example-operator         # unique identifier
    type: helm                      # "helm" or "url"
    repo: https://charts.example.io # Helm repository URL
    chart: example-operator         # Helm chart name
    version: v1.2.3                 # upstream version to extract
    license: Apache-2.0             # SPDX license identifier
    homepage: https://example.io    # project homepage
```

### Helm source with custom values

Some charts gate CRD installation behind Helm values. Pass them via the `values` map:

```yaml
sources:
  - name: cert-manager
    type: helm
    repo: https://charts.jetstack.io
    chart: cert-manager
    version: v1.17.2
    license: Apache-2.0
    homepage: https://cert-manager.io
    values:
      crds.enabled: "true"
```

### URL source (direct manifest)

For projects that publish CRD manifests as release assets rather than Helm charts:

```yaml
sources:
  - name: gateway-api
    type: url
    url: https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/experimental-install.yaml
    version: v1.2.1
    license: Apache-2.0
    homepage: https://gateway-api.sigs.k8s.io
```

### Filtering extracted CRDs

When a source produces CRDs you don't need, or when multiple sources overlap on the same API group, use `include` (allowlist) or `exclude` (denylist) filters. If both are set, `include` takes precedence.

```yaml
sources:
  - name: example-operator
    type: helm
    repo: https://charts.example.io
    chart: example-operator
    version: v1.0.0
    license: Apache-2.0
    homepage: https://example.io
    include:
      - Widget                          # match by Kind (case-insensitive)
      - example.io/Gadget               # match by group/Kind
    exclude:
      - example.io/*                    # wildcard: all Kinds in a group
```

## Running locally

### Prerequisites

- Go 1.25+
- Helm 3

Or use the Nix flake (provides both via `direnv`):

```bash
direnv allow   # or: nix develop
```

### Extract all schemas

```bash
go run ./cmd/extract/
```

### Extract a single source

```bash
go run ./cmd/extract/ -source cert-manager
```

### CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `-sources` | `sources` | Directory containing source YAML config files |
| `-output` | `schemas` | Output directory for extracted schemas |
| `-source` | (all) | Process only the named source |
| `-debug` | `false` | Enable debug logging (helm commands, HTTP fetches, per-schema detail) |

### Validate source configs

```bash
check-jsonschema --schemafile source.schema.json sources/*.yaml
```

### Run tests

```bash
go test ./...
```

## How it works

1. Load source configs from `sources/` (one file per API group)
2. For each source, fetch CRDs:
   - **Helm**: `helm pull` + extract from `crds/` directory, then `helm template --include-crds` for template-generated CRDs
   - **URL**: HTTP GET the manifest
3. Parse multi-document YAML, keep only `CustomResourceDefinition` resources with `served: true` versions that contain an `openAPIV3Schema`
4. Apply include/exclude filters from the source config
5. Detect conflicts (same group/kind/version from different sources with different content)
6. Deduplicate identical schemas across sources
7. Write schema and provenance files to `schemas/{group}/{apiversion}/`
8. Generate a CycloneDX 1.5 SBOM listing all sources as components

## Automation

### CI pipeline

Two workflows run in CI:

- **`update-schemas.yml`**: Runs extraction on every push to `main` that touches `sources/`, `cmd/`, or `internal/`. On PRs, schemas are uploaded as an artifact for review. After extraction, annotated git tags are created for each source whose version changed.
- **`validate.yml`**: Validates source configs against `source.schema.json` and checks extracted schemas for well-formed JSON and required structure. Runs on PRs and pushes to `main`.

### Version updates

Renovate is configured (`renovate.json`) to detect Helm chart versions and GitHub release URLs in source configs and open PRs weekly. When a version-bump PR merges, CI re-extracts schemas and tags the result.
