package markly

import (
	"fmt"
	"strings"
)

const checkboxPrefix = "- ["

// checkboxLine validates the line at the given body-relative index as a
// checkbox line ("- [ ]", "- [x]", "- [~]") and returns its trimmed
// form.
func (md *MDFile) checkboxLine(bodyIdx int) (string, error) {
	lines := md.bodyLines()
	trimmed := strings.TrimLeft(lines[bodyIdx], whitespaceChars)
	if len(trimmed) < 6 || !strings.HasPrefix(trimmed, checkboxPrefix) || trimmed[4] != ']' {
		return "", fmt.Errorf("line is not a checkbox")
	}
	return trimmed, nil
}

// SetCheckboxMarker flips the marker of the checkbox on the given
// absolute file line. The rest of the line stays intact. Valid markers
// are ' ', 'x', and '~'.
func (md *MDFile) SetCheckboxMarker(line int, marker byte) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	bodyIdx, err := md.lineAt(line)
	if err != nil {
		return err
	}

	lines := md.bodyLines()
	target := lines[bodyIdx]
	if _, err := md.checkboxLine(bodyIdx); err != nil {
		return fmt.Errorf("line %d of document is not a checkbox", line)
	}

	idx := strings.Index(target, "[")
	updated := []byte(target)
	updated[idx+1] = marker
	lines[bodyIdx] = string(updated)
	md.applyBodyLines(lines)
	return nil
}

// RewriteCheckboxTitle replaces only the title text of a checkbox line,
// keeping indent, marker, and trailing suffixes (due dates, capture
// timestamps).
func (md *MDFile) RewriteCheckboxTitle(line int, newTitle string) error {
	if err := md.ensureBodyLoaded(); err != nil {
		return err
	}
	bodyIdx, err := md.lineAt(line)
	if err != nil {
		return err
	}

	lines := md.bodyLines()
	target := lines[bodyIdx]
	trimmed := strings.TrimLeft(target, whitespaceChars)
	if _, err := md.checkboxLine(bodyIdx); err != nil {
		return fmt.Errorf("line %d of document is not a checkbox", line)
	}

	indent := target[:len(target)-len(trimmed)]
	rest := trimmed[6:]

	suffixes := ""
	for {
		open := strings.Index(rest, " (")
		if open < 0 {
			break
		}
		if !strings.HasSuffix(rest, ")") {
			break
		}
		inner := rest[open+2 : len(rest)-1]
		if strings.HasPrefix(inner, "due: ") || isCaptureTimestamp(inner) {
			suffixes = rest[open:] + suffixes
			rest = rest[:open]
			continue
		}
		break
	}

	lines[bodyIdx] = indent + trimmed[:6] + newTitle + suffixes
	md.applyBodyLines(lines)
	return nil
}

// isCaptureTimestamp reports whether s looks like a "2006-01-02 15:04"
// capture timestamp (16 characters, fixed punctuation).
func isCaptureTimestamp(s string) bool {
	if len(s) != 16 {
		return false
	}
	for i, r := range s {
		switch i {
		case 4, 7:
			if r != '-' {
				return false
			}
		case 10:
			if r != ' ' {
				return false
			}
		case 13:
			if r != ':' {
				return false
			}
		default:
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
