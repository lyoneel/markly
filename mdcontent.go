package markly

type MDHeading struct {
	Level    int    // Level is the heading level (1-6)
	Text     string // Text is the text of the heading
	FromLine int    // FromLine is the starting line number relative to content start
	ToLine   int    // ToLine is the ending line number relative to content start
}

type MDContent struct {
	Headings []*MDHeading // list of Headings and subheadings from content
	RawBody  string       // RawBody as is
}
