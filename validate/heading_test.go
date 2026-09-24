package validate

import (
	"testing"

	"gitlab.com/lyoneel/markly"
)

func boolPtr(v bool) *bool { return &v }

func headingsFrom(t *testing.T, doc string) []*markly.MDHeading {
	t.Helper()
	md := markly.NewMDFileFromString(doc, markly.WithFenceAwareHeadings())
	c, err := md.GetContent()
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	return c.Headings
}

func checkHeadings(t *testing.T, doc, textMode, hierarchyMode, orderMode string, expected []ExpectedHeading, passed bool, want []HeadingIssue) {
	t.Helper()
	res, err := ValidateHeadings(headingsFrom(t, doc), textMode, hierarchyMode, orderMode, expected)
	if err != nil {
		t.Fatalf("ValidateHeadings: %v", err)
	}
	if res.Passed != passed {
		t.Fatalf("passed = %v, want %v, issues = %+v", res.Passed, passed, res.Issues)
	}
	if len(res.Issues) != len(want) {
		t.Fatalf("got %d issues %+v, want %d %+v", len(res.Issues), res.Issues, len(want), want)
	}
	for i := range want {
		if res.Issues[i].Severity != want[i].Severity || res.Issues[i].Field != want[i].Field {
			t.Fatalf("issue %d = %+v, want field %q severity %q", i, res.Issues[i], want[i].Field, want[i].Severity)
		}
	}
}

func h(text string, level int) ExpectedHeading {
	return ExpectedHeading{Text: text, Level: level}
}

func opt(text string, level int) ExpectedHeading {
	return ExpectedHeading{Text: text, Level: level, Required: boolPtr(false)}
}

func errIssue(field string) HeadingIssue {
	return HeadingIssue{Field: field, Severity: severityError}
}

func warnIssue(field string) HeadingIssue {
	return HeadingIssue{Field: field, Severity: severityWarning}
}

func TestStrictTextExtraWordFails(t *testing.T) {
	checkHeadings(t, "# Notes extra\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Notes", 1)}, false, []HeadingIssue{errIssue("Notes")})
}

func TestStrictTextCaseOnlyPasses(t *testing.T) {
	checkHeadings(t, "# notes\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Notes", 1)}, true, nil)
}

func TestStrictTextCollapsedWhitespacePasses(t *testing.T) {
	checkHeadings(t, "# Notes  here\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Notes here", 1)}, true, nil)
}

func TestStrictTextTrailingPunctuationFails(t *testing.T) {
	checkHeadings(t, "# Notes.\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Notes", 1)}, false, []HeadingIssue{errIssue("Notes")})
}

func TestPermissiveTextWordOrderPasses(t *testing.T) {
	checkHeadings(t, "# bar foo\n", textPermissive, hierarchyStrict, orderLoose, []ExpectedHeading{h("foo bar", 1)}, true, nil)
}

func TestPermissiveTextMissingWordFails(t *testing.T) {
	checkHeadings(t, "# foo\n", textPermissive, hierarchyStrict, orderLoose, []ExpectedHeading{h("foo bar", 1)}, false, []HeadingIssue{errIssue("foo bar")})
}

func TestPermissiveTextCatDoesNotMatchCategory(t *testing.T) {
	checkHeadings(t, "# category\n", textPermissive, hierarchyStrict, orderLoose, []ExpectedHeading{h("cat", 1)}, false, []HeadingIssue{errIssue("cat")})
}

func TestPermissiveTextPunctuationIgnored(t *testing.T) {
	checkHeadings(t, "# Notes.\n", textPermissive, hierarchyStrict, orderLoose, []ExpectedHeading{h("Notes", 1)}, true, nil)
}

func TestStrictHierarchyLevelMismatchFails(t *testing.T) {
	checkHeadings(t, "## Notes\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Notes", 1)}, false, []HeadingIssue{errIssue("Notes")})
}

func TestLooseHierarchySameDeeperPassShallowerFails(t *testing.T) {
	checkHeadings(t, "## Notes\n", textStrict, hierarchyLoose, orderLoose, []ExpectedHeading{h("Notes", 2)}, true, nil)
	checkHeadings(t, "### Notes\n", textStrict, hierarchyLoose, orderLoose, []ExpectedHeading{h("Notes", 2)}, true, nil)
	checkHeadings(t, "# Notes\n", textStrict, hierarchyLoose, orderLoose, []ExpectedHeading{h("Notes", 2)}, false, []HeadingIssue{errIssue("Notes")})
}

func TestIgnoreHierarchyAnyLevelPasses(t *testing.T) {
	checkHeadings(t, "### Notes\n", textStrict, hierarchyIgnore, orderLoose, []ExpectedHeading{{Text: "Notes"}}, true, nil)
}

func TestItemTextModeOverridesCall(t *testing.T) {
	item := h("Notes", 1)
	item.TextMode = textStrict
	checkHeadings(t, "# Notes extra\n", textPermissive, hierarchyStrict, orderLoose, []ExpectedHeading{item}, false, []HeadingIssue{errIssue("Notes")})
}

func TestItemHierarchyModeOverridesCall(t *testing.T) {
	item := h("Notes", 2)
	item.HierarchyMode = hierarchyIgnore
	checkHeadings(t, "# Notes\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{item}, true, nil)
}

func TestStrictOrderOptionalAbsentPasses(t *testing.T) {
	doc := "# Overview\n# Install\n"
	exp := []ExpectedHeading{h("Overview", 1), opt("Notes", 1), h("Install", 1)}
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderStrict, exp, true, nil)
}

func TestStrictOrderOptionalPresentInPositionPasses(t *testing.T) {
	doc := "# Overview\n# Notes\n# Install\n"
	exp := []ExpectedHeading{h("Overview", 1), opt("Notes", 1), h("Install", 1)}
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderStrict, exp, true, nil)
}

func TestStrictOrderUnmatchedBeforeFails(t *testing.T) {
	checkHeadings(t, "# Extra\n# Overview\n", textStrict, hierarchyStrict, orderStrict, []ExpectedHeading{h("Overview", 1)}, false, []HeadingIssue{errIssue("Extra")})
}

func TestStrictOrderUnmatchedBetweenFails(t *testing.T) {
	doc := "# Overview\n# Extra\n# Install\n"
	exp := []ExpectedHeading{h("Overview", 1), h("Install", 1)}
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderStrict, exp, false, []HeadingIssue{errIssue("Extra")})
}

func TestStrictOrderUnmatchedAfterFails(t *testing.T) {
	doc := "# Overview\n# Extra\n"
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderStrict, []ExpectedHeading{h("Overview", 1)}, false, []HeadingIssue{errIssue("Extra")})
}

func TestStrictOrderReversedRequiredFails(t *testing.T) {
	doc := "# Install\n# Overview\n"
	exp := []ExpectedHeading{h("Overview", 1), h("Install", 1)}
	res, err := ValidateHeadings(headingsFrom(t, doc), textStrict, hierarchyStrict, orderStrict, exp)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed {
		t.Fatalf("passed = true, issues = %+v", res.Issues)
	}
	if len(res.Issues) == 0 || res.Issues[0].Severity != severityError || res.Issues[0].Field != "Install" {
		t.Fatalf("issues = %+v", res.Issues)
	}
}

func TestStrictOrderSecondMatchFails(t *testing.T) {
	checkHeadings(t, "# Overview\n# Overview\n", textStrict, hierarchyStrict, orderStrict, []ExpectedHeading{h("Overview", 1)}, false, []HeadingIssue{errIssue("Overview")})
}

func TestLooseOrderUnmatchedBetweenPasses(t *testing.T) {
	doc := "# Overview\n# Extra\n# Install\n"
	exp := []ExpectedHeading{h("Overview", 1), h("Install", 1)}
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderLoose, exp, true, nil)
}

func TestLooseOrderSecondMatchPasses(t *testing.T) {
	checkHeadings(t, "# Overview\n# Overview\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Overview", 1)}, true, nil)
}

func TestLooseOrderReversedRequiredFails(t *testing.T) {
	doc := "# Install\n# Overview\n"
	exp := []ExpectedHeading{h("Overview", 1), h("Install", 1)}
	res, err := ValidateHeadings(headingsFrom(t, doc), textStrict, hierarchyStrict, orderLoose, exp)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed {
		t.Fatalf("passed = true, issues = %+v", res.Issues)
	}
	seen := false
	for _, iss := range res.Issues {
		if iss.Severity == severityError && iss.Field == "Install" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("issues = %+v", res.Issues)
	}
}

func TestOmittedRequiredMeansRequired(t *testing.T) {
	checkHeadings(t, "# Other\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Overview", 1)}, false, []HeadingIssue{errIssue("Overview")})
}

func TestWhenPresentParentAndChildAbsentPasses(t *testing.T) {
	parent := opt("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{h("Details", 2)}
	checkHeadings(t, "# Other\n", textStrict, hierarchyIgnore, orderLoose, []ExpectedHeading{parent}, true, nil)
}

func TestWhenPresentOrphanChildWarns(t *testing.T) {
	parent := opt("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{h("Details", 2)}
	checkHeadings(t, "# Details\n", textStrict, hierarchyIgnore, orderLoose, []ExpectedHeading{parent}, true, []HeadingIssue{warnIssue("Details")})
}

func TestWhenPresentRequiredChildMissingFails(t *testing.T) {
	parent := h("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{h("Details", 2)}
	checkHeadings(t, "# Overview\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{parent}, false, []HeadingIssue{errIssue("Details")})
}

func TestWhenPresentOptionalChildAbsentPasses(t *testing.T) {
	parent := h("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{opt("History", 2)}
	checkHeadings(t, "# Overview\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{parent}, true, nil)
}

func TestWhenPresentChildAtParentLevelIsNotSubheading(t *testing.T) {
	parent := h("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{h("Details", 2)}
	checkHeadings(t, "# Overview\n# Details\n", textStrict, hierarchyStrict, orderStrict, []ExpectedHeading{parent}, false, []HeadingIssue{errIssue("Details")})
}

func TestWhenPresentChildAfterParentSpanIsNotSubheading(t *testing.T) {
	parent := h("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{h("Details", 2)}
	next := h("Install", 1)
	doc := "# Overview\n# Install\n## Details\n"
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderStrict, []ExpectedHeading{parent, next}, false, []HeadingIssue{errIssue("Details")})
}

func TestNestedWhenPresentEnforcedOnlyWhenChildPresent(t *testing.T) {
	deep := h("Deep", 3)
	details := opt("Details", 2)
	details.WhenPresent = []ExpectedHeading{deep}
	parent := h("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{details}
	checkHeadings(t, "# Overview\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{parent}, true, nil)
	checkHeadings(t, "# Overview\n## Details\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{parent}, false, []HeadingIssue{errIssue("Deep")})
	checkHeadings(t, "# Overview\n## Details\n### Deep\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{parent}, true, nil)
}

func TestOptionalParentPresentEnforcesChildren(t *testing.T) {
	parent := opt("Notes", 1)
	parent.WhenPresent = []ExpectedHeading{h("Detail", 2)}
	checkHeadings(t, "# Notes\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{parent}, false, []HeadingIssue{errIssue("Detail")})
}

func TestOptionalParentAbsentDoesNotEnforceChildren(t *testing.T) {
	parent := opt("Notes", 1)
	parent.WhenPresent = []ExpectedHeading{h("Detail", 2)}
	checkHeadings(t, "", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{parent}, true, nil)
}

func TestFenceBacktickHeadingIgnored(t *testing.T) {
	doc := "```\n# Hidden\n```\n"
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Hidden", 1)}, false, []HeadingIssue{errIssue("Hidden")})
}

func TestFenceTildeHeadingIgnored(t *testing.T) {
	doc := "~~~\n# Hidden\n~~~\n"
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Hidden", 1)}, false, []HeadingIssue{errIssue("Hidden")})
}

func TestHeadingAfterFenceIsSeen(t *testing.T) {
	doc := "```\n# Hidden\n```\n\n# After\n"
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("After", 1)}, true, nil)
}

func TestSetextUnderlineIsNotATX(t *testing.T) {
	doc := "Overview\n========\n"
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Overview", 1)}, false, []HeadingIssue{errIssue("Overview")})
}

func TestEmptyExpectedNoHeadingsPasses(t *testing.T) {
	res, err := ValidateHeadings(nil, textStrict, hierarchyStrict, orderStrict, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed || len(res.Issues) != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestNoHeadingsRequiredItemFails(t *testing.T) {
	checkHeadings(t, "body only\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Overview", 1)}, false, []HeadingIssue{errIssue("Overview")})
}

func TestFrontmatterOnlyHasNoHeadings(t *testing.T) {
	doc := "---\ntitle: Only\n---\n"
	hs := headingsFrom(t, doc)
	if len(hs) != 0 {
		t.Fatalf("headings = %d, want 0", len(hs))
	}
	checkHeadings(t, doc, textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Overview", 1)}, false, []HeadingIssue{errIssue("Overview")})
}

func TestPermissiveEarlierItemConsumesHeading(t *testing.T) {
	exp := []ExpectedHeading{h("red fox", 1), h("red", 1)}
	checkHeadings(t, "# red fox\n", textPermissive, hierarchyStrict, orderLoose, exp, false, []HeadingIssue{errIssue("red")})
}

func TestUnicodeCaseFoldStrict(t *testing.T) {
	checkHeadings(t, "# über\n", textStrict, hierarchyStrict, orderLoose, []ExpectedHeading{h("Über", 1)}, true, nil)
}

func TestWarningOnlyPassedTrue(t *testing.T) {
	parent := opt("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{h("Details", 2)}
	checkHeadings(t, "# Details\n", textStrict, hierarchyIgnore, orderLoose, []ExpectedHeading{parent}, true, []HeadingIssue{warnIssue("Details")})
}

func TestWarningAndErrorPassedFalse(t *testing.T) {
	parent := opt("Overview", 1)
	parent.WhenPresent = []ExpectedHeading{h("Details", 2)}
	exp := []ExpectedHeading{parent, h("Install", 1)}
	want := []HeadingIssue{warnIssue("Details"), errIssue("Install")}
	checkHeadings(t, "# Details\n", textStrict, hierarchyIgnore, orderLoose, exp, false, want)
}
