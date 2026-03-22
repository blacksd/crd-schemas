# crd-schemas

Kubernetes CRD JSON Schemas extracted from upstream Helm charts and release manifests. Use them with [kubeconform](https://github.com/yannh/kubeconform) to validate custom resources.

## Usage

Validate manifests against built-in schemas plus CRD schemas from this repo:

```bash
kubeconform \
  -schema-location default \
  -schema-location 'https://raw.githubusercontent.com/blacksd/crd-schemas/main/{{.Group}}/{{.ResourceAPIVersion}}/{{.ResourceKind}}.json' \
  my-manifests/
```

Pin to a specific source version by replacing `main` with a git tag (tags follow the `source-name/version` convention, e.g. `cert-manager/v1.17.2`):

```bash
kubeconform \
  -schema-location default \
  -schema-location 'https://raw.githubusercontent.com/blacksd/crd-schemas/cert-manager/v1.17.2/{{.Group}}/{{.ResourceAPIVersion}}/{{.ResourceKind}}.json' \
  my-manifests/
```

> **Note:** Tags created before the flat layout restructuring still use a `schemas/` prefix. For those older tags, insert `schemas/` before `{{.Group}}` in the URL.

For local/offline use, clone the repo and point kubeconform at it directly:

```bash
kubeconform \
  -schema-location default \
  -schema-location '/path/to/crd-schemas/{{.Group}}/{{.ResourceAPIVersion}}/{{.ResourceKind}}.json' \
  my-manifests/
```

## Schema layout

```
{api-group}/{apiVersion}/{kind}.json
```

For example, cert-manager produces:

```
cert-manager.io/v1/certificate.json
acme.cert-manager.io/v1/challenge.json
acme.cert-manager.io/v1/order.json
```

Each schema has a companion `.provenance.json` file with per-schema extraction metadata and integrity hashes. Each API group directory also contains a CycloneDX 1.5 SBOM (`sbom.cdx.json`) listing the upstream sources that produced its schemas.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to add or update CRD sources.
