package markly

import "strings"

// SlugHeading converts a heading title into a stable anchor: lowercase,
// runs of non-alphanumeric characters collapsed to a single hyphen,
// hyphens trimmed.
func SlugHeading(s string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// firstH1Index returns the body-relative line index of the first
// level-1 heading ("# Title") outside code fences, or -1 when none
// exists. Fences are toggled on lines starting with ``` or ~~~.
func (md *MDFile) firstH1Index() int {
	lines := md.bodyLines()
	fenced := false
	for i, line := range lines {
		if md.isFenceLine(line) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if after, ok := strings.CutPrefix(line, "# "); ok && strings.TrimSpace(after) != "" {
			return i
		}
	}
	return -1
}

// ExtractFirstH1 returns the text of the first level-1 heading outside
// code fences and whether one was found.
func (md *MDFile) ExtractFirstH1() (title string, found bool) {
	if err := md.ensureBodyLoaded(); err != nil {
		return "", false
	}
	idx := md.firstH1Index()
	if idx < 0 {
		return "", false
	}
	after, _ := strings.CutPrefix(md.bodyLines()[idx], "# ")
	return strings.TrimSpace(after), true
}

// RemoveFirstH1 strips the first level-1 heading line from the body.
// It is a no-op when no heading exists outside code fences.
func (md *MDFile) RemoveFirstH1() error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	idx := md.firstH1Index()
	if idx < 0 {
		return nil
	}
	lines := md.bodyLines()
	updated := append([]string{}, lines[:idx]...)
	updated = append(updated, lines[idx+1:]...)
	md.applyBodyLines(updated)
	return nil
}

// SetFirstHeading replaces the first level-1 heading with a new title,
// inserting one at the top of the body when none exists.
func (md *MDFile) SetFirstHeading(title string) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	lines := md.bodyLines()
	idx := md.firstH1Index()
	if idx >= 0 {
		lines[idx] = "# " + title
		md.applyBodyLines(lines)
		return nil
	}

	// No heading: insert "# title" at the top of the body, preceded by
	// a blank line when the body is non-empty so the heading stays a
	// separate block.
	updated := []string{"# " + title}
	if len(lines) > 0 && !(len(lines) == 1 && lines[0] == "") {
		updated = append(updated, "")
	}
	updated = append(updated, lines...)
	md.applyBodyLines(updated)
	return nil
}
