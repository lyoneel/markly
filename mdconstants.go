package markly

// YAML delimiter for frontmatter
const yamlDelimiter = "---"

// TOML delimiter for frontmatter
const tomlDelimiter = "+++"

// Newline character for joining lines
const newlineChar = "\n"

// Carriage return character (for line trimming)
const carriageReturn = "\r"

// Whitespace characters used in heading detection and trimming
const whitespaceChars = " \t"

// Heading marker character
const headingMarker = "#"

// Maximum ATX heading level (1-6 per Markdown spec)
const maxHeadingLevel = 6
