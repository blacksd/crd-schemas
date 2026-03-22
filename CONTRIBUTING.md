# Contributing

## Acceptance criteria

Sources must reference **publicly accessible** CRD definitions. This means:

- Helm chart repositories must be unauthenticated (no private registries, no OCI images behind login).
- URL sources must resolve without credentials (public GitHub releases, raw URLs, etc.).
- The upstream project must distribute its CRDs as a published artifact (release asset, Helm chart, or in-tree manifest at a tagged ref).
- The upstream license must permit redistribution of derived artifacts (the extracted JSON schemas). Most OSI-approved licenses qualify (Apache-2.0, MIT, MPL-2.0, AGPL-3.0, etc.). Licenses that restrict redistribution or derived works, such as CC BY-NC or proprietary/source-available licenses, do not. The upstream license is recorded in each schema's provenance metadata for attribution.

We do not accept sources that require authentication, VPN access, or any form of private distribution. The extracted schemas are served publicly and must be reproducible by anyone.

## Adding a new CRD source

Each source config file in `.sources/` is named after the primary API group it provides (e.g., `cert-manager.io.yaml`). Source configs are validated automatically against the [upstream schema](https://raw.githubusercontent.com/blacksd/crd-schema-extractor/v1.0.0/source.schema.json) on every PR.

IDEs with YAML schema support can use the upstream schema URL for autocompletion:

```jsonc
// .vscode/settings.json
{
  "yaml.schemas": {
    "https://raw.githubusercontent.com/blacksd/crd-schema-extractor/v1.0.0/source.schema.json": ".sources/*.yaml"
  }
}
```

### 1. Create the source file

Create `.sources/{api-group}.yaml`:

```yaml
sources:
  - name: my-operator          # unique, lowercase, alphanumeric with hyphens
    type: helm                  # "helm", "url", or "git+github"
    repo: https://charts.example.io
    chart: my-operator
    version: v1.2.3
    license: Apache-2.0        # SPDX identifier
    homepage: https://example.io
```

For Helm charts published to OCI registries, use an `oci://` repo URL:

```yaml
sources:
  - name: my-operator
    type: helm
    repo: oci://ghcr.io/my-org/charts
    chart: my-operator
    version: v1.2.3
    license: Apache-2.0
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

For GitHub repositories that commit CRD YAML files directly (not via Helm or release assets):

```yaml
sources:
  - name: my-project
    type: git+github
    repo: https://github.com/org/project
    version: v1.2.3
    license: Apache-2.0
    homepage: https://example.io
    path: path/to/crds          # optional: restrict scan to a subdirectory
```

### 2. Required fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique identifier. Lowercase, alphanumeric, hyphens only. |
| `type` | string | `helm`, `url`, or `git+github`. |
| `repo` | string | Helm repository URL or GitHub repository URL. Required for `helm` and `git+github` types. |
| `chart` | string | Helm chart name. Required for `helm` type. |
| `url` | string | Manifest URL. Required for `url` type. |
| `version` | string | Upstream version to extract. Must start with a digit or `v`. |
| `license` | string | SPDX license identifier for the upstream project. |
| `homepage` | string | Project homepage URL. |

### 3. Optional fields

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Subdirectory to scan for CRD files. Useful for `git+github` sources in large repositories. |
| `values` | map | Helm `--set` key-value pairs for charts that gate CRDs behind values. |
| `include` | list | Allowlist of kinds to keep. Supports `Kind`, `group/Kind`, `group/*`. |
| `exclude` | list | Denylist of kinds to drop. Same pattern syntax as `include`. |

If both `include` and `exclude` are set, `include` takes precedence.

### 4. Test locally

```bash
# Extract only your new source
crd-schema-extractor extract .sources --fetch-only my-operator --debug

# Verify the output
ls {api-group}/
```

### 5. Open a PR

Push your branch and open a pull request. CI will:

- Validate your source config against the upstream source schema

## Known gaps

The following popular projects are not currently covered due to technical limitations:

### No public CRD artifact

- **Karpenter** (`karpenter.sh`) -- The core `kubernetes-sigs/karpenter` repo is a framework library. CRDs are shipped by provider-specific implementations (e.g., `aws/karpenter-provider-aws`), which use OCI registries that require authentication.

### Licensing or automation issues

- **EMQX Operator** (`apps.emqx.io`) -- No license declared in the repository.
- **MySQL Operator for Kubernetes** (`mysql.oracle.com`) -- Uses Oracle's proprietary licensing, not an SPDX-compatible open-source license.
- **Apache Flink Operator** (`flink.apache.org`) -- The Helm repository URL is release-version-specific (changes with every release), which breaks Renovate-based version automation.

Contributions that resolve any of these limitations are welcome.

## Updating an existing source version

Renovate handles this automatically via weekly PRs. To bump manually, edit the `version` field (and `url` for URL sources) in the relevant source config file.

## Conventions

- One source config file per primary API group
- File named after the API group: `{api-group}.yaml`
- Source names are lowercase with hyphens, matching the upstream project name
- Versions include the `v` prefix where the upstream project uses one
- When a Helm chart requires values to install CRDs, document them in the `values` map
- Use `include`/`exclude` filters when a source produces CRDs outside its primary API group that overlap with another source
