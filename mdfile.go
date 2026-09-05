// Package markly provides tools for reading, parsing, and managing
// Markdown files with YAML or TOML frontmatter metadata. It supports
// lazy loading, dependency resolution between files, typed metadata
// access, and efficient batch operations using the dirly integration.
package markly

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// FMType represents the frontmatter format type.
type FMType string

const (
	FMTypeYAML FMType = "yaml"
	FMTypeTOML FMType = "toml"
)

var validFMTypes = map[FMType]bool{
	FMTypeYAML: true,
	FMTypeTOML: true,
}

// Serialize returns the full file content as a string, including frontmatter and body.
// Frontmatter is serialized based on md.Type ("yaml" or "toml").
// If md.metadata is nil or empty, no frontmatter is included.
// In-memory documents (string/bytes constructors) load their content
// on demand so the body is never lost. For lazy disk-backed files
// without a loaded content, only frontmatter is returned.
func (md *MDFile) Serialize() string {
	if md.content == nil && md.source != nil {
		_ = md.LoadContent()
	}
	var fileContent strings.Builder

	// Add frontmatter if metadata exists and Type is set
	if md.metadata != nil && len(md.metadata.data) > 0 && md.Type != "" {
		delimiter := getDelimiterForType(md.Type)
		fileContent.WriteString(delimiter + newlineChar)
		metadataLines := md.serializeMetadata()
		for _, line := range metadataLines {
			fileContent.WriteString(line + newlineChar)
		}
		fileContent.WriteString(delimiter + newlineChar)
		// Separate frontmatter from the body with one blank line,
		// unless the body already carries it (round-trip stability).
		if md.content == nil || !strings.HasPrefix(md.content.RawBody, newlineChar) {
			fileContent.WriteString(newlineChar)
		}
	}

	// Add content body
	if md.content != nil && md.content.RawBody != "" {
		fileContent.WriteString(md.content.RawBody)
		if !strings.HasSuffix(md.content.RawBody, newlineChar) {
			fileContent.WriteString(newlineChar)
		}
	}

	return fileContent.String()
}

// Save writes the MDFile back to disk, preserving YAML/TOML frontmatter format.
// Recalculates line ranges if metadata was modified.
func (md *MDFile) Save() error {
	if md.path == "" {
		return fmt.Errorf("cannot save: file path is empty")
	}

	if err := md.ensureSourceContent(); err != nil {
		return err
	}
	md.refreshMetadataLineRange()

	return os.WriteFile(md.path, []byte(md.Serialize()), 0644)
}

// SaveAtomic writes the MDFile back to disk through a uniquely named
// temp file in the target directory, then renames it over the target.
// A crash mid-write leaves the previous file intact.
func (md *MDFile) SaveAtomic() error {
	if md.path == "" {
		return fmt.Errorf("cannot save: file path is empty")
	}

	if err := md.ensureSourceContent(); err != nil {
		return err
	}
	md.refreshMetadataLineRange()

	tmp, err := os.CreateTemp(filepath.Dir(md.path), filepath.Base(md.path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(md.Serialize()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, md.path)
}

// refreshMetadataLineRange recalculates the frontmatter line range
// after metadata modifications.
func (md *MDFile) refreshMetadataLineRange() {
	if md.metadata != nil && md.metadata.modified {
		lines := md.serializeMetadata()
		newLineCount := len(lines) + 3 // delimiters (2) + blank line (1)
		md.metadata.FromLine = 1
		md.metadata.ToLine = newLineCount - 2
		md.metadata.modified = false
	}
}

// ensureSourceContent loads the body before a save so documents built
// from strings keep their content even when LoadContent was never
// called explicitly.
func (md *MDFile) ensureSourceContent() error {
	if md.content != nil {
		return nil
	}
	lazy := md.lazyContent
	md.lazyContent = false
	defer func() { md.lazyContent = lazy }()
	return md.LoadContent()
}

// getDelimiterForType returns the appropriate delimiter for a given format type.
func getDelimiterForType(formatType FMType) string {
	if formatType == FMTypeTOML {
		return tomlDelimiter
	}
	return yamlDelimiter // default to YAML
}

// serializeMetadata serializes metadata back to lines based on Type format.
func (md *MDFile) serializeMetadata() []string {
	if md.metadata == nil || len(md.metadata.data) == 0 {
		return nil
	}

	switch md.Type {
	case FMTypeTOML:
		return md.serializeToml()
	default:
		return md.serializeYaml()
	}
}

// SetSerializer registers a custom frontmatter serializer for YAML
// documents. The returned text (without delimiters) is used verbatim
// as the frontmatter block. TOML documents keep the builtin marshaler.
func (md *MDFile) SetSerializer(fn func(meta map[string]any) (string, error)) {
	md.serializer = fn
}

// serializeYaml serializes metadata to YAML lines.
func (md *MDFile) serializeYaml() []string {
	if md.serializer != nil {
		out, err := md.serializer(md.metadata.data)
		if err != nil {
			return nil
		}
		return strings.Split(strings.TrimSpace(out), newlineChar)
	}

	yamlBytes, err := yaml.Marshal(md.metadata.data)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(yamlBytes)), newlineChar)
	return lines
}

// serializeToml serializes metadata to TOML lines.
func (md *MDFile) serializeToml() []string {
	tomlBytes, err := toml.Marshal(md.metadata.data)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(tomlBytes)), newlineChar)
	return lines
}

// MDFile represents a Markdown file with YAML frontmatter metadata.
type MDFile struct {
	path        string
	source      []byte      // In-memory source; non-nil for string/bytes constructors
	metadata    *MDMetadata // Frontmatter metadata
	content     *MDContent
	lazyContent bool
	contentFrom int    // Line number where content starts (after YAML frontmatter)
	Type        FMType // Format type: "yaml" or "toml"

	rawScalars bool // keep date-like scalars as original text
	fenceAware bool // skip headings inside fenced code blocks

	serializer func(map[string]any) (string, error) // custom YAML frontmatter serializer
}

// FileOption configures MDFile loading behavior.
type FileOption func(*MDFile)

// NewMDFile creates a new MDFile from the given path.
func NewMDFile(path string, opts ...FileOption) *MDFile {
	f := &MDFile{path: path, lazyContent: true}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func NewMDFileWithContent(path string, opts ...FileOption) (*MDFile, error) {
	file := &MDFile{path: path, lazyContent: false}
	for _, opt := range opts {
		opt(file)
	}
	if err := file.processMetadata(); err != nil {
		return nil, err
	}
	_, err := file.GetContent()
	return file, err
}

// NewMDFileFromString creates an MDFile from an in-memory document.
// The document is parsed the same way as a file on disk; use SetPath
// before Save to give it a write target.
func NewMDFileFromString(content string, opts ...FileOption) *MDFile {
	return NewMDFileFromBytes([]byte(content), opts...)
}

// NewMDFileFromBytes creates an MDFile from an in-memory document.
// Metadata is parsed immediately; content stays available through
// GetContent.
func NewMDFileFromBytes(content []byte, opts ...FileOption) *MDFile {
	file := &MDFile{source: content, lazyContent: false}
	for _, opt := range opts {
		opt(file)
	}
	if err := file.processMetadata(); err != nil {
		// Keep the file usable; metadata simply stays nil like a
		// missing frontmatter block.
		file.metadata = nil
	}
	return file
}

// SetPath sets the filesystem path used by Save and SaveAtomic.
func (md *MDFile) SetPath(path string) {
	md.path = path
}

// SetLazyLoading enables or disables lazy loading of content.
// When true, content is only loaded when explicitly accessed via GetContent().
func (md *MDFile) SetLazyLoading(enabled bool) {
	md.lazyContent = enabled
}

// SetType sets the frontmatter format type (FMTypeYAML or FMTypeTOML).
// This allows users to convert between formats before calling Save().
func (md *MDFile) SetType(formatType FMType) error {
	if !validFMTypes[formatType] {
		return fmt.Errorf("unsupported frontmatter format: %s (use 'yaml' or 'toml')", formatType)
	}
	md.Type = formatType
	return nil
}

// GetMetadata loads and returns the parsed metadata.
// If metadata has not been loaded yet, it reads and parses the frontmatter from disk.
func (md *MDFile) GetMetadata() (*MDMetadata, error) {
	if md.metadata == nil {
		if err := md.processMetadata(); err != nil {
			return nil, err
		}
	}
	return md.metadata, nil
}

// SetMetadata attaches a metadata block to the file. Intended for
// documents parsed without frontmatter: once set, Serialize emits a
// frontmatter block. When the file has no type yet, md.Type is taken
// from meta.Type so the serializer knows which delimiter to use. The
// metadata is marked modified so Save refreshes its line range.
func (md *MDFile) SetMetadata(meta *MDMetadata) {
	md.metadata = meta
	if meta == nil {
		return
	}
	if md.Type == "" && meta.Type != "" {
		md.Type = meta.Type
	}
	meta.modified = true
}

// extractFrontmatter extracts raw frontmatter lines between delimiters from markdown.
// Supports both YAML (---) and TOML (+++) formats.
// Returns the frontmatter lines, format ("yaml" or "toml"), and line range (fromLine, toLine).
func (md *MDFile) extractFrontmatter(scanner *bufio.Scanner) ([]string, string, int, int, error) {
	var lines []string
	inSection := false
	lineNum := 0
	fromLine := 0
	toLine := 0
	format := ""

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if !inSection && line == yamlDelimiter {
			inSection = true
			format = "yaml"
			continue // Skip opening delimiter, content starts on next line
		}

		if !inSection && line == tomlDelimiter {
			inSection = true
			format = "toml"
			continue // Skip opening delimiter, content starts on next line
		}

		if inSection && (line == yamlDelimiter || line == tomlDelimiter) {
			toLine = lineNum - 1 // Frontmatter ends before closing delimiter
			break
		}

		if inSection {
			if fromLine == 0 {
				fromLine = lineNum // First frontmatter content line
			}
			lines = append(lines, line)
		}
	}

	return lines, format, fromLine, toLine, scanner.Err()
}

// parseYaml dynamically unmarshals YAML into a generic map[string]any.
// Scalars become string/number/bool, sequences become []any,
// mappings become nested map[string]any.
func (md *MDFile) parseYaml(lines []string) (map[string]any, error) {
	yamlContent := strings.Join(lines, newlineChar)

	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(yamlContent), &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %w", err)
	}

	return metadata, nil
}

// parseYamlNode parses frontmatter lines into a yaml.Node tree.
func parseYamlNode(lines []string) (*yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines, newlineChar)), &root); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %w", err)
	}
	return &root, nil
}

// parseYamlRawScalars parses frontmatter through yaml.Node and converts
// scalars by tag: !!bool to bool, !!int to int64, !!float to float64,
// !!null to nil. Everything else (strings, timestamps, dates) keeps its
// original text so round-trips never rewrite values.
func parseYamlRawScalars(lines []string) (map[string]any, *yaml.Node, error) {
	root, err := parseYamlNode(lines)
	if err != nil {
		return nil, nil, err
	}

	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return map[string]any{}, root, nil
	}
	value, ok := rawScalarValue(root.Content[0]).(map[string]any)
	if !ok {
		return map[string]any{}, root, nil
	}
	return value, root, nil
}

// rawScalarValue converts a yaml node tree into plain Go values,
// preserving the original text of non-bool/int/float/null scalars.
func rawScalarValue(n *yaml.Node) any {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil
		}
		return rawScalarValue(n.Content[0])
	case yaml.MappingNode:
		m := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			m[n.Content[i].Value] = rawScalarValue(n.Content[i+1])
		}
		return m
	case yaml.SequenceNode:
		s := make([]any, 0, len(n.Content))
		for _, item := range n.Content {
			s = append(s, rawScalarValue(item))
		}
		return s
	case yaml.AliasNode:
		return rawScalarValue(n.Alias)
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!bool":
			b, err := strconv.ParseBool(n.Value)
			if err != nil {
				return n.Value
			}
			return b
		case "!!int":
			i, err := strconv.ParseInt(n.Value, 0, 64)
			if err != nil {
				return n.Value
			}
			return i
		case "!!float":
			f, err := strconv.ParseFloat(n.Value, 64)
			if err != nil {
				return n.Value
			}
			return f
		case "!!null":
			return nil
		default:
			// Strings, timestamps, and dates keep their original text.
			return n.Value
		}
	}
	return nil
}

// parseToml dynamically unmarshals TOML into a generic map[string]any.
// Scalars become string/number/bool, arrays become []any,
// tables become nested map[string]any.
func (md *MDFile) parseToml(lines []string) (map[string]any, error) {
	tomlContent := strings.Join(lines, newlineChar)

	var metadata map[string]any
	if err := toml.Unmarshal([]byte(tomlContent), &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %w", err)
	}

	return metadata, nil
}

// WithRawScalars keeps date-like and timestamp scalars in frontmatter
// as their original text instead of typed values. Booleans, integers,
// floats, and nulls keep their natural types.
func WithRawScalars() FileOption {
	return func(md *MDFile) { md.rawScalars = true }
}

// WithFenceAwareHeadings skips headings inside fenced code blocks
// (``` or ~~~) when building the heading list.
func WithFenceAwareHeadings() FileOption {
	return func(md *MDFile) { md.fenceAware = true }
}

// openReader returns the reader to parse from: the in-memory source
// when set, otherwise the file at md.path. The returned closer is nil
// for in-memory sources.
func (md *MDFile) openReader() (io.ReadCloser, error) {
	if md.source != nil {
		return io.NopCloser(bytes.NewReader(md.source)), nil
	}
	file, err := os.Open(md.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("metadata not found: %s", md.path)
		}
		return nil, fmt.Errorf("open failed: %s", md.path)
	}
	return file, nil
}

func (md *MDFile) processMetadata() error {
	if md.metadata != nil {
		return nil // Already parsed
	}
	reader, err := md.openReader()
	if err != nil {
		return err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	frontmatterLines, format, fromLine, toLine, err := md.extractFrontmatter(scanner)
	if err != nil {
		return err
	}

	if len(frontmatterLines) == 0 {
		md.contentFrom = 1 // No frontmatter, content starts at line 1
		return nil         // No metadata
	}

	if md.rawScalars && format == "yaml" {
		mdata, root, convErr := parseYamlRawScalars(frontmatterLines)
		if convErr != nil {
			return fmt.Errorf("parse metadata (%s): %w", format, convErr)
		}
		md.Type = FMTypeYAML
		md.metadata = NewMDMetadataYaml(mdata, fromLine, toLine)
		md.metadata.root = root
		return nil
	}

	var mdata map[string]any
	switch format {
	case "yaml":
		mdata, err = md.parseYaml(frontmatterLines)
	case "toml":
		mdata, err = md.parseToml(frontmatterLines)
	default:
		return fmt.Errorf("unsupported frontmatter format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("parse metadata (%s): %w", format, err)
	}

	md.Type = FMType(format)
	switch format {
	case "yaml":
		md.metadata = NewMDMetadataYaml(mdata, fromLine, toLine)
		root, nodeErr := parseYamlNode(frontmatterLines)
		if nodeErr == nil {
			md.metadata.root = root
		}
	case "toml":
		md.metadata = NewMDMetadataToml(mdata, fromLine, toLine)
	}
	return nil
}

// LoadContent loads and parses the markdown content (after YAML/TOML frontmatter).
// This is a lazy operation - only executed when explicitly called.
func (md *MDFile) LoadContent() error {
	if md.content != nil {
		return nil // Already loaded
	}

	reader, err := md.openReader()
	if err != nil {
		return err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)

	// Check if first line is frontmatter delimiter (YAML --- or TOML +++)
	hasFrontmatter := false
	var firstLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && line != carriageReturn && line != newlineChar {
			firstLine = line
			break
		}
	}

	hasFrontmatter = firstLine == yamlDelimiter || firstLine == tomlDelimiter

	var frontmatterLines []string
	if hasFrontmatter {
		// First line is already consumed, now read until closing delimiter
		lineNum := 1 // Start counting from first line (the opening delimiter)
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if line == yamlDelimiter || line == tomlDelimiter {
				break // Found closing delimiter (either --- or +++)
			}
			frontmatterLines = append(frontmatterLines, scanner.Text())
		}

		// Parse remaining content as markdown (start from next line after closing delimiter)
		md.contentFrom = lineNum + 1
		content, err := md.parseMarkdownContent(scanner)
		if err != nil {
			return fmt.Errorf("parse content: %w", err)
		}
		md.content = content
		return nil
	}

	// No frontmatter - parse entire file including first line using scanner
	md.contentFrom = 1
	content := &MDContent{
		Headings: make([]*MDHeading, 0),
		RawBody:  "",
	}

	lineNum := 1
	var currentHeading *MDHeading
	var rawLines []string

	// Add first line back to processing
	rawLines = append(rawLines, firstLine)
	fenced := false
	if md.isFenceLine(firstLine) {
		fenced = !fenced
	}
	if !fenced {
		if headingLevel, text := md.isHeading(firstLine); headingLevel > 0 {
			currentHeading = &MDHeading{
				Level:    headingLevel,
				Text:     strings.TrimSpace(text),
				FromLine: lineNum, // Heading occupies this single line
				ToLine:   lineNum, // Same as FromLine for ATX headings (single-line)
			}
			content.Headings = append(content.Headings, currentHeading)
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		rawLines = append(rawLines, line)

		if md.isFenceLine(line) {
			fenced = !fenced
		} else if !(md.fenceAware && fenced) {
			if headingLevel, text := md.isHeading(line); headingLevel > 0 {
				currentHeading = &MDHeading{
					Level:    headingLevel,
					Text:     strings.TrimSpace(text),
					FromLine: lineNum, // Heading occupies this single line
					ToLine:   lineNum, // Same as FromLine for ATX headings (single-line)
				}
				content.Headings = append(content.Headings, currentHeading)
			}
		}

		lineNum++
	}

	content.RawBody = strings.Join(rawLines, "\n")
	md.content = content
	return scanner.Err()
}

// GetContent returns the parsed content. If lazy loading is enabled and content
// hasn't been loaded yet, this method will trigger the load on first access.
func (md *MDFile) GetContent() (*MDContent, error) {
	if md.content != nil {
		return md.content, nil
	}

	if !md.lazyContent {
		// Eager loading - load immediately
		err := md.LoadContent()
		if err != nil {
			return nil, err
		}
		return md.content, nil
	}

	// Lazy loading - content will be loaded when explicitly requested via LoadContent()
	return nil, fmt.Errorf("content not yet loaded; call LoadContent() first")
}

// GetContentFrom returns the line number where content starts (after YAML frontmatter).
// Returns 0 if content hasn't been loaded yet.
func (md *MDFile) GetContentFrom() int {
	return md.contentFrom
}

// parseMarkdownContent parses markdown content and extracts headings with their text.
func (md *MDFile) parseMarkdownContent(scanner *bufio.Scanner) (*MDContent, error) {
	content := &MDContent{
		Headings: make([]*MDHeading, 0),
		RawBody:  "",
	}

	var currentHeading *MDHeading
	var rawLines []string
	fenced := false

	for scanner.Scan() {
		line := scanner.Text()

		rawLines = append(rawLines, line)

		if md.isFenceLine(line) {
			fenced = !fenced
			if currentHeading != nil {
				currentHeading.ToLine = len(rawLines)
			}
			continue
		}

		if md.fenceAware && fenced {
			if currentHeading != nil {
				currentHeading.ToLine = len(rawLines)
			}
			continue
		}

		// Check if this is a heading (ATX style: # ## ### etc.)
		if headingLevel, text := md.isHeading(line); headingLevel > 0 {
			currentHeading = &MDHeading{
				Level:    headingLevel,
				Text:     strings.TrimSpace(text),
				FromLine: len(rawLines),
				ToLine:   len(rawLines),
			}
			content.Headings = append(content.Headings, currentHeading)
		} else if currentHeading != nil {
			// Update ToLine for non-heading lines (heading spans multiple lines)
			currentHeading.ToLine = len(rawLines)
		}
	}

	content.RawBody = strings.Join(rawLines, "\n")
	return content, scanner.Err()
}

// isFenceLine reports whether the line opens or closes a fenced code
// block (``` or ~~~ prefix, optional leading whitespace).
func (md *MDFile) isFenceLine(line string) bool {
	trimmed := strings.TrimLeft(line, whitespaceChars)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// isHeading checks if a line is an ATX-style heading and returns its level (1-6) and text.
// Returns 0 if not a heading.
func (md *MDFile) isHeading(line string) (int, string) {
	trimmed := strings.TrimRight(strings.TrimLeft(line, whitespaceChars), carriageReturn+newlineChar)
	if !strings.HasPrefix(trimmed, headingMarker) {
		return 0, ""
	}

	level := 0
	for _, c := range trimmed {
		if c == '#' {
			level++
		} else if strings.ContainsRune(whitespaceChars, c) {
			break // Stop counting # when we hit whitespace
		} else {
			return 0, "" // Non-# non-whitespace char means not a heading
		}

		if level > maxHeadingLevel {
			return 0, "" // Max level is 6
		}
	}

	if level == 0 {
		return 0, ""
	}

	// Extract text after the heading markers and optional whitespace
	textStart := strings.Index(trimmed, headingMarker) + level
	for textStart < len(trimmed) && strings.ContainsRune(whitespaceChars, rune(trimmed[textStart])) {
		textStart++
	}

	return level, trimmed[textStart:]
}
