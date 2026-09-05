package markly

import (
	"fmt"
	"strings"
)

// Body lines use absolute 1-based file line numbers: line 1 is the
// first line of the file, including the frontmatter delimiters. This
// matches anchor conventions where a stored line number identifies a
// position in the whole file, not in the body alone.

// ensureBodyLoaded makes sure the content is parsed before any body
// mutation runs.
func (md *MDFile) ensureBodyLoaded() error {
	if md.content != nil {
		return nil
	}
	lazy := md.lazyContent
	md.lazyContent = false
	defer func() { md.lazyContent = lazy }()
	return md.LoadContent()
}

// bodyLines splits the body into lines.
func (md *MDFile) bodyLines() []string {
	if md.content == nil {
		return nil
	}
	if md.content.RawBody == "" {
		return []string{}
	}
	return strings.Split(md.content.RawBody, "\n")
}

// applyBodyLines joins the lines back into the body and marks the
// content as the serialization source.
func (md *MDFile) applyBodyLines(lines []string) {
	md.content.RawBody = strings.Join(lines, "\n")
}

// frontmatterLineCount returns the number of file lines occupied by
// the frontmatter block including delimiters, plus the separating
// blank line Serialize inserts after it. Returns 0 when there is no
// frontmatter.
func (md *MDFile) frontmatterLineCount() int {
	if md.metadata == nil || len(md.metadata.data) == 0 || md.Type == "" {
		return 0
	}
	// opening delimiter + metadata lines + closing delimiter + blank line
	return len(md.serializeMetadata()) + 3
}

// AppendLine adds a line at the end of the body.
func (md *MDFile) AppendLine(line string) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	lines := md.bodyLines()
	lines = append(lines, line)
	md.applyBodyLines(lines)
	return nil
}

// lineToBodyIdx converts an absolute file line number to a body-relative
// index. With frontmatter, the block plus its blank separator occupies
// frontmatterLineCount lines; without frontmatter, body line 0 is file
// line 1.
func (md *MDFile) lineToBodyIdx(line int) int {
	if fm := md.frontmatterLineCount(); fm > 0 {
		return line - fm
	}
	return line - 1
}

// bodyIdxToFileLine converts a body-relative index back to an absolute
// file line number (inverse of lineToBodyIdx).
func (md *MDFile) bodyIdxToFileLine(bodyIdx int) int {
	if fm := md.frontmatterLineCount(); fm > 0 {
		return fm + bodyIdx
	}
	return bodyIdx + 1
}

// InsertLine inserts text before the given absolute file line number.
// InsertLine(len+1) appends at the end, matching ly-datum behavior
// where line numbers point at existing lines.
func (md *MDFile) InsertLine(line int, text string) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	lines := md.bodyLines()
	bodyIdx := md.lineToBodyIdx(line)
	if bodyIdx < 0 || bodyIdx > len(lines) {
		return fmt.Errorf("line %d out of range", line)
	}
	updated := append([]string{}, lines[:bodyIdx]...)
	updated = append(updated, text)
	updated = append(updated, lines[bodyIdx:]...)
	md.applyBodyLines(updated)
	return nil
}

// RemoveLine removes the line at the given absolute file line number.
func (md *MDFile) RemoveLine(line int) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	lines := md.bodyLines()
	bodyIdx := md.lineToBodyIdx(line)
	if bodyIdx < 0 || bodyIdx >= len(lines) {
		return fmt.Errorf("line %d out of range", line)
	}
	updated := append([]string{}, lines[:bodyIdx]...)
	updated = append(updated, lines[bodyIdx+1:]...)
	md.applyBodyLines(updated)
	return nil
}

// SetBody replaces the whole body text.
func (md *MDFile) SetBody(body string) {
	if md.content == nil {
		md.content = &MDContent{Headings: make([]*MDHeading, 0)}
	}
	md.content.RawBody = body
	md.content.Headings = make([]*MDHeading, 0)
}

// lineAt returns the body-relative index of an absolute file line
// number, or an error when out of range.
func (md *MDFile) lineAt(line int) (int, error) {
	lines := md.bodyLines()
	bodyIdx := md.lineToBodyIdx(line)
	if bodyIdx < 0 || bodyIdx >= len(lines) {
		return 0, fmt.Errorf("line %d out of range", line)
	}
	return bodyIdx, nil
}
