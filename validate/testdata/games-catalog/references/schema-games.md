---
config:
  overview-heading: "## Overview"
  tag-list-heading: "## Tag List"
fields:
  game: {required: true, shape: game}
  knowledge-key-id: {required: false, shape: string}
  versions: {required: true, shape: versions}
  tags: {required: true, shape: list, from: tag-vocabulary}
  availability: {required: true, shape: storemap, from: controlled-stores}
  publisher: {required: true, shape: list}
  developer: {required: true, shape: list}
  game-features: {required: true, shape: list, from: controlled-features}
  dei-level: {required: true, shape: single, from: controlled-dei-level}
  controller-support: {required: false, shape: list, from: controlled-controller-support}
  other-features: {required: false, shape: list, from: controlled-other-features}
  anti-cheat: {required: false, shape: anticheat}
  dlcs: {required: true, shape: dlcs, from: controlled-dlcs-kind}
---

# Schema

Trimmed test fixture: the frontmatter is verbatim; the body is not part of the fixture.
