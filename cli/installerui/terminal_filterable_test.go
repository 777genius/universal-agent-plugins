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
