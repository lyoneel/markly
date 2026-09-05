package markly

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestDataReturnsUnderlyingMap(t *testing.T) {
	md := NewMDFileFromString("---\ntitle: T\ncount: 3\n---\n\nbody\n")
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	data := meta.Data()
	if data == nil {
		t.Fatal("Data() returned nil")
	}
	if data["title"] != "T" {
		t.Errorf("title = %v, want T", data["title"])
	}
	if len(data) != 2 {
		t.Errorf("data has %d keys, want 2", len(data))
	}
}

func TestWithRawScalarsKeepsTimestampsAsText(t *testing.T) {
	content := "---\ncreated: 2026-01-02\nupdated_at: 2026-01-02T15:04:05+01:00\ncount: 5\nactive: true\n---\n\nbody\n"

	md := NewMDFileFromString(content, WithRawScalars())
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if got := meta.Get("created"); got != "2026-01-02" {
		t.Errorf("created = %v (%T), want raw string 2026-01-02", got, got)
	}
	if got := meta.Get("updated_at"); got != "2026-01-02T15:04:05+01:00" {
		t.Errorf("updated_at = %v (%T), want raw original text", got, got)
	}
	// Natural types still apply for bool/int/float.
	if got := meta.Get("count"); got != int64(5) {
		t.Errorf("count = %v (%T), want int64(5)", got, got)
	}
	if got := meta.Get("active"); got != true {
		t.Errorf("active = %v (%T), want bool true", got, got)
	}
}

func TestDefaultModeTypesTimestamps(t *testing.T) {
	content := "---\ncreated: 2026-01-02\n---\n\nbody\n"
	md := NewMDFileFromString(content)
	meta, err := md.GetMetadata()
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	// In default mode yaml.v3 resolves a bare date to a time.Time, so the
	// raw string is NOT preserved. We only assert it is not the raw string.
	if got := meta.Get("created"); got == "2026-01-02" {
		t.Errorf("default mode unexpectedly kept created as raw string")
	}
}

func TestIsFlowSequence(t *testing.T) {
	md := NewMDFileFromString("---\ninline: [a, b]\nblock:\n  - c\n  - d\nplain: text\n---\n\nbody\n")
	meta, _ := md.GetMetadata()

	if !meta.IsFlowSequence("inline") {
		t.Error("inline should be a flow sequence")
	}
	if meta.IsFlowSequence("block") {
		t.Error("block should not be a flow sequence")
	}
	if meta.IsFlowSequence("plain") {
		t.Error("plain scalar is not a sequence")
	}
	if meta.IsFlowSequence("missing") {
		t.Error("missing key is not a flow sequence")
	}
}

func TestHasBlockScalarSequence(t *testing.T) {
	blockOnly := NewMDFileFromString("---\ntags:\n  - a\n  - b\n---\n\nbody\n")
	meta, _ := blockOnly.GetMetadata()
	if !meta.HasBlockScalarSequence() {
		t.Error("block scalar sequence not detected")
	}

	inlineOnly := NewMDFileFromString("---\ntags: [a, b]\n---\n\nbody\n")
	meta2, _ := inlineOnly.GetMetadata()
	if meta2.HasBlockScalarSequence() {
		t.Error("inline sequence wrongly reported as block scalar sequence")
	}

	blockOfMaps := NewMDFileFromString("---\nsources:\n  - name: x\n    url: y\n---\n\nbody\n")
	meta3, _ := blockOfMaps.GetMetadata()
	if meta3.HasBlockScalarSequence() {
		t.Error("block sequence of mappings wrongly reported as scalar sequence")
	}

	noSeq := NewMDFileFromString("---\ntitle: t\n---\n\nbody\n")
	meta4, _ := noSeq.GetMetadata()
	if meta4.HasBlockScalarSequence() {
		t.Error("no sequence present but block scalar sequence reported")
	}
}

func TestSetSerializerCustom(t *testing.T) {
	md := NewMDFileFromString("---\nb: 2\na: 1\n---\n\nbody\n")
	if _, err := md.GetContent(); err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	md.SetSerializer(func(meta map[string]any) (string, error) {
		keys := make([]string, 0, len(meta))
		for k := range meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			b.WriteString(k)
			b.WriteString(": ")
			fmt.Fprintf(&b, "%v", meta[k])
			b.WriteString("\n")
		}
		return b.String(), nil
	})

	out := md.Serialize()
	// Sorted keys: a before b.
	aIdx := strings.Index(out, "a: 1")
	bIdx := strings.Index(out, "b: 2")
	if aIdx < 0 || bIdx < 0 || aIdx > bIdx {
		t.Errorf("custom serializer did not order keys; output:\n%s", out)
	}
	if !strings.Contains(out, "body") {
		t.Error("body missing from serialized output")
	}
}

func TestDefaultSerializerUnchangedWhenUnset(t *testing.T) {
	md := NewMDFileFromString("---\ntitle: Keep\n---\n\nbody\n")
	out := md.Serialize()
	if !strings.Contains(out, "title: Keep") {
		t.Errorf("default serializer output unexpected:\n%s", out)
	}
}
