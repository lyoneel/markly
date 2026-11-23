// Package markly provides tools for reading, parsing, and managing
// Markdown files with YAML or TOML frontmatter metadata. It supports
// lazy loading, dependency resolution between files, typed metadata
// access, and efficient batch operations using the dirly integration.
package markly

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
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

// ErrMetadataSerialization reports that the metadata could not be
// turned back into frontmatter text: a custom serializer returned an
// error, or a value has no YAML/TOML form (a func, a channel, ...).
// Render, Save and SaveAtomic return it wrapped; detect it with
// errors.Is. Save and SaveAtomic never touch the file when it occurs.
var ErrMetadataSerialization = errors.New("metadata serialization failed")

// Serialize returns the full file content as a string, including frontmatter and body.
// Frontmatter is serialized based on md.Type ("yaml" or "toml").
// If md.metadata is nil or empty, no frontmatter is included.
// In-memory documents (string/bytes constructors) load their content
// on demand so the body is never lost. For lazy disk-backed files
// without a loaded content, only frontmatter is returned.
//
// Serialize returns an empty string when the metadata cannot be
// serialized; use Render to get the error.
func (md *MDFile) Serialize() string {
	out, err := md.Render()
	if err != nil {
		return ""
	}
	return out
}

// Render is Serialize with error reporting: it returns the full file
// content, or an error wrapping ErrMetadataSerialization when the
// metadata cannot be serialized (a failing custom serializer, or a
// value YAML/TOML cannot encode).
func (md *MDFile) Render() (string, error) {
	out, _, err := md.render()
	return out, err
}

// render builds the file content and reports how many metadata lines
// the frontmatter block holds.
func (md *MDFile) render() (string, int, error) {
	if md.content == nil && md.source != nil {
		_ = md.LoadContent()
	}
	var fileContent strings.Builder
	metaLines := 0

	// Add frontmatter if metadata exists and Type is set
	if md.metadata != nil && len(md.metadata.data) > 0 && md.Type != "" {
		metadataLines, err := md.serializeMetadata()
		if err != nil {
			return "", 0, err
		}
		metaLines = len(metadataLines)
		delimiter := getDelimiterForType(md.Type)
		fileContent.WriteString(delimiter + newlineChar)
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

	return fileContent.String(), metaLines, nil
}

// prepareSave loads the body and renders the file content. Nothing is
// written and no state changes when rendering fails.
func (md *MDFile) prepareSave() (string, int, error) {
	if md.path == "" {
		return "", 0, fmt.Errorf("cannot save: file path is empty")
	}
	if err := md.ensureSourceContent(); err != nil {
		return "", 0, err
	}
	return md.render()
}

// Save writes the MDFile back to disk, preserving YAML/TOML frontmatter format.
// Recalculates line ranges if metadata was modified. When the metadata
// cannot be serialized, Save returns an error wrapping
// ErrMetadataSerialization and leaves the file untouched.
//
// Save writes in place (os.WriteFile): a new file gets DefaultFilePerm
// subject to umask, and a symlink is followed. Files loaded through an
// MDFolder are written through the folder instead (see SaveAtomic).
// Use SaveAtomic when a crash must never leave a partial file.
func (md *MDFile) Save() error {
	out, metaLines, err := md.prepareSave()
	if err != nil {
		return err
	}
	if w := md.folderWriter(); w != nil {
		err = w([]byte(out))
	} else {
		err = os.WriteFile(md.path, []byte(out), DefaultFilePerm) // #nosec G306 -- markdown documents are meant to be readable; umask applies
	}
	if err != nil {
		return err
	}
	md.refreshMetadataLineRange(metaLines)
	return nil
}

// SaveAtomic writes the MDFile back to disk atomically: the content goes
// to a uniquely named temp file in the target's directory, is fsynced,
// and is renamed over the target, and the directory is then fsynced on
// POSIX (best effort). A crash leaves either the old or the new content.
//
//   - An existing file keeps its mode and, on Unix, its owner and group
//     where the process is allowed to set them.
//   - A symlink is resolved and its target is replaced; the link stays.
//   - A new file gets DefaultFilePerm, subject to umask.
//   - Serialization errors are returned before anything is written, and
//     on any failure the temp file is removed and the target is left
//     unchanged. A missing or unwritable directory is an error.
//
// Files loaded through an MDFolder are written through dirly's atomic
// writer instead, which applies the folder's symlink policy and keeps
// mode and ownership (failing with dirly.ErrOwnership when it cannot).
// After SetPath to another location the file is saved standalone.
func (md *MDFile) SaveAtomic() error {
	out, metaLines, err := md.prepareSave()
	if err != nil {
		return err
	}
	if w := md.folderWriter(); w != nil {
		err = w([]byte(out))
	} else {
		err = writeFileAtomic(md.path, []byte(out), DefaultFilePerm)
	}
	if err != nil {
		return err
	}
	md.refreshMetadataLineRange(metaLines)
	return nil
}

// folderWriter returns the MDFolder writer when the file still points at
// the path it was loaded from, or nil for standalone files.
func (md *MDFile) folderWriter() func([]byte) error {
	if md.write != nil && md.path == md.writePath {
		return md.write
	}
	return nil
}

// refreshMetadataLineRange recalculates the frontmatter line range
// after metadata modifications, given the number of serialized
// metadata lines.
func (md *MDFile) refreshMetadataLineRange(metaLines int) {
	if md.metadata != nil && md.metadata.modified {
		newLineCount := metaLines + 3 // delimiters (2) + blank line (1)
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

// serializeMetadata serializes metadata back to lines based on Type
// format. Failures wrap ErrMetadataSerialization.
func (md *MDFile) serializeMetadata() ([]string, error) {
	if md.metadata == nil || len(md.metadata.data) == 0 {
		return nil, nil
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
// An error from fn makes Render, Save and SaveAtomic fail with an
// error wrapping both ErrMetadataSerialization and fn's error.
func (md *MDFile) SetSerializer(fn func(meta map[string]any) (string, error)) {
	md.serializer = fn
}

// serializeYaml serializes metadata to YAML lines.
func (md *MDFile) serializeYaml() ([]string, error) {
	if md.serializer != nil {
		out, err := md.serializer(md.metadata.data)
		if err != nil {
			return nil, fmt.Errorf("%w: custom serializer: %w", ErrMetadataSerialization, err)
		}
		return strings.Split(strings.TrimSpace(out), newlineChar), nil
	}

	yamlBytes, err := safeMarshal(yaml.Marshal, md.metadata.data)
	if err != nil {
		return nil, fmt.Errorf("%w: yaml: %w", ErrMetadataSerialization, err)
	}
	return strings.Split(strings.TrimSpace(string(yamlBytes)), newlineChar), nil
}

// serializeToml serializes metadata to TOML lines.
func (md *MDFile) serializeToml() ([]string, error) {
	tomlBytes, err := safeMarshal(toml.Marshal, md.metadata.data)
	if err != nil {
		return nil, fmt.Errorf("%w: toml: %w", ErrMetadataSerialization, err)
	}
	return strings.Split(strings.TrimSpace(string(tomlBytes)), newlineChar), nil
}

// safeMarshal calls marshal and turns a panic into an error. yaml.v3
// panics instead of returning an error for values it cannot encode,
// such as funcs and channels.
func safeMarshal(marshal func(any) ([]byte, error), v any) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("cannot encode metadata: %v", r)
		}
	}()
	return marshal(v)
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

	strictDuplicates bool // reject duplicate top-level frontmatter keys

	serializer func(map[string]any) (string, error) // custom YAML frontmatter serializer

	open func() (io.ReadCloser, error) // policy-checked reader for MDFolder files; nil reads md.path directly

	write     func([]byte) error // policy-checked atomic writer for MDFolder files
	writePath string             // path write belongs to; SetPath elsewhere disables it
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

// MaxLineLength is the longest line, in bytes, a document may contain.
// Lines up to this size (inline data: images, long tables, minified
// HTML) load normally; the scanner buffer grows only as far as the
// longest line needs. A longer line fails the load with an error
// wrapping bufio.ErrTooLong.
const MaxLineLength = 16 << 20

// newDocScanner returns a line scanner for a document that accepts
// lines up to MaxLineLength bytes.
func newDocScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxLineLength+1)
	return scanner
}

// scanErr returns the scanner error, naming the limit when a line is
// too long.
func scanErr(scanner *bufio.Scanner) error {
	err := scanner.Err()
	if errors.Is(err, bufio.ErrTooLong) {
		return fmt.Errorf("line longer than %d bytes: %w", MaxLineLength, err)
	}
	return err
}

// scannerLines adapts a bufio.Scanner to the line source used by
// scanFrontmatter.
func scannerLines(scanner *bufio.Scanner) func() (string, bool) {
	return func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}
}

// extractFrontmatter extracts the frontmatter block at the start of the
// document through the shared scanFrontmatter rules. It returns the
// block content lines, the format ("yaml" or "toml"), and the line
// range of the content (fromLine, toLine). No block yields no lines.
func (md *MDFile) extractFrontmatter(scanner *bufio.Scanner) ([]string, string, int, int, error) {
	block, _ := scanFrontmatter(scannerLines(scanner))
	if err := scanErr(scanner); err != nil {
		return nil, "", 0, 0, err
	}
	if !block.found || len(block.lines) == 0 {
		return nil, "", 0, 0, nil
	}
	return block.lines, string(block.format), 2, block.closeLine - 1, nil
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

// ErrExcessiveAliasing reports YAML frontmatter whose aliases expand
// into far more values than the document holds (a "billion laughs"
// document). The WithRawScalars parse returns it, wrapped, instead of
// expanding the aliases; detect it with errors.Is.
var ErrExcessiveAliasing = errors.New("excessive YAML alias expansion")

// Alias expansion budget for the WithRawScalars parse: the converted
// value may hold at most rawScalarBaseBudget values plus
// rawScalarAliasFactor values per node of the parsed document. Normal
// frontmatter, including anchors reused a handful of times, stays far
// below it; an alias bomb hits it after a few thousand steps.
const (
	rawScalarBaseBudget  = 10_000
	rawScalarAliasFactor = 10
)

// parseYamlRawScalars parses frontmatter through yaml.Node and converts
// scalars by tag: !!bool to bool, !!int to int64, !!float to float64,
// !!null to nil. Everything else (strings, timestamps, dates) keeps its
// original text so round-trips never rewrite values. Alias expansion is
// bounded; an excessive expansion fails with ErrExcessiveAliasing.
func parseYamlRawScalars(lines []string) (map[string]any, *yaml.Node, error) {
	root, err := parseYamlNode(lines)
	if err != nil {
		return nil, nil, err
	}

	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return map[string]any{}, root, nil
	}
	conv := &rawScalarConverter{budget: rawScalarBaseBudget + rawScalarAliasFactor*countYamlNodes(root)}
	converted, err := conv.value(root.Content[0])
	if err != nil {
		return nil, nil, err
	}
	value, ok := converted.(map[string]any)
	if !ok {
		return map[string]any{}, root, nil
	}
	return value, root, nil
}

// countYamlNodes counts the nodes of a parsed tree without following
// aliases, which is the size of the document as written.
func countYamlNodes(n *yaml.Node) int {
	count := 1
	for _, c := range n.Content {
		count += countYamlNodes(c)
	}
	return count
}

// rawScalarConverter converts a yaml node tree into plain Go values,
// preserving the original text of non-bool/int/float/null scalars.
// Every produced value spends one unit of budget, so alias expansion
// cannot grow without limit.
type rawScalarConverter struct {
	budget int
}

func (c *rawScalarConverter) value(n *yaml.Node) (any, error) {
	if c.budget <= 0 {
		return nil, ErrExcessiveAliasing
	}
	c.budget--
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return c.value(n.Content[0])
	case yaml.MappingNode:
		m := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			v, err := c.value(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m[n.Content[i].Value] = v
		}
		return m, nil
	case yaml.SequenceNode:
		s := make([]any, 0, len(n.Content))
		for _, item := range n.Content {
			v, err := c.value(item)
			if err != nil {
				return nil, err
			}
			s = append(s, v)
		}
		return s, nil
	case yaml.AliasNode:
		if n.Alias == nil {
			return nil, nil
		}
		return c.value(n.Alias)
	case yaml.ScalarNode:
		return rawScalar(n), nil
	}
	return nil, nil
}

// rawScalar converts one scalar node by tag.
func rawScalar(n *yaml.Node) any {
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

// WithStrictDuplicates rejects documents whose frontmatter repeats a
// top-level key. The parse fails with a *FrontmatterError naming the
// key and the line of the repeat. Without the option, behavior stays
// as before: the standard YAML path reports duplicates through its own
// unmarshal errors and the raw-scalar path lets the last value win.
func WithStrictDuplicates() FileOption {
	return func(md *MDFile) { md.strictDuplicates = true }
}

// openReader returns the reader to parse from: the in-memory source
// when set, otherwise the file at md.path. The returned closer is nil
// for in-memory sources.
//
// A failed open is a *fs.PathError for md.path that keeps the cause,
// so os.IsNotExist, os.IsPermission and errors.Is with os.ErrNotExist
// or os.ErrPermission all work. Its text starts with "metadata not
// found" for a missing file and "open failed" otherwise. Symlink policy
// refusals of MDFolder files wrap ErrPathEscapesRoot or
// ErrSymlinkNotAllowed.
func (md *MDFile) openReader() (io.ReadCloser, error) {
	if md.source != nil {
		return io.NopCloser(bytes.NewReader(md.source)), nil
	}
	var file io.ReadCloser
	var err error
	if md.open != nil {
		file, err = md.open()
	} else {
		file, err = os.Open(md.path)
	}
	if err != nil {
		return nil, md.openError(err)
	}
	return file, nil
}

// openError converts an open failure into the documented error shape.
func (md *MDFile) openError(err error) error {
	var pe *fs.PathError
	if !errors.As(err, &pe) || errors.Is(err, ErrPathEscapesRoot) || errors.Is(err, ErrSymlinkNotAllowed) {
		return fmt.Errorf("open failed: %s: %w", md.path, err)
	}
	op := "open failed"
	if errors.Is(err, fs.ErrNotExist) {
		op = "metadata not found"
	}
	return &fs.PathError{Op: op, Path: md.path, Err: pe.Err}
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

	scanner := newDocScanner(reader)
	frontmatterLines, format, fromLine, toLine, err := md.extractFrontmatter(scanner)
	if err != nil {
		return err
	}

	if len(frontmatterLines) == 0 {
		md.contentFrom = 1 // No frontmatter, content starts at line 1
		return nil         // No metadata
	}

	if md.strictDuplicates && format == "yaml" {
		if dupErr := checkDuplicateYamlKeys(frontmatterLines); dupErr != nil {
			return dupErr
		}
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

	scanner := newDocScanner(reader)

	// Frontmatter detection shares its rules with processMetadata and
	// SplitFrontmatter: a block opens on line 1 and closes with the same
	// delimiter. Without one, every line read so far is body text.
	block, consumed := scanFrontmatter(scannerLines(scanner))
	var pre []string
	if block.found {
		md.contentFrom = block.closeLine + 1
	} else {
		md.contentFrom = 1
		pre = consumed
	}
	content, err := md.parseMarkdownContent(scanner, pre)
	if err != nil {
		return fmt.Errorf("parse content: %w", err)
	}
	md.content = content
	return nil
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

// parseMarkdownContent parses markdown content and extracts headings with
// their text. The lines in pre are parsed first, then the rest of the
// scanner.
func (md *MDFile) parseMarkdownContent(scanner *bufio.Scanner, pre []string) (*MDContent, error) {
	content := &MDContent{
		Headings: make([]*MDHeading, 0),
		RawBody:  "",
	}

	var currentHeading *MDHeading
	var rawLines []string
	fenced := false

	next := scannerLines(scanner)
	for {
		var line string
		if len(pre) > 0 {
			line, pre = pre[0], pre[1:]
		} else {
			var ok bool
			if line, ok = next(); !ok {
				break
			}
		}

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
	return content, scanErr(scanner)
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
