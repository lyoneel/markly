// Package validate implements a schema-driven validator for markdown
// frontmatter. A schema is a markdown file whose YAML frontmatter
// carries a fields map (required, shape, values, keys, from) and an
// optional config map of format constants. Domain-specific shapes stay
// caller-side through ShapeFunc registration; the package knows
// fields, shapes, and vocabularies only.
package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gitlab.com/lyoneel/markly"
	"gopkg.in/yaml.v3"
)

// ErrSchema reports a malformed schema file. LoadSchema wraps every
// load failure with it; use errors.Is to detect.
var ErrSchema = errors.New("invalid schema")

// Field declares one document field rule from the schema fields map.
type Field struct {
	Name     string
	Required bool
	Shape    string
	Values   []string // inline vocabulary, single and list shapes
	From     string   // vocabulary reference resolved against the schema location
	Keys     []string // required keys, object and objects shapes
}

// Schema is a loaded schema: the ordered field rules, the config
// constants, and the resolved vocabularies. Vocab carries one entry per
// From reference; callers may override an entry before validating when
// the reference needs domain-specific extraction. Config keeps the raw
// config values; scalar constants are string, bool, or number. Ctx is
// the caller-provided context handed to every registered ShapeFunc;
// set it before validating when the schema registers domain shapes.
type Schema struct {
	Fields []Field
	Config map[string]any
	Vocab  map[string]map[string]bool
	Ctx    any

	shapes map[string]ShapeFunc
}

// vocabPattern extracts non-empty non-# lines from fenced code blocks.
var vocabPattern = regexp.MustCompile("(?s)```[^\\n]*\\n(.*?)```")

// LoadSchema parses the schema file at path. The file is either a
// markdown document whose frontmatter carries fields and config, or a
// plain .yaml/.yml document with the same keys. Every From reference
// resolves at load time against the schema location (the schema
// directory, then the parent directory itself plus its assets and
// references folders); an unresolvable reference is a schema error.
func LoadSchema(path string) (*Schema, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchema, err)
	}
	name := filepath.Base(path)

	text := string(raw)
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		block, _, found := markly.SplitFrontmatter(text)
		if !found {
			return nil, fmt.Errorf("%w: %s: schema has no 'fields' mapping", ErrSchema, name)
		}
		text = block
	}

	var data map[string]any
	if err := yaml.Unmarshal([]byte(text), &data); err != nil {
		return nil, fmt.Errorf("%w: %s: YAML parse error: %v", ErrSchema, name, err)
	}
	order := schemaFieldOrder(text)
	if len(order) == 0 {
		return nil, fmt.Errorf("%w: %s: schema has no 'fields' mapping", ErrSchema, name)
	}
	fieldsRaw, _ := data["fields"].(map[string]any)

	schema := &Schema{Config: map[string]any{}, Vocab: map[string]map[string]bool{}, shapes: map[string]ShapeFunc{}}
	if configRaw, ok := data["config"].(map[string]any); ok {
		for key, value := range configRaw {
			schema.Config[key] = value
		}
	}

	for _, fieldName := range order {
		rules, _ := fieldsRaw[fieldName].(map[string]any)
		if rules == nil {
			rules = map[string]any{}
		}
		field := Field{Name: fieldName}
		field.Required, _ = rules["required"].(bool)
		field.Shape, _ = rules["shape"].(string)
		if field.Shape == "" {
			field.Shape = "string"
		}
		if from, _ := rules["from"].(string); from != "" {
			field.From = from
		}
		if values, ok := rules["values"].([]any); ok {
			for _, v := range values {
				field.Values = append(field.Values, fmt.Sprintf("%v", v))
			}
		}
		if keys, ok := rules["keys"].([]any); ok {
			for _, k := range keys {
				if s, ok := k.(string); ok {
					field.Keys = append(field.Keys, s)
				}
			}
		}
		schema.Fields = append(schema.Fields, field)
	}

	for _, field := range schema.Fields {
		if field.From == "" {
			continue
		}
		if _, seen := schema.Vocab[field.From]; seen {
			continue
		}
		vocab, err := readVocabulary(filepath.Dir(path), field.From)
		if err != nil {
			return nil, err
		}
		schema.Vocab[field.From] = vocab
	}
	return schema, nil
}

// schemaFieldOrder returns the document order of the fields mapping
// keys. The rules map decode loses order, so the keys come from a node
// walk over the same text.
func schemaFieldOrder(text string) []string {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return nil
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != "fields" {
			continue
		}
		fieldsMapping := mapping.Content[i+1]
		if fieldsMapping.Kind != yaml.MappingNode {
			return nil
		}
		order := make([]string, 0, len(fieldsMapping.Content)/2)
		for j := 0; j+1 < len(fieldsMapping.Content); j += 2 {
			order = append(order, fieldsMapping.Content[j].Value)
		}
		return order
	}
	return nil
}

// readVocabulary loads one From reference: the non-empty non-# lines
// inside fenced code blocks of the referenced markdown file. The
// search mirrors the fm_validate constant walk: the schema directory
// first, then the parent directory and its assets and references
// folders. References containing a slash resolve only inside the
// schema directory.
func readVocabulary(schemaDir, ref string) (map[string]bool, error) {
	root := filepath.Dir(schemaDir)
	var candidates []string
	if strings.Contains(ref, "/") {
		candidates = append(candidates, filepath.Join(schemaDir, ref+".md"))
	} else {
		for _, base := range []string{schemaDir, filepath.Join(root, "assets"), filepath.Join(root, "references"), root} {
			candidates = append(candidates, filepath.Join(base, ref+".md"))
		}
	}
	for _, candidate := range candidates {
		if !fileExists(candidate) {
			continue
		}
		raw, err := os.ReadFile(candidate)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrSchema, err)
		}
		values := make(map[string]bool)
		for _, block := range vocabPattern.FindAllStringSubmatch(string(raw), -1) {
			for _, line := range strings.Split(block[1], "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					values[line] = true
				}
			}
		}
		return values, nil
	}
	return nil, fmt.Errorf("%w: schema 'from' target '%s' not found as a constant file", ErrSchema, ref)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
