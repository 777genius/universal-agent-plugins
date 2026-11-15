package installerui

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFilterableCatalogBoundsDoNotExpandAgentMenus(t *testing.T) {
	req := MultiSelectRequest{SelectRequest: SelectRequest{Title: "Skills"}, Filterable: true, MinSelected: 1}
	for i := range 2000 {
		req.Options = append(req.Options, Option{ID: fmt.Sprintf("skill-%d", i), Label: strings.Repeat("x", 128)})
	}
	if _, err := normalizeMulti(req); err != nil {
		t.Fatalf("bounded searchable catalog rejected: %v", err)
	}
	req.Filterable = false
	if _, err := normalizeMulti(req); err == nil {
		t.Fatal("ordinary agent menus unexpectedly exceed their existing limit")
	}
	req.Filterable = true
	req.Options = append(req.Options, Option{ID: "overflow", Label: "Overflow"})
	if _, err := normalizeMulti(req); err == nil {
		t.Fatal("unbounded searchable catalog accepted")
	}
}

func TestFilterableSelectionPreservesBasketAndQueuedNextAnswer(t *testing.T) {
	input := strings.NewReader(" /Beta\r \rnext-answer\n")
	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req := MultiSelectRequest{SelectRequest: SelectRequest{Title: "Choose Skills", Options: []Option{{ID: "alpha", Label: "Alpha"}, {ID: "beta", Label: "Beta"}}}, MinSelected: 1, Filterable: true}
	selected, err := (testRich{Input: input, Output: &output, NoColor: true}).SelectMany(ctx, req)
	if err != nil || !reflect.DeepEqual(selected.IDs, []string{"alpha", "beta"}) {
		t.Fatalf("search lost hidden selections or query Enter blocked input: %+v %v", selected, err)
	}
	rest, err := io.ReadAll(input)
	if err != nil || string(rest) != "next-answer\n" {
		t.Fatalf("search consumed the next prompt answer: %q %v", rest, err)
	}
	if output.Len() == 0 {
		t.Fatal("selection succeeded without displaying a question")
	}
}

func TestFilterableSelectionKeepsClippedLabelsIndependent(t *testing.T) {
	prefix := "review · " + strings.Repeat("shared-directory/", 40)
	req := MultiSelectRequest{
		SelectRequest: SelectRequest{Title: "Choose Skills", Options: []Option{
			{ID: "first", Label: prefix + "one/review"},
			{ID: "second", Label: prefix + "two/review"},
		}},
		MinSelected: 1,
		Filterable:  true,
	}
	for _, tt := range []struct {
		name, input, want string
	}{
		{name: "first", input: " \r", want: "first"},
		{name: "second", input: "\x1b[B \r", want: "second"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			selected, err := (testRich{Input: strings.NewReader(tt.input), Output: &output, NoColor: true}).SelectMany(ctx, req)
			if err != nil || !reflect.DeepEqual(selected.IDs, []string{tt.want}) {
				t.Fatalf("clipped labels changed another selection: %+v %v", selected, err)
			}
		})
	}
}

func TestFilterableSelectionRecoversFromEmptyFilterNavigation(t *testing.T) {
	req := MultiSelectRequest{SelectRequest: SelectRequest{Title: "Choose Skills", Options: []Option{
		{ID: "alpha", Label: "Alpha"}, {ID: "beta", Label: "Beta"},
	}}, MinSelected: 1, Filterable: true}
	for _, tt := range []struct {
		name, input, want string
	}{
		{name: "apply empty query", input: "/missing\x1b[B\r \r", want: "alpha"},
		{name: "replace empty query", input: "/missing\x1b[B" + strings.Repeat("\x7f", 7) + "Beta\r \r", want: "beta"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.input + "next-answer\n")
			var output strings.Builder
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			selected, err := (testRich{Input: input, Output: &output, NoColor: true}).SelectMany(ctx, req)
			if err != nil || !reflect.DeepEqual(selected.IDs, []string{tt.want}) {
				t.Fatalf("empty filter navigation lost the selection: %+v %v", selected, err)
			}
			rest, err := io.ReadAll(input)
			if err != nil || string(rest) != "next-answer\n" {
				t.Fatalf("empty filter navigation consumed the next answer: %q %v", rest, err)
			}
		})
	}
}
