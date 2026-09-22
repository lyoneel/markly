---
config:
  sections: ["## Question", "## Findings", "## Confidence"]
fields:
  date: {required: true, shape: date}
  updated: {required: true, shape: date}
  type: {required: true, shape: single, values: [thread]}
  status: {required: true, shape: single, values: [draft, review, ready, active, completed, done, archived, deprecated, research-complete]}
  keywords: {required: true, shape: list}
  description: {required: true, shape: string}
  sources: {required: true, shape: objects, keys: [name, url, title]}
  conclusions: {required: true, shape: list}
  related: {required: false, shape: list}
  tools: {required: false, shape: list}
  hardware: {required: false, shape: list}
  games: {required: false, shape: list}
  hosts: {required: false, shape: list}
  notes: {required: false, shape: string}
  region: {required: false, shape: string}
  domain: {required: false, shape: string}
  session: {required: false, shape: object, keys: [id, uuid, title, data_dir]}
---

# Threads Schema

Trimmed test fixture: the frontmatter is verbatim; the body is not part of the fixture.
