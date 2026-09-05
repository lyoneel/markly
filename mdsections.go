package markly

import (
	"fmt"
	"strings"
)

const sectionHeadingPrefix = "## "

// FindSection returns the absolute file line number (1-based) of the
// `## ` heading whose slug matches, and whether it was found.
func (md *MDFile) FindSection(slug string) (line int, found bool) {
	if err := md.ensureBodyLoaded(); err != nil {
		return 0, false
	}
	idx := md.sectionBodyIndex(slug)
	if idx < 0 {
		return 0, false
	}
	return md.bodyIdxToFileLine(idx), true
}

// sectionBodyIndex returns the body-relative index of the `## ` heading
// whose slug matches, or -1.
func (md *MDFile) sectionBodyIndex(slug string) int {
	for i, line := range md.bodyLines() {
		if !strings.HasPrefix(line, sectionHeadingPrefix) {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, sectionHeadingPrefix))
		if SlugHeading(title) == slug {
			return i
		}
	}
	return -1
}

// SetSectionHeading renames the `## ` heading identified by slug.
func (md *MDFile) SetSectionHeading(slug, newTitle string) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	idx := md.sectionBodyIndex(slug)
	if idx < 0 {
		return fmt.Errorf("section %q not found", slug)
	}
	lines := md.bodyLines()
	lines[idx] = sectionHeadingPrefix + newTitle
	md.applyBodyLines(lines)
	return nil
}

// sectionBulletLike reports whether the line looks like a `- key: value`
// metadata bullet.
func sectionBulletLike(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "- ") {
		return false
	}
	return strings.Contains(trimmed, ": ")
}

// SetSectionBullet replaces or inserts `- key: value` directly under
// the section heading identified by slug. An existing bullet with the
// same key is replaced in place; otherwise the bullet is inserted on
// the line right after the heading.
func (md *MDFile) SetSectionBullet(slug, key, value string) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	idx := md.sectionBodyIndex(slug)
	if idx < 0 {
		return fmt.Errorf("section %q not found", slug)
	}
	lines := md.bodyLines()

	bullet := "- " + key + ": " + value
	for j := idx + 1; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if strings.HasPrefix(lines[j], sectionHeadingPrefix) {
			break
		}
		if strings.HasPrefix(trimmed, "- "+key+":") {
			lines[j] = bullet
			md.applyBodyLines(lines)
			return nil
		}
		if trimmed != "" {
			break
		}
	}

	updated := append([]string{}, lines[:idx+1]...)
	updated = append(updated, bullet)
	updated = append(updated, lines[idx+1:]...)
	md.applyBodyLines(updated)
	return nil
}

// InsertLineUnderHeading places line under the `## <heading>` heading,
// after any consecutive `- key: value` metadata bullets directly below
// it. When createMissing is true and the heading is absent, the heading
// and the line are appended at the end of the body.
func (md *MDFile) InsertLineUnderHeading(heading, line string, createMissing bool) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	lines := md.bodyLines()

	target := sectionHeadingPrefix + heading
	for i, existing := range lines {
		if strings.TrimSpace(existing) != target {
			continue
		}
		insertAt := i + 1
		for insertAt < len(lines) && sectionBulletLike(strings.TrimSpace(lines[insertAt])) {
			insertAt++
		}
		updated := append([]string{}, lines[:insertAt]...)
		updated = append(updated, line)
		updated = append(updated, lines[insertAt:]...)
		md.applyBodyLines(updated)
		return nil
	}

	if !createMissing {
		return fmt.Errorf("section %q not found", heading)
	}

	updated := lines
	if len(updated) > 0 && updated[len(updated)-1] != "" {
		updated = append(append([]string{}, updated...), "")
	} else if len(updated) == 0 {
		updated = []string{}
	}
	updated = append(updated, target, line)
	md.applyBodyLines(updated)
	return nil
}
