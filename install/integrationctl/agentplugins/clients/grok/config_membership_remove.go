package grok

import (
	"bytes"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
)

type membershipRemoval struct {
	Body    []byte
	Offset  int
	Removed string
	Present bool
}

// prepareProjectRemoveBytes removes only the owned full native ID from the
// current enabled array. It does not restore an old document or touch disabled.
func prepareProjectRemoveBytes(original []byte, id string) (membershipRemoval, error) {
	if !membershipEncodedID(id) {
		return membershipRemoval{}, errGrokMembership
	}
	before, lists, err := membershipDocument(original)
	if err != nil {
		return membershipRemoval{}, err
	}
	selected := -1
	for index, value := range lists["enabled"] {
		if strings.EqualFold(value, id) && value != id {
			return membershipRemoval{}, errGrokMembership
		}
		if value == id {
			selected = index
		}
	}
	if selected < 0 {
		return membershipRemoval{Body: bytes.Clone(original)}, nil
	}
	start, end, err := ownedMemberRemovalSpan(original, id, selected, len(lists["enabled"]))
	if err != nil {
		return membershipRemoval{}, err
	}
	body := append(bytes.Clone(original[:start]), original[end:]...)
	after, observed, err := membershipDocument(body)
	want := slices.Delete(slices.Clone(lists["enabled"]), selected, selected+1)
	if err != nil || !slices.Equal(observed["enabled"], want) || !membershipForeignEqual(before, after) {
		return membershipRemoval{}, errGrokMembership
	}
	return membershipRemoval{Body: body, Offset: start, Removed: string(original[start:end]), Present: true}, nil
}

func ownedMemberRemovalSpan(body []byte, id string, selected, count int) (int, int, error) {
	var parser unstable.Parser
	parser.Reset(body)
	cursor := membershipTableCursor{}
	for parser.NextExpression() {
		expression := parser.Expression()
		if expression.Kind == unstable.Table || expression.Kind == unstable.ArrayTable {
			if err := cursor.visit(body, expression); err != nil {
				return 0, 0, err
			}
			continue
		}
		if !cursor.inside || expression.Kind != unstable.KeyValue {
			continue
		}
		enabled, err := membershipEnabledKey(expression)
		if err != nil {
			return 0, 0, err
		}
		if enabled {
			return selectedRemovalSpan(&parser, expression, id, selected, count)
		}
	}
	return 0, 0, errGrokMembership
}

func selectedRemovalSpan(parser *unstable.Parser, expression *unstable.Node, id string, selected, count int) (int, int, error) {
	closing, _, err := singleLineArrayInsertion(parser, expression, count)
	if err != nil {
		return 0, 0, err
	}
	spans := make([][2]int, 0, count)
	children := expression.Value().Children()
	for children.Next() {
		child := children.Node()
		spans = append(spans, [2]int{int(child.Raw.Offset), int(child.Raw.Offset + child.Raw.Length)})
	}
	if len(spans) != count || selected < 0 || selected >= count {
		return 0, 0, errGrokMembership
	}
	start, end := spans[selected][0], spans[selected][1]
	body := parser.Data()
	if token := string(body[start:end]); token != "\""+id+"\"" && token != "'"+id+"'" {
		return 0, 0, errGrokMembership
	}
	next := closing
	if selected < count-1 {
		next = spans[selected+1][0]
	}
	if comma := bytes.IndexByte(body[end:next], ','); comma >= 0 {
		return start, end + comma + 1, nil
	}
	if selected > 0 {
		previous := spans[selected-1][1]
		comma := bytes.IndexByte(body[previous:start], ',')
		if comma < 0 {
			return 0, 0, errGrokMembership
		}
		return previous + comma, end, nil
	}
	return start, end, nil
}
