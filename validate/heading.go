package validate

import (
	"fmt"
	"strings"
	"unicode"

	"gitlab.com/lyoneel/markly"
)

const (
	textStrict      = "strict"
	textPermissive  = "permissive"
	hierarchyStrict = "strict"
	hierarchyLoose  = "loose"
	hierarchyIgnore = "ignore"
	orderStrict     = "strict"
	orderLoose      = "loose"
	severityError   = "error"
	severityWarning = "warning"
)

// ExpectedHeading is one heading the caller wants to find. Required nil
// means the heading is required. TextMode and HierarchyMode, when set,
// override the modes passed to ValidateHeadings for this item only.
// WhenPresent lists child headings that apply only when this heading is
// found. A child sits at a deeper level than the parent and before the
// next heading at the parent's level or shallower.
type ExpectedHeading struct {
	Text          string            `json:"text"`
	Level         int               `json:"level,omitempty"`
	Required      *bool             `json:"required,omitempty"`
	TextMode      string            `json:"text_mode,omitempty"`
	HierarchyMode string            `json:"hierarchy_mode,omitempty"`
	WhenPresent   []ExpectedHeading `json:"when_present,omitempty"`
}

// HeadingIssue is one heading-check finding. Severity is "error" or
// "warning". Field is the heading text the finding is about.
type HeadingIssue struct {
	File     string `json:"file"`
	Field    string `json:"field"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

// HeadingResult is the outcome of ValidateHeadings. Passed is true when
// no issue has severity error. Warnings do not clear Passed.
type HeadingResult struct {
	Passed bool           `json:"passed"`
	Issues []HeadingIssue `json:"issues"`
}

type resolvedHeading struct {
	text          string
	level         int
	required      bool
	textMode      string
	hierarchyMode string
	whenPresent   []resolvedHeading
}

// ValidateHeadings compares headings with an expected list. headings must
// come from a parse that used WithFenceAwareHeadings. textMode is strict
// or permissive, hierarchyMode is strict, loose, or ignore, and orderMode
// is strict or loose. The function does not read frontmatter and does not
// call Schema.ValidateDocument. It takes no document type, section key, or
// template name. An empty mode or an unknown mode returns an error.
func ValidateHeadings(headings []*markly.MDHeading, textMode, hierarchyMode, orderMode string, expected []ExpectedHeading) (HeadingResult, error) {
	if err := checkCallModes(textMode, hierarchyMode, orderMode); err != nil {
		return HeadingResult{}, err
	}
	items, err := resolveAll(expected, textMode, hierarchyMode)
	if err != nil {
		return HeadingResult{}, err
	}
	issues := align(headings, items, orderMode)
	return HeadingResult{Passed: !hasError(issues), Issues: issues}, nil
}

func checkCallModes(textMode, hierarchyMode, orderMode string) error {
	switch textMode {
	case textStrict, textPermissive:
	default:
		return fmt.Errorf("text_mode must be strict or permissive")
	}
	switch hierarchyMode {
	case hierarchyStrict, hierarchyLoose, hierarchyIgnore:
	default:
		return fmt.Errorf("hierarchy_mode must be strict, loose, or ignore")
	}
	switch orderMode {
	case orderStrict, orderLoose:
	default:
		return fmt.Errorf("order_mode must be strict or loose")
	}
	return nil
}

func resolveAll(expected []ExpectedHeading, textMode, hierarchyMode string) ([]resolvedHeading, error) {
	out := make([]resolvedHeading, len(expected))
	for i, item := range expected {
		resolved, err := resolveOne(item, textMode, hierarchyMode)
		if err != nil {
			return nil, err
		}
		out[i] = resolved
	}
	return out, nil
}

func resolveOne(item ExpectedHeading, textMode, hierarchyMode string) (resolvedHeading, error) {
	tm := textMode
	if item.TextMode != "" {
		switch item.TextMode {
		case textStrict, textPermissive:
			tm = item.TextMode
		default:
			return resolvedHeading{}, fmt.Errorf("expected heading %q: text_mode must be strict or permissive", item.Text)
		}
	}
	hm := hierarchyMode
	if item.HierarchyMode != "" {
		switch item.HierarchyMode {
		case hierarchyStrict, hierarchyLoose, hierarchyIgnore:
			hm = item.HierarchyMode
		default:
			return resolvedHeading{}, fmt.Errorf("expected heading %q: hierarchy_mode must be strict, loose, or ignore", item.Text)
		}
	}
	if hm != hierarchyIgnore && (item.Level < 1 || item.Level > 6) {
		return resolvedHeading{}, fmt.Errorf("expected heading %q: level must be 1 through 6 when hierarchy mode is %s", item.Text, hm)
	}
	children, err := resolveAll(item.WhenPresent, textMode, hierarchyMode)
	if err != nil {
		return resolvedHeading{}, err
	}
	required := true
	if item.Required != nil {
		required = *item.Required
	}
	return resolvedHeading{
		text:          item.Text,
		level:         item.Level,
		required:      required,
		textMode:      tm,
		hierarchyMode: hm,
		whenPresent:   children,
	}, nil
}

func align(headings []*markly.MDHeading, items []resolvedHeading, orderMode string) []HeadingIssue {
	issues := []HeadingIssue{}
	used := make([]bool, len(items))
	var pending []*markly.MDHeading
	ei := 0
	i := 0
	for i < len(headings) {
		h := headings[i]
		kind, idx := findMatch(h, items, used, ei)
		switch kind {
		case kindItem, kindEarly:
			if kind == kindEarly {
				issues = append(issues, issue(h.Text, fmt.Sprintf("heading appears before required heading %q", items[ei].text), severityError))
			}
			if !hierarchyOK(h.Level, items[idx]) {
				issues = append(issues, issue(h.Text, hierarchyReason(h.Level, items[idx]), severityError))
			}
			used[idx] = true
			ei = idx + 1
			i++
			if len(items[idx].whenPresent) > 0 {
				span, next := takeSpan(headings, i, h.Level)
				issues = append(issues, align(span, items[idx].whenPresent, orderMode)...)
				i = next
			}
		case kindDup:
			if orderMode == orderStrict {
				issues = append(issues, issue(h.Text, fmt.Sprintf("duplicate heading %q", items[idx].text), severityError))
			}
			i++
		case kindLate:
			issues = append(issues, issue(h.Text, fmt.Sprintf("heading appears after later heading %q was already passed", items[idx].text), severityError))
			used[idx] = true
			i++
		default:
			pending = append(pending, h)
			i++
		}
	}
	issues = append(issues, resolvePending(pending, items, used, orderMode)...)
	for k, item := range items {
		if item.required && !used[k] {
			issues = append(issues, issue(item.text, fmt.Sprintf("missing required heading %q", item.text), severityError))
		}
	}
	return dropSuppressed(issues)
}

const (
	kindNone = iota
	kindItem
	kindDup
	kindEarly
	kindLate
)

func findMatch(h *markly.MDHeading, items []resolvedHeading, used []bool, ei int) (int, int) {
	for j := ei; j < len(items); j++ {
		if textMatch(h.Text, items[j]) {
			for k := ei; k < j; k++ {
				if items[k].required {
					return kindEarly, j
				}
			}
			return kindItem, j
		}
		if items[j].required {
			for k := j + 1; k < len(items); k++ {
				if textMatch(h.Text, items[k]) {
					return kindEarly, k
				}
			}
			break
		}
	}
	for k := range items {
		if used[k] && textMatch(h.Text, items[k]) {
			return kindDup, k
		}
	}
	for k := 0; k < ei; k++ {
		if !used[k] && textMatch(h.Text, items[k]) {
			return kindLate, k
		}
	}
	return kindNone, -1
}

func takeSpan(headings []*markly.MDHeading, i, parentLevel int) ([]*markly.MDHeading, int) {
	j := i
	for j < len(headings) && headings[j].Level > parentLevel {
		j++
	}
	return headings[i:j], j
}

func resolvePending(pending []*markly.MDHeading, items []resolvedHeading, used []bool, orderMode string) []HeadingIssue {
	var issues []HeadingIssue
	for _, h := range pending {
		present, absent := findChildHit(h.Text, items, used, true)
		switch {
		case present != nil && present.required && !present.found:
			issues = append(issues, issue(h.Text, fmt.Sprintf("heading %q is not a subheading of %q", h.Text, present.parent), severityError))
			issues = append(issues, issue(present.text, suppressMark, severityError))
		case present != nil && orderMode == orderStrict:
			issues = append(issues, issue(h.Text, fmt.Sprintf("unexpected heading %q", h.Text), severityError))
		case present == nil && absent != nil:
			issues = append(issues, issue(h.Text, fmt.Sprintf("heading %q appears without parent %q", h.Text, absent.parent), severityWarning))
		case orderMode == orderStrict:
			issues = append(issues, issue(h.Text, fmt.Sprintf("unexpected heading %q", h.Text), severityError))
		}
	}
	return issues
}

const suppressMark = "suppress-missing"

type childHit struct {
	text     string
	parent   string
	required bool
	found    bool
}

func findChildHit(heading string, items []resolvedHeading, used []bool, parentUsed bool) (present, absent *childHit) {
	for i, item := range items {
		thisUsed := parentUsed && i < len(used) && used[i]
		for _, child := range item.whenPresent {
			if !textMatch(heading, child) {
				continue
			}
			hit := &childHit{text: child.text, parent: item.text, required: child.required, found: false}
			if thisUsed {
				if present == nil {
					present = hit
				}
			} else if absent == nil {
				absent = hit
			}
		}
		p, a := findChildHit(heading, item.whenPresent, nil, thisUsed)
		if present == nil {
			present = p
		}
		if absent == nil {
			absent = a
		}
	}
	return present, absent
}

func dropSuppressed(issues []HeadingIssue) []HeadingIssue {
	suppress := map[string]int{}
	for _, iss := range issues {
		if iss.Reason == suppressMark {
			suppress[iss.Field]++
		}
	}
	out := make([]HeadingIssue, 0, len(issues))
	for _, iss := range issues {
		if iss.Reason == suppressMark {
			continue
		}
		if suppress[iss.Field] > 0 && strings.HasPrefix(iss.Reason, "missing required heading") {
			suppress[iss.Field]--
			continue
		}
		out = append(out, iss)
	}
	return out
}

func hierarchyOK(level int, item resolvedHeading) bool {
	switch item.hierarchyMode {
	case hierarchyIgnore:
		return true
	case hierarchyLoose:
		return level >= item.level
	default:
		return level == item.level
	}
}

func hierarchyReason(level int, item resolvedHeading) string {
	switch item.hierarchyMode {
	case hierarchyLoose:
		return fmt.Sprintf("heading level %d is shallower than %d", level, item.level)
	default:
		return fmt.Sprintf("heading level %d, want %d", level, item.level)
	}
}

func textMatch(heading string, item resolvedHeading) bool {
	if item.textMode == textPermissive {
		return permissiveMatch(heading, item.text)
	}
	return strings.EqualFold(collapseWS(heading), collapseWS(item.text))
}

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func permissiveMatch(heading, expected string) bool {
	want := words(expected)
	if len(want) == 0 {
		return false
	}
	have := words(heading)
	for _, w := range want {
		found := false
		for _, h := range have {
			if strings.EqualFold(w, h) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func words(s string) []string {
	fields := strings.Fields(collapseWS(s))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		cleaned := strings.Map(func(r rune) rune {
			if unicode.IsPunct(r) {
				return -1
			}
			return r
		}, f)
		if cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

func issue(field, reason, severity string) HeadingIssue {
	return HeadingIssue{Field: field, Reason: reason, Severity: severity}
}

func hasError(issues []HeadingIssue) bool {
	for _, iss := range issues {
		if iss.Severity == severityError {
			return true
		}
	}
	return false
}
