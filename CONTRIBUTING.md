# Contributing

## Adding a new CRD source

Each source config file in `sources/` is named after the primary API group it provides (e.g., `cert-manager.io.yaml`). Source configs are validated automatically against `source.schema.json` on every PR.

Source files are validated against [`source.schema.json`](source.schema.json). IDEs with YAML schema support can use it for autocompletion:

```jsonc
// .vscode/settings.json
{
  "yaml.schemas": {
    "source.schema.json": "sources/*.yaml"
  }
}
```

### 1. Create the source file

Create `sources/{api-group}.yaml`:

```yaml
sources:
  - name: my-operator          # unique, lowercase, alphanumeric with hyphens
    type: helm                  # "helm" or "url"
    repo: https://charts.example.io
    chart: my-operator
    version: v1.2.3
    license: Apache-2.0        # SPDX identifier
    homepage: https://example.io
```

For URL-based sources (direct manifest download):

```yaml
sources:
  - name: my-project
    type: url
    url: https://github.com/org/project/releases/download/v1.2.3/crds.yaml
    version: v1.2.3
    license: Apache-2.0
    homepage: https://example.io
```

### 2. Required fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique identifier. Lowercase, alphanumeric, hyphens only. |
| `type` | string | `helm` or `url`. |
| `repo` | string | Helm repository URL. Required for `helm` type. |
| `chart` | string | Helm chart name. Required for `helm` type. |
| `url` | string | Manifest URL. Required for `url` type. |
| `version` | string | Upstream version to extract. Must start with a digit or `v`. |
| `license` | string | SPDX license identifier for the upstream project. |
| `homepage` | string | Project homepage URL. |

### 3. Optional fields

| Field | Type | Description |
|-------|------|-------------|
| `values` | map | Helm `--set` key-value pairs for charts that gate CRDs behind values. |
| `include` | list | Allowlist of kinds to keep. Supports `Kind`, `group/Kind`, `group/*`. |
| `exclude` | list | Denylist of kinds to drop. Same pattern syntax as `include`. |

If both `include` and `exclude` are set, `include` takes precedence.

### 4. Test locally

```bash
# Extract only your new source
go run ./cmd/extract/ -source my-operator -debug

# Verify the output
ls schemas/{api-group}/
```

### 5. Open a PR

Push your branch and open a pull request. CI will:

- Validate your source config against `source.schema.json`
- Run extraction and upload the resulting schemas as a PR artifact
- Validate all extracted JSON schemas for well-formedness

## Updating an existing source version

Renovate handles this automatically via weekly PRs. To bump manually, edit the `version` field (and `url` for URL sources) in the relevant source config file.

## Conventions

- One source config file per primary API group
- File named after the API group: `{api-group}.yaml`
- Source names are lowercase with hyphens, matching the upstream project name
- Versions include the `v` prefix where the upstream project uses one
- When a Helm chart requires values to install CRDs, document them in the `values` map
- Use `include`/`exclude` filters when a source produces CRDs outside its primary API group that overlap with another source
