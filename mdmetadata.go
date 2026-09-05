package markly

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// MDMetadata provides typed access to YAML metadata stored as map[string]any
type MDMetadata struct {
	data     map[string]any
	FromLine int        // FromLine is the starting line number of YAML content (after opening ---)
	ToLine   int        // ToLine is the ending line number of YAML content (before closing ---)
	Type     FMType     // Format type: "yaml", "toml", or empty if no frontmatter
	modified bool       // internal only - tracks if metadata was changed since last save
	root     *yaml.Node // internal only - parsed YAML node tree for structure inspection (YAML only)
}

// NewMDMetadata creates a new MDMetadata from parsed YAML data with line range information
func NewMDMetadata(data map[string]any, fromLine, toLine int) *MDMetadata {
	return &MDMetadata{data: data, FromLine: fromLine, ToLine: toLine}
}

// NewMDMetadataWithFormat creates a new MDMetadata from parsed YAML data with format type
func NewMDMetadataWithFormat(data map[string]any, fromLine, toLine int, format FMType) *MDMetadata {
	return &MDMetadata{data: data, FromLine: fromLine, ToLine: toLine, Type: format}
}

// NewMDMetadataSimple creates a new MDMetadata from parsed YAML data without line range info
func NewMDMetadataSimple(data map[string]any) *MDMetadata {
	return &MDMetadata{data: data}
}

// GetString returns a string value for the given key, or empty string if not found/not a string
func (m *MDMetadata) GetString(key string) string {
	if v, ok := m.data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt returns an int value for the given key, or 0 if not found/not a number
func (m *MDMetadata) GetInt(key string) int {
	if v, ok := m.data[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64: // YAML numbers are parsed as float64 by default
			return int(n)
		case int64: // TOML uses int64 for integers
			return int(n)
		}
	}
	return 0
}

// GetBool returns a bool value for the given key, or false if not found/not a bool
func (m *MDMetadata) GetBool(key string) bool {
	if v, ok := m.data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetList returns a []any for the given key, or nil if not found/not a list
func (m *MDMetadata) GetList(key string) []any {
	if v, ok := m.data[key]; ok {
		if l, ok := v.([]any); ok {
			return l
		}
	}
	return nil
}

// GetStringList returns a []string for the given key, or nil if not found/not a list of strings.
// Handles both YAML lists (- item1, - item2) and scalar strings (value).
func (m *MDMetadata) GetStringList(key string) []string {
	if v, ok := m.data[key]; ok {
		if l, ok := v.([]any); ok {
			result := make([]string, 0, len(l))
			for _, item := range l {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
		if s, ok := v.(string); ok {
			return []string{s}
		}
	}
	return nil
}

// GetMap returns a nested map[string]any for the given key, or nil if not found/not a map
func (m *MDMetadata) GetMap(key string) map[string]any {
	if v, ok := m.data[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}

// UnmarshalInto unmarshals the metadata into a custom struct using yaml tags.
// Uses yaml.v3 for optimal performance - single optimized pass with no reflection overhead.
func (m *MDMetadata) UnmarshalInto(target any) error {
	yamlBytes, err := yaml.Marshal(m.data)
	if err != nil {
		return fmt.Errorf("marshal to bytes: %w", err)
	}

	return yaml.Unmarshal(yamlBytes, target)
}

// Get returns the raw value for a key (useful for custom type assertions)
func (m *MDMetadata) Get(key string) any {
	return m.data[key]
}

// Data returns the underlying metadata map. Mutate it only through the
// Set* and Delete methods so the modified flag stays accurate for Save.
func (m *MDMetadata) Data() map[string]any {
	return m.data
}

// IsFlowSequence reports whether the value of key is a YAML sequence
// written in flow style ([a, b]). Returns false when the key is
// missing, not a sequence, or the frontmatter is not YAML.
func (m *MDMetadata) IsFlowSequence(key string) bool {
	mapping := m.rootMapping()
	if mapping == nil {
		return false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}
		val := mapping.Content[i+1]
		return val.Kind == yaml.SequenceNode && val.Style&yaml.FlowStyle != 0
	}
	return false
}

// HasBlockScalarSequence reports whether the frontmatter contains at
// least one top-level block-formatted sequence whose items are all
// scalars (e.g. a tags list written as "- a" lines).
func (m *MDMetadata) HasBlockScalarSequence() bool {
	mapping := m.rootMapping()
	if mapping == nil {
		return false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		val := mapping.Content[i+1]
		if val.Kind != yaml.SequenceNode || val.Style&yaml.FlowStyle != 0 {
			continue
		}
		if scalarOnlySequence(val) {
			return true
		}
	}
	return false
}

// rootMapping returns the top-level mapping node of the parsed YAML
// document, or nil when unavailable.
func (m *MDMetadata) rootMapping() *yaml.Node {
	if m.root == nil || m.root.Kind != yaml.DocumentNode || len(m.root.Content) == 0 {
		return nil
	}
	mapping := m.root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	return mapping
}

// scalarOnlySequence reports whether every item of the sequence is a
// scalar node.
func scalarOnlySequence(seq *yaml.Node) bool {
	for _, item := range seq.Content {
		if item.Kind != yaml.ScalarNode {
			return false
		}
	}
	return true
}

// Keys returns all top-level keys in the metadata
func (m *MDMetadata) Keys() []string {
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

// GetType returns the format type of the frontmatter ("yaml", "toml", or empty if none)
func (m *MDMetadata) GetType() string {
	return string(m.Type)
}

// Set sets a value for the given key
func (m *MDMetadata) Set(key string, value any) {
	m.data[key] = value
	m.modified = true
}

// SetString sets a string value for the given key
func (m *MDMetadata) SetString(key string, value string) {
	m.data[key] = value
	m.modified = true
}

// SetInt sets an int value for the given key
func (m *MDMetadata) SetInt(key string, value int) {
	m.data[key] = int64(value)
	m.modified = true
}

// SetBool sets a bool value for the given key
func (m *MDMetadata) SetBool(key string, value bool) {
	m.data[key] = value
	m.modified = true
}

// SetList sets a []any list value for the given key
func (m *MDMetadata) SetList(key string, value []any) {
	m.data[key] = value
	m.modified = true
}

// SetStringList sets a []string list value for the given key
func (m *MDMetadata) SetStringList(key string, value []string) {
	anyList := make([]any, len(value))
	for i, v := range value {
		anyList[i] = v
	}
	m.data[key] = anyList
	m.modified = true
}

// SetMap sets a nested map[string]any value for the given key
func (m *MDMetadata) SetMap(key string, value map[string]any) {
	m.data[key] = value
	m.modified = true
}

// Delete removes a key from the metadata
func (m *MDMetadata) Delete(key string) {
	delete(m.data, key)
	m.modified = true
}

// NewMDMetadataYaml creates a new MDMetadata from parsed YAML data with line range info
func NewMDMetadataYaml(data map[string]any, fromLine, toLine int) *MDMetadata {
	return NewMDMetadataWithFormat(data, fromLine, toLine, FMTypeYAML)
}

// NewMDMetadataToml creates a new MDMetadata from parsed TOML data with line range info
func NewMDMetadataToml(data map[string]any, fromLine, toLine int) *MDMetadata {
	return NewMDMetadataWithFormat(data, fromLine, toLine, FMTypeTOML)
}
