package markly

import "strings"

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
