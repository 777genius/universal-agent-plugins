package skillsresolve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

type sourceFixture struct {
	discovery Discovery
	preview   Preview
	opened    bool
	closed    bool
	selected  string
}

func (f *sourceFixture) Open(context.Context, Request) (Session, string, string, error) {
	f.opened = true
	return f, "owner/repo", "0123456789012345678901234567890123456789", nil
}
func (f *sourceFixture) Discover(context.Context) (Discovery, error) { return f.discovery, nil }
func (f *sourceFixture) Preview(_ context.Context, path string) (Preview, error) {
	f.selected = path
	return f.preview, nil
}
func (f *sourceFixture) Close() error { f.closed = true; return nil }

func textPointer(s string) *string { return &s }

func TestCompleteDiscoveryAndExactSelection(t *testing.T) {
	for _, tc := range []struct {
		name, expected string
		complete       bool
		candidates     []Candidate
		hint           *string
	}{
		{"unique", "resolved", true, []Candidate{{Path: "actual/review", Name: "review"}}, textPointer("review")},
		{"ambiguous", "selection_required", true, []Candidate{{Path: "a", Name: "review"}, {Path: "b", Name: "review"}}, textPointer("review")},
		{"stale hint", "selection_required", true, []Candidate{{Path: "actual", Name: "new-name"}}, textPointer("old-name")},
		{"no candidates", "not_found", true, nil, nil},
		{"incomplete singleton", "incomplete", false, []Candidate{{Path: "actual", Name: "review"}}, textPointer("review")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &sourceFixture{discovery: Discovery{Complete: tc.complete, Candidates: tc.candidates}}
			r, err := New(f).Resolve(t.Context(), Request{Repository: "owner/repo", SkillNameHint: tc.hint})
			if err != nil || r.Status != tc.expected || !f.closed {
				t.Fatalf("unexpected result %+v, error %v, closed=%v", r, err, f.closed)
			}
		})
	}
}

func TestRawDigestAndExplicitRoot(t *testing.T) {
	raw := "---\r\nname: review\r\ndescription: Пример\r\n---\r\n# Text\r\n"
	f := &sourceFixture{preview: Preview{RawContent: raw, HasResources: true}}
	r, err := New(f).Resolve(t.Context(), Request{Repository: "owner/repo", Commit: textPointer("0123456789012345678901234567890123456789"), Path: textPointer("")})
	hash := sha256.Sum256([]byte(raw))
	if err != nil || r.Status != "resolved" || r.Skill.RawContent != raw || r.Skill.RawSHA256 != hex.EncodeToString(hash[:]) || !r.Skill.HasResources || f.selected != "" {
		t.Fatalf("source bytes or root selection changed: %+v %v", r, err)
	}
	for _, preview := range []Preview{{RawContent: "invalid\xff"}, {RawContent: raw, HasUnsupportedResources: true}} {
		f.preview = preview
		r, err = New(f).Resolve(t.Context(), Request{Repository: "owner/repo", Commit: textPointer(r.Commit), Path: textPointer("")})
		if err != nil || r.Status != "incomplete" || r.Skill != nil {
			t.Fatal("unsupported bytes/entry were eligible", r, err)
		}
	}
}

func TestInvalidRequestCannotOpenSource(t *testing.T) {
	for _, request := range []Request{
		{}, {Repository: "owner/repo", Path: textPointer("")},
		{Repository: "owner/repo", Commit: textPointer("branch"), Path: textPointer("")},
		{Repository: "owner/repo", Commit: textPointer("0123456789012345678901234567890123456789"), Path: textPointer("a\nb")},
		{Repository: "owner/repo", SkillNameHint: textPointer("bad\x00hint")},
	} {
		f := &sourceFixture{}
		if _, err := New(f).Resolve(t.Context(), request); !errors.Is(err, ErrInvalidRequest) || f.opened {
			t.Fatal("invalid request reached source", request, err)
		}
	}
}
