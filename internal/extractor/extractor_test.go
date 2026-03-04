package extractor

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/rs/zerolog"

	"github.com/blacksd/crd-schemas/internal/source"
)

// nopLog returns a disabled logger for tests that don't care about log output.
var nopLog = zerolog.New(io.Discard)

// Minimal CRD YAML for testing the parser.
const testCRDYAML = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.example.com
spec:
  group: example.com
  names:
    kind: Foo
    plural: foos
    singular: foo
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                replicas:
                  type: integer
    - name: v1beta1
      served: true
      storage: false
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
    - name: v1alpha1
      served: false
      storage: false
      schema:
        openAPIV3Schema:
          type: object
`

func TestParseCRDs(t *testing.T) {
	schemas, err := parseCRDs(nopLog, []byte(testCRDYAML))
	if err != nil {
		t.Fatalf("parseCRDs: %v", err)
	}

	// Should extract v1 and v1beta1 (both served), skip v1alpha1 (not served)
	if len(schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(schemas))
	}

	byVersion := map[string]CRDSchema{}
	for _, s := range schemas {
		byVersion[s.APIVersion] = s
	}

	v1, ok := byVersion["v1"]
	if !ok {
		t.Fatal("expected v1 schema not found")
	}
	if v1.Group != "example.com" {
		t.Errorf("v1 group = %q, want %q", v1.Group, "example.com")
	}
	if v1.Kind != "Foo" {
		t.Errorf("v1 kind = %q, want %q", v1.Kind, "Foo")
	}

	// Verify the schema is valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(v1.Schema, &parsed); err != nil {
		t.Errorf("v1 schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("v1 schema type = %v, want %q", parsed["type"], "object")
	}

	if _, ok := byVersion["v1beta1"]; !ok {
		t.Error("expected v1beta1 schema not found")
	}

	if _, ok := byVersion["v1alpha1"]; ok {
		t.Error("v1alpha1 should have been skipped (served=false)")
	}
}

const testMultiDocYAML = `apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  ports:
    - port: 80
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: bars.example.com
spec:
  group: example.com
  names:
    kind: Bar
    plural: bars
    singular: bar
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
`

func TestParseCRDsMultiDoc(t *testing.T) {
	schemas, err := parseCRDs(nopLog, []byte(testMultiDocYAML))
	if err != nil {
		t.Fatalf("parseCRDs: %v", err)
	}

	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema (only the CRD), got %d", len(schemas))
	}

	if schemas[0].Kind != "Bar" {
		t.Errorf("kind = %q, want %q", schemas[0].Kind, "Bar")
	}
}

func TestParseCRDsEmpty(t *testing.T) {
	schemas, err := parseCRDs(nopLog, []byte(""))
	if err != nil {
		t.Fatalf("parseCRDs on empty input: %v", err)
	}
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas, got %d", len(schemas))
	}
}

func TestParseCRDsNoCRDs(t *testing.T) {
	yaml := `apiVersion: v1
kind: Service
metadata:
  name: svc
spec:
  ports:
    - port: 80
`
	schemas, err := parseCRDs(nopLog, []byte(yaml))
	if err != nil {
		t.Fatalf("parseCRDs: %v", err)
	}
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas from non-CRD input, got %d", len(schemas))
	}
}

func TestParseCRDsNoSchema(t *testing.T) {
	yaml := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: noschemas.example.com
spec:
  group: example.com
  names:
    kind: NoSchema
    plural: noschemas
    singular: noschema
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
`
	schemas, err := parseCRDs(nopLog, []byte(yaml))
	if err != nil {
		t.Fatalf("parseCRDs: %v", err)
	}
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas (CRD without openAPIV3Schema), got %d", len(schemas))
	}
}

func TestParseCRDsMalformedYAML(t *testing.T) {
	schemas, err := parseCRDs(nopLog, []byte(`{{not valid yaml at all`))
	if err != nil {
		t.Fatalf("parseCRDs should not return error on malformed input, got: %v", err)
	}
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas from malformed YAML, got %d", len(schemas))
	}
}

func TestParseCRDsMalformedCRDAmongValid(t *testing.T) {
	// A valid CRD followed by a document that claims to be a CRD but has
	// broken spec structure -- the parser should extract the valid one and
	// skip the broken one.
	yaml := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: goods.example.com
spec:
  group: example.com
  names:
    kind: Good
    plural: goods
    singular: good
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: bads.example.com
spec:
  group: example.com
  names:
    kind: Bad
    plural: bads
    singular: bad
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: INVALID_TYPE_THAT_SHOULD_NOT_CRASH
`
	schemas, err := parseCRDs(nopLog, []byte(yaml))
	if err != nil {
		t.Fatalf("parseCRDs should not return error, got: %v", err)
	}

	// Both should still parse -- the schema content is passed through as-is,
	// validation is kubeconform's job, not ours.
	if len(schemas) != 2 {
		t.Fatalf("expected 2 schemas (we extract, not validate), got %d", len(schemas))
	}
}

func TestParseCRDsTruncatedDocument(t *testing.T) {
	// A CRD that is cut off mid-document
	yaml := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: truncated.example.com
spec:
  group: example.com
  names:
    kind: Truncated
    plural: truncated
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Sc`

	schemas, err := parseCRDs(nopLog, []byte(yaml))
	if err != nil {
		t.Fatalf("parseCRDs should not return error on truncated input, got: %v", err)
	}
	// Truncated doc won't have a valid schema, so 0 schemas
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas from truncated CRD, got %d", len(schemas))
	}
}

func TestParseCRDsMissingGroup(t *testing.T) {
	yaml := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nogroup.example.com
spec:
  names:
    kind: NoGroup
    plural: nogroups
    singular: nogroup
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`
	schemas, err := parseCRDs(nopLog, []byte(yaml))
	if err != nil {
		t.Fatalf("parseCRDs: %v", err)
	}
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas from CRD with no group, got %d", len(schemas))
	}
}

func TestDedup(t *testing.T) {
	schemas := []CRDSchema{
		{Group: "example.com", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{}`)},
		{Group: "example.com", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{}`)},
		{Group: "example.com", Kind: "Bar", APIVersion: "v1", Schema: []byte(`{}`)},
	}

	result := dedup(schemas)
	if len(result) != 2 {
		t.Fatalf("expected 2 deduplicated schemas, got %d", len(result))
	}
}

// --- Filter tests ---

func testSchemas() []CRDSchema {
	return []CRDSchema{
		{Group: "gateway.networking.k8s.io", Kind: "GatewayClass", APIVersion: "v1"},
		{Group: "gateway.networking.k8s.io", Kind: "Gateway", APIVersion: "v1"},
		{Group: "gateway.networking.k8s.io", Kind: "HTTPRoute", APIVersion: "v1"},
		{Group: "gateway.networking.k8s.io", Kind: "TCPRoute", APIVersion: "v1alpha2"},
		{Group: "gateway.networking.k8s.io", Kind: "TLSRoute", APIVersion: "v1alpha2"},
		{Group: "example.com", Kind: "Foo", APIVersion: "v1"},
	}
}

func schemaKinds(schemas []CRDSchema) []string {
	var kinds []string
	for _, s := range schemas {
		kinds = append(kinds, s.Kind)
	}
	return kinds
}

func containsKind(schemas []CRDSchema, kind string) bool {
	for _, s := range schemas {
		if s.Kind == kind {
			return true
		}
	}
	return false
}

func TestFilterNoRules(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{Name: "test"}

	result := Filter(schemas, src)
	if len(result) != len(schemas) {
		t.Errorf("no rules: expected %d schemas, got %d", len(schemas), len(result))
	}
}

func TestFilterIncludeByKind(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Include: []string{"TCPRoute", "TLSRoute"},
	}

	result := Filter(schemas, src)
	if len(result) != 2 {
		t.Fatalf("expected 2 schemas, got %d: %v", len(result), schemaKinds(result))
	}
	if !containsKind(result, "TCPRoute") {
		t.Error("expected TCPRoute in result")
	}
	if !containsKind(result, "TLSRoute") {
		t.Error("expected TLSRoute in result")
	}
}

func TestFilterIncludeByGroupKind(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Include: []string{"gateway.networking.k8s.io/GatewayClass", "example.com/Foo"},
	}

	result := Filter(schemas, src)
	if len(result) != 2 {
		t.Fatalf("expected 2 schemas, got %d: %v", len(result), schemaKinds(result))
	}
	if !containsKind(result, "GatewayClass") {
		t.Error("expected GatewayClass in result")
	}
	if !containsKind(result, "Foo") {
		t.Error("expected Foo in result")
	}
}

func TestFilterIncludeByGroupWildcard(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Include: []string{"gateway.networking.k8s.io/*"},
	}

	result := Filter(schemas, src)
	if len(result) != 5 {
		t.Fatalf("expected 5 gateway schemas, got %d: %v", len(result), schemaKinds(result))
	}
	if containsKind(result, "Foo") {
		t.Error("Foo from example.com should not be included")
	}
}

func TestFilterIncludeCaseInsensitive(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Include: []string{"gatewayclass", "tcproute"},
	}

	result := Filter(schemas, src)
	if len(result) != 2 {
		t.Fatalf("expected 2 schemas (case-insensitive), got %d: %v", len(result), schemaKinds(result))
	}
}

func TestFilterExcludeByKind(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Exclude: []string{"TCPRoute", "TLSRoute"},
	}

	result := Filter(schemas, src)
	if len(result) != 4 {
		t.Fatalf("expected 4 schemas, got %d: %v", len(result), schemaKinds(result))
	}
	if containsKind(result, "TCPRoute") {
		t.Error("TCPRoute should have been excluded")
	}
	if containsKind(result, "TLSRoute") {
		t.Error("TLSRoute should have been excluded")
	}
}

func TestFilterExcludeByGroupWildcard(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Exclude: []string{"gateway.networking.k8s.io/*"},
	}

	result := Filter(schemas, src)
	if len(result) != 1 {
		t.Fatalf("expected 1 schema (only Foo), got %d: %v", len(result), schemaKinds(result))
	}
	if result[0].Kind != "Foo" {
		t.Errorf("expected Foo, got %s", result[0].Kind)
	}
}

func TestFilterExcludeByGroupKind(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Exclude: []string{"gateway.networking.k8s.io/GatewayClass"},
	}

	result := Filter(schemas, src)
	if len(result) != 5 {
		t.Fatalf("expected 5 schemas, got %d: %v", len(result), schemaKinds(result))
	}
	if containsKind(result, "GatewayClass") {
		t.Error("GatewayClass should have been excluded")
	}
}

func TestFilterIncludeTakesPrecedenceOverExclude(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Include: []string{"Foo"},
		Exclude: []string{"Foo"}, // should be ignored
	}

	result := Filter(schemas, src)
	if len(result) != 1 {
		t.Fatalf("include should take precedence: expected 1 schema, got %d", len(result))
	}
	if result[0].Kind != "Foo" {
		t.Errorf("expected Foo, got %s", result[0].Kind)
	}
}

func TestFilterIncludeNoMatch(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Include: []string{"NonExistentKind"},
	}

	result := Filter(schemas, src)
	if len(result) != 0 {
		t.Errorf("expected 0 schemas for non-matching include, got %d", len(result))
	}
}

func TestFilterExcludeGroupDoesNotMatchOtherGroup(t *testing.T) {
	schemas := testSchemas()
	src := source.Source{
		Name:    "test",
		Exclude: []string{"other.group.io/GatewayClass"},
	}

	result := Filter(schemas, src)
	if len(result) != len(schemas) {
		t.Errorf("wrong group should not match: expected %d, got %d", len(schemas), len(result))
	}
}

// --- Conflict detection tests ---

func TestDetectConflictsNone(t *testing.T) {
	schemas := []CRDSchema{
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"type":"object"}`), SourceName: "src-a"},
		{Group: "b.io", Kind: "Bar", APIVersion: "v1", Schema: []byte(`{"type":"object"}`), SourceName: "src-b"},
	}

	conflicts := DetectConflicts(nopLog, schemas)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(conflicts))
	}
}

func TestDetectConflictsIdenticalDuplicates(t *testing.T) {
	schemas := []CRDSchema{
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"type":"object"}`), SourceName: "src-a"},
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"type":"object"}`), SourceName: "src-b"},
	}

	conflicts := DetectConflicts(nopLog, schemas)
	if len(conflicts) != 0 {
		t.Errorf("identical duplicates should not be conflicts, got %d", len(conflicts))
	}
}

func TestDetectConflictsIdenticalIgnoresFormatting(t *testing.T) {
	schemas := []CRDSchema{
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"type": "object"}`), SourceName: "src-a"},
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"type":"object"}`), SourceName: "src-b"},
	}

	conflicts := DetectConflicts(nopLog, schemas)
	if len(conflicts) != 0 {
		t.Errorf("formatting-only differences should not be conflicts, got %d", len(conflicts))
	}
}

func TestDetectConflictsDifferentContent(t *testing.T) {
	schemas := []CRDSchema{
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"type":"object"}`), SourceName: "src-a"},
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"type":"string"}`), SourceName: "src-b"},
	}

	conflicts := DetectConflicts(nopLog, schemas)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	c := conflicts[0]
	if c.Group != "a.io" {
		t.Errorf("conflict group = %q, want %q", c.Group, "a.io")
	}
	if c.Kind != "Foo" {
		t.Errorf("conflict kind = %q, want %q", c.Kind, "Foo")
	}
	if c.APIVersion != "v1" {
		t.Errorf("conflict apiVersion = %q, want %q", c.APIVersion, "v1")
	}
	if len(c.Sources) != 2 {
		t.Errorf("expected 2 conflicting sources, got %d", len(c.Sources))
	}
}

func TestDetectConflictsMultiple(t *testing.T) {
	schemas := []CRDSchema{
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"a":1}`), SourceName: "src-a"},
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"a":2}`), SourceName: "src-b"},
		{Group: "b.io", Kind: "Bar", APIVersion: "v1", Schema: []byte(`{"b":1}`), SourceName: "src-a"},
		{Group: "b.io", Kind: "Bar", APIVersion: "v1", Schema: []byte(`{"b":2}`), SourceName: "src-c"},
		{Group: "c.io", Kind: "Baz", APIVersion: "v1", Schema: []byte(`{"c":1}`), SourceName: "src-a"}, // no conflict
	}

	conflicts := DetectConflicts(nopLog, schemas)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(conflicts))
	}
}

func TestDetectConflictsThreeSources(t *testing.T) {
	schemas := []CRDSchema{
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"v":1}`), SourceName: "src-a"},
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"v":2}`), SourceName: "src-b"},
		{Group: "a.io", Kind: "Foo", APIVersion: "v1", Schema: []byte(`{"v":3}`), SourceName: "src-c"},
	}

	conflicts := DetectConflicts(nopLog, schemas)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if len(conflicts[0].Sources) != 3 {
		t.Errorf("expected 3 sources in conflict, got %d", len(conflicts[0].Sources))
	}
}

func TestDetectConflictsEmpty(t *testing.T) {
	conflicts := DetectConflicts(nopLog, nil)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts for nil input, got %d", len(conflicts))
	}
}

// --- matchPattern tests ---

func TestMatchPatternKindOnly(t *testing.T) {
	s := CRDSchema{Group: "example.com", Kind: "Foo"}
	if !matchPattern(s, "Foo") {
		t.Error("should match kind 'Foo'")
	}
	if matchPattern(s, "Bar") {
		t.Error("should not match kind 'Bar'")
	}
}

func TestMatchPatternKindCaseInsensitive(t *testing.T) {
	s := CRDSchema{Group: "example.com", Kind: "GatewayClass"}
	if !matchPattern(s, "gatewayclass") {
		t.Error("should match case-insensitively")
	}
	if !matchPattern(s, "GATEWAYCLASS") {
		t.Error("should match case-insensitively")
	}
}

func TestMatchPatternGroupKind(t *testing.T) {
	s := CRDSchema{Group: "example.com", Kind: "Foo"}
	if !matchPattern(s, "example.com/Foo") {
		t.Error("should match group/kind")
	}
	if matchPattern(s, "other.com/Foo") {
		t.Error("should not match wrong group")
	}
	if matchPattern(s, "example.com/Bar") {
		t.Error("should not match wrong kind")
	}
}

func TestMatchPatternGroupKindCaseInsensitive(t *testing.T) {
	s := CRDSchema{Group: "example.com", Kind: "GatewayClass"}
	if !matchPattern(s, "example.com/gatewayclass") {
		t.Error("kind part should be case-insensitive")
	}
}

func TestMatchPatternGroupWildcard(t *testing.T) {
	s := CRDSchema{Group: "example.com", Kind: "Foo"}
	if !matchPattern(s, "example.com/*") {
		t.Error("should match group wildcard")
	}
	if matchPattern(s, "other.com/*") {
		t.Error("should not match wrong group with wildcard")
	}
}

// --- SchemaKey test ---

func TestSchemaKey(t *testing.T) {
	s := CRDSchema{Group: "example.com", Kind: "Foo", APIVersion: "v1"}
	key := SchemaKey(s)
	if key != "example.com/Foo/v1" {
		t.Errorf("SchemaKey = %q, want %q", key, "example.com/Foo/v1")
	}
}

// --- jsonEqual tests ---

func TestJSONEqualIdentical(t *testing.T) {
	if !jsonEqual([]byte(`{"a":1}`), []byte(`{"a":1}`)) {
		t.Error("identical JSON should be equal")
	}
}

func TestJSONEqualDifferentFormatting(t *testing.T) {
	if !jsonEqual([]byte(`{"a": 1, "b": 2}`), []byte(`{"b":2,"a":1}`)) {
		t.Error("same content with different formatting/key order should be equal")
	}
}

func TestJSONEqualDifferentContent(t *testing.T) {
	if jsonEqual([]byte(`{"a":1}`), []byte(`{"a":2}`)) {
		t.Error("different content should not be equal")
	}
}

func TestJSONEqualInvalidJSON(t *testing.T) {
	if jsonEqual([]byte(`not json`), []byte(`{"a":1}`)) {
		t.Error("invalid JSON should not be equal")
	}
	if jsonEqual([]byte(`{"a":1}`), []byte(`not json`)) {
		t.Error("invalid JSON should not be equal")
	}
}
