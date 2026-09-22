package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gitlab.com/lyoneel/markly"
)

// Issue reports one frontmatter violation. The zero Field with a
// non-empty Reason marks a document-level problem (frontmatter absent
// or unparseable).
type Issue struct {
	File   string
	Field  string
	Reason string
}

// ShapeFunc validates one field value of a registered shape. The
// returned issues carry Field and Reason only; the engine stamps File.
// ctx is the caller-provided context value registered with
// RegisterShape and carries whatever domain state the shape needs.
type ShapeFunc func(ctx any, field Field, value any) []Issue

var dateRe = regexp.MustCompile(`^\d{4}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])$`)
var decimalRe = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)

// RegisterShape installs fn under name, overriding the builtin
// implementation when the name matches a generic shape.
func (s *Schema) RegisterShape(name string, fn ShapeFunc) {
	if s.shapes == nil {
		s.shapes = map[string]ShapeFunc{}
	}
	s.shapes[name] = fn
}

// ValidateDocument validates one document's frontmatter data against
// the schema. Fields report in schema document order. An unsupported
// shape name surfaces as a per-document issue, matching the fm_validate
// behavior this package ports.
func (s *Schema) ValidateDocument(name string, data map[string]any) []Issue {
	issues := []Issue{}
	add := func(field, reason string) {
		issues = append(issues, Issue{File: name, Field: field, Reason: reason})
	}

	for _, field := range s.Fields {
		value, present := data[field.Name]
		if !present || value == nil {
			if field.Required {
				add(field.Name, fmt.Sprintf("missing required field %s", field.Name))
			}
			continue
		}

		if fn, ok := s.shapes[field.Shape]; ok {
			for _, issue := range fn(s.Ctx, field, value) {
				issue.File = name
				issues = append(issues, issue)
			}
			continue
		}

		vocab := s.vocabFor(field)
		keyNames := field.Keys

		switch field.Shape {
		case "string":
			if _, ok := value.(string); !ok {
				add(field.Name, fmt.Sprintf("%s must be a string", field.Name))
			}
		case "number":
			if isBool(value) || !isNumber(value) {
				add(field.Name, fmt.Sprintf("%s must be a number", field.Name))
			}
		case "boolean":
			if !isBool(value) {
				add(field.Name, fmt.Sprintf("%s must be a boolean", field.Name))
			}
		case "date":
			if isInt(value) && !isBool(value) {
				value = fmt.Sprintf("%d", value)
			}
			str, ok := value.(string)
			if !ok || !dateRe.MatchString(str) {
				add(field.Name, fmt.Sprintf("%s must be a YYYYMMDD date string", field.Name))
			}
		case "single":
			str, ok := value.(string)
			if !ok {
				add(field.Name, fmt.Sprintf("%s must be a string", field.Name))
			} else if strings.TrimSpace(str) != "" && vocab != nil && !vocab[str] {
				add(field.Name, fmt.Sprintf("invalid %s value '%s'", field.Name, str))
			}
		case "list":
			items, ok := value.([]any)
			if !ok {
				add(field.Name, fmt.Sprintf("%s must be a list", field.Name))
			} else {
				for _, item := range items {
					str, isStr := item.(string)
					if !isStr || strings.TrimSpace(str) == "" {
						add(field.Name, fmt.Sprintf("%s entries must be non-empty strings", field.Name))
					} else if vocab != nil && !vocab[str] {
						add(field.Name, fmt.Sprintf("%s value '%s' not in controlled list", field.Name, str))
					}
				}
			}
		case "case":
			m, ok := value.(map[string]any)
			if !ok {
				add(field.Name, fmt.Sprintf("%s must be a map", field.Name))
			} else {
				for k, v := range m {
					if vocab != nil && !vocab[k] {
						add(field.Name, fmt.Sprintf("%s key '%s' not in controlled list", field.Name, k))
					}
					str, isStr := v.(string)
					if !isStr || !decimalRe.MatchString(str) {
						add(field.Name, fmt.Sprintf("%s[%s] must be a decimal string", field.Name, k))
					}
				}
			}
		case "object":
			m, ok := value.(map[string]any)
			if !ok {
				add(field.Name, fmt.Sprintf("%s must be a map", field.Name))
			} else {
				for _, k := range keyNames {
					// An empty string value counts as present.
					if _, has := m[k]; !has {
						add(field.Name, fmt.Sprintf("%s missing required key '%s'", field.Name, k))
					}
				}
			}
		case "objects":
			items, ok := value.([]any)
			if !ok {
				add(field.Name, fmt.Sprintf("%s must be a list", field.Name))
			} else {
				for i, item := range items {
					m, isMap := item.(map[string]any)
					if !isMap || len(m) == 0 {
						add(field.Name, fmt.Sprintf("%s[%d] must be a non-empty map", field.Name, i))
					} else {
						for _, k := range keyNames {
							// An empty string value counts as present.
							if _, has := m[k]; !has {
								add(field.Name, fmt.Sprintf("%s[%d] missing required key '%s'", field.Name, i, k))
							}
						}
					}
				}
			}
		default:
			add(field.Name, fmt.Sprintf("unsupported shape '%s' for field %s", field.Shape, field.Name))
		}
	}
	return issues
}

// ValidateMetadata validates one document's frontmatter through the
// parsed metadata and adds the style checks the plain data map cannot
// carry. A field declaring inline: true enforces, per shape:
//
//   - list and objects: the sequence must be a flow list ([a, b])
//   - object and case: the mapping must be a flow map ({a: 1})
//   - string: the value must not be a block scalar (| or >)
//
// The shape checks run on the decoded data exactly as in
// ValidateDocument. Style checks apply to YAML frontmatter only; TOML
// has no block style and always satisfies the rule. Missing fields and
// non-list or non-map values stay with the shape checks, so a value
// reports at most one style finding.
func (s *Schema) ValidateMetadata(name string, meta *markly.MDMetadata) []Issue {
	if meta == nil {
		// No frontmatter block: nothing to style-check.
		return s.ValidateDocument(name, map[string]any{})
	}
	data := meta.Data()
	issues := s.ValidateDocument(name, data)
	if meta.GetType() != "yaml" {
		return issues
	}
	for _, field := range s.Fields {
		styleIssue := s.inlineStyleIssue(name, field, data, meta)
		if styleIssue != "" {
			issues = append(issues, Issue{File: name, Field: field.Name, Reason: styleIssue})
		}
	}
	return issues
}

// inlineStyleIssue returns the reason for an inline rule violation on
// one field, or "" when the value satisfies the rule or the shape
// carries no style meaning.
func (s *Schema) inlineStyleIssue(name string, field Field, data map[string]any, meta *markly.MDMetadata) string {
	if !field.Inline {
		return ""
	}
	value, present := data[field.Name]
	if !present || value == nil {
		return ""
	}
	switch field.Shape {
	case "list", "objects":
		if _, isList := value.([]any); isList && !meta.IsFlowSequence(field.Name) {
			return fmt.Sprintf("%s must be an inline list", field.Name)
		}
	case "object", "case":
		if _, isMap := value.(map[string]any); isMap && !meta.IsFlowMapping(field.Name) {
			return fmt.Sprintf("%s must be an inline map", field.Name)
		}
	case "string":
		if _, isStr := value.(string); isStr && meta.IsBlockScalar(field.Name) {
			return fmt.Sprintf("%s must be an inline string", field.Name)
		}
	}
	return ""
}

// ValidateFile parses and validates one markdown document. Parse
// problems surface as document-level issues; the error return covers
// only read failures.
func (s *Schema) ValidateFile(path string) ([]Issue, error) {
	name := filepath.Base(path)
	data, err := documentFrontmatter(path)
	if err != nil {
		return []Issue{{File: name, Reason: err.Error()}}, nil
	}
	return s.ValidateDocument(name, data), nil
}

// ValidateDir validates every markdown document under target: a single
// file, a directory walked recursively, or a glob pattern. A target
// that matches no document is an error.
func (s *Schema) ValidateDir(target string) ([]Issue, error) {
	paths, err := CollectDocs(target)
	if err != nil {
		return nil, err
	}
	issues := []Issue{}
	for _, path := range paths {
		docIssues, err := s.ValidateFile(path)
		if err != nil {
			return nil, err
		}
		issues = append(issues, docIssues...)
	}
	return issues, nil
}

// documentFrontmatter parses one document through markly and maps
// parse failures onto issue reasons: absent, TOML, or empty
// frontmatter reports "no YAML frontmatter found"; broken YAML reports
// the document name with the parse error.
func documentFrontmatter(path string) (map[string]any, error) {
	name := filepath.Base(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	meta, _, err := markly.ParseFrontmatter(string(raw))
	if err != nil {
		if !errors.Is(err, markly.ErrNoFrontmatter) {
			return nil, fmt.Errorf("%s: YAML parse error: %v", name, err)
		}
		return nil, errNoFrontmatter{}
	}
	if meta == nil || meta.GetType() != "yaml" || meta.Data() == nil {
		return nil, errNoFrontmatter{}
	}
	return meta.Data(), nil
}

// errNoFrontmatter reports an absent, empty, or TOML frontmatter block.
type errNoFrontmatter struct{}

func (errNoFrontmatter) Error() string { return "no YAML frontmatter found" }

// CollectDocs resolves the target argument to a sorted document list:
// a directory walks recursively for .md files, an existing file passes
// through, and anything else goes through the recursive glob.
func CollectDocs(target string) ([]string, error) {
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("empty --docs argument")
	}
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		var docs []string
		err := filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
				docs = append(docs, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(docs)
		return docs, nil
	}
	if fileExists(target) {
		return []string{target}, nil
	}
	matched, err := globRecursive(target)
	if err != nil {
		return nil, err
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("no documents matched")
	}
	sort.Strings(matched)
	return matched, nil
}

func globRecursive(pattern string) ([]string, error) {
	regex, err := globToRegex(pattern)
	if err != nil {
		return nil, err
	}
	base := patternRoot(pattern)
	var matched []string
	err = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && regex.MatchString(filepath.ToSlash(path)) {
			matched = append(matched, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(matched))
	unique := matched[:0]
	for _, m := range matched {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	return unique, nil
}

func patternRoot(pattern string) string {
	clean := filepath.ToSlash(pattern)
	parts := strings.Split(clean, "/")
	var root []string
	for _, part := range parts[:len(parts)-1] {
		if strings.ContainsAny(part, "*?[") {
			break
		}
		root = append(root, part)
	}
	base := strings.Join(root, "/")
	if base == "" {
		base = "."
	}
	return base
}

func globToRegex(pattern string) (*regexp.Regexp, error) {
	pattern = filepath.ToSlash(pattern)
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 3
					continue
				}
				b.WriteString(".*")
				i += 2
				continue
			}
			b.WriteString("[^/]*")
			i++
		case '?':
			b.WriteString("[^/]")
			i++
		case '[':
			j := strings.IndexByte(pattern[i:], ']')
			if j < 0 {
				b.WriteString("\\[")
				i++
				continue
			}
			b.WriteString(pattern[i : i+j+1])
			i += j + 1
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// vocabFor returns the membership set for the field: the From
// vocabulary when set, otherwise the inline values.
func (s *Schema) vocabFor(field Field) map[string]bool {
	if field.From != "" {
		return s.Vocab[field.From]
	}
	if len(field.Values) == 0 {
		return nil
	}
	vocab := make(map[string]bool, len(field.Values))
	for _, v := range field.Values {
		vocab[v] = true
	}
	return vocab
}

func isBool(v any) bool {
	_, ok := v.(bool)
	return ok
}

func isNumber(v any) bool {
	switch v.(type) {
	case int, int64, float64, float32:
		return true
	}
	return false
}

func isInt(v any) bool {
	switch v.(type) {
	case int, int64:
		return true
	}
	return false
}
