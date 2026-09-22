package markly

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// ErrNoFrontmatter reports that a document carries no frontmatter
// block. ParseFrontmatter returns it when the text does not open with
// a --- or +++ delimiter line; detect it with errors.Is.
var ErrNoFrontmatter = errors.New("no frontmatter")

// FrontmatterError wraps a frontmatter parse failure with the position
// where the parse broke. Line is the 1-based line within the full
// document text passed to ParseFrontmatter; it is 0 when the parser
// does not report a position.
type FrontmatterError struct {
	Line int
	Err  error
}

func (e *FrontmatterError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("frontmatter line %d: %v", e.Line, e.Err)
	}
	return fmt.Sprintf("frontmatter: %v", e.Err)
}

func (e *FrontmatterError) Unwrap() error { return e.Err }

// yamlLinePattern matches the line reference embedded in yaml.v3 error
// messages ("yaml: line 3: ..." and "yaml: unmarshal errors:\n  line 3: ...").
var yamlLinePattern = regexp.MustCompile(`line (\d+)`)

// yamlErrorLine extracts the line number referenced by a yaml.v3 error
// message, or 0 when the message carries none.
func yamlErrorLine(err error) int {
	m := yamlLinePattern.FindStringSubmatch(err.Error())
	if m == nil {
		return 0
	}
	line, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return 0
	}
	return line
}

// newFrontmatterError converts a raw YAML or TOML parse error into a
// *FrontmatterError. The yaml and toml packages report positions
// relative to the frontmatter block content, which starts on document
// line 2, so extracted positions shift by one.
func newFrontmatterError(err error) error {
	line := yamlErrorLine(err)
	if pos, ok := err.(*toml.DecodeError); ok {
		row, _ := pos.Position()
		line = row
	}
	if line > 0 {
		line++
	}
	return &FrontmatterError{Line: line, Err: err}
}

// ParseFrontmatter parses the frontmatter block of a markdown document
// and returns the metadata plus the body after the block. It replaces
// the NewMDFileFromString plus GetMetadata sequence when callers need
// honest error reporting. Errors:
//
//   - ErrNoFrontmatter when the text does not open with a delimiter
//   - *FrontmatterError when the block exists but fails to parse
//
// A present but empty block returns (nil, body, nil), matching the
// GetMetadata contract of no metadata without error. The metadata line
// range refers to the full document text.
func ParseFrontmatter(text string) (*MDMetadata, string, error) {
	raw, body, found := SplitFrontmatter(text)
	if !found {
		return nil, body, ErrNoFrontmatter
	}
	if strings.TrimSpace(raw) == "" {
		return nil, body, nil
	}
	lines := strings.Split(raw, "\n")

	firstLine, _, _ := strings.Cut(text, "\n")
	format := FMTypeYAML
	if strings.TrimSuffix(firstLine, "\r") == tomlDelimiter {
		format = FMTypeTOML
	}

	var data map[string]any
	var err error
	switch format {
	case FMTypeTOML:
		data, err = parseFrontmatterTOML(lines)
	default:
		data, err = parseFrontmatterYAML(lines)
	}
	if err != nil {
		return nil, body, newFrontmatterError(err)
	}
	return NewMDMetadataWithFormat(data, 2, len(lines)+1, format), body, nil
}

// parseFrontmatterYAML unmarshals frontmatter lines as YAML into a
// generic map.
func parseFrontmatterYAML(lines []string) (map[string]any, error) {
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(lines, newlineChar)), &metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

// parseFrontmatterTOML unmarshals frontmatter lines as TOML into a
// generic map.
func parseFrontmatterTOML(lines []string) (map[string]any, error) {
	var metadata map[string]any
	if err := toml.Unmarshal([]byte(strings.Join(lines, newlineChar)), &metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

// checkDuplicateYamlKeys reports the first duplicated top-level key in
// a frontmatter block as a *FrontmatterError naming the key and the
// document line of the repeat. Nested mappings stay outside the check;
// their duplicates follow the behavior of the active parse mode. Parse
// failures return nil so the regular parse path reports them.
func checkDuplicateYamlKeys(lines []string) error {
	root, err := parseYamlNode(lines)
	if err != nil {
		return nil
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	seen := make(map[string]int)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		if first, dup := seen[key.Value]; dup {
			return &FrontmatterError{
				Line: key.Line + 1,
				Err:  fmt.Errorf("duplicate frontmatter key %q (first defined at line %d)", key.Value, first+1),
			}
		}
		seen[key.Value] = key.Line
	}
	return nil
}

// SplitFrontmatter splits a markdown document into its raw frontmatter
// block and the body after it. The document opens with a --- (YAML) or
// +++ (TOML) delimiter on the first line; the block closes at the next
// delimiter line. raw carries the frontmatter content without the
// delimiters; body carries the text after the closing delimiter line.
// Line endings normalize to \n, matching the scanner-based loader.
// found reports whether a frontmatter block was detected.
// Without frontmatter, raw is empty, body is the whole text, and found
// is false. An opening delimiter without a closing one puts the rest of
// the document into raw with an empty body, mirroring the extraction
// the MDFile loader applies to malformed documents.
func SplitFrontmatter(text string) (raw string, body string, found bool) {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if len(lines) == 0 {
		return "", text, false
	}
	if lines[0] != yamlDelimiter && lines[0] != tomlDelimiter {
		return "", text, false
	}
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if line == yamlDelimiter || line == tomlDelimiter {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	return strings.Join(lines[1:], "\n"), "", true
}
