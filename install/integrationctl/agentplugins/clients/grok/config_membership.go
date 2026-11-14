package grok

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

const grokMembershipLimit = 1 << 20

var errGrokMembership = errors.New("grok plugin membership predecessor is unsupported")

// prepareProjectEnableBytes is pure prospective preparation, not write authority.
// The caller must bind ID/name to actual native inventory and obtain consent.
// No caller executes this helper until lock/effects/recovery admission exists.
func prepareProjectEnableBytes(original []byte, pluginID, pluginName string) ([]byte, error) {
	if len(original) > grokMembershipLimit || !membershipText(pluginName) || !membershipEncodedID(pluginID) {
		return nil, errGrokMembership
	}
	before, lists, err := membershipDocument(original)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"enabled", "disabled"} {
		if membershipContains(lists[key], pluginID, pluginName) {
			return nil, errGrokMembership
		}
	}
	_, hasPlugins := before["plugins"]
	position, prefix, suffix, err := enabledInsertion(original, len(lists["enabled"]), hasPlugins)
	if err != nil {
		return nil, err
	}
	insertion := []byte(prefix + "\"" + pluginID + "\"" + suffix)
	if len(original)+len(insertion) > grokMembershipLimit {
		return nil, errGrokMembership
	}
	postimage := make([]byte, 0, len(original)+len(insertion))
	postimage = append(postimage, original[:position]...)
	postimage = append(postimage, insertion...)
	postimage = append(postimage, original[position:]...)
	after, observed, err := membershipDocument(postimage)
	if err != nil || !membershipAppended(lists["enabled"], observed["enabled"], pluginID) || !membershipForeignEqual(before, after) {
		return nil, errGrokMembership
	}
	return postimage, nil
}

func membershipText(value string) bool {
	return value != "" && len(value) <= 4096 && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}
func membershipEncodedID(value string) bool {
	if !membershipText(value) {
		return false
	}
	// This ASCII subset needs no escaping inside a TOML basic string.
	for _, ch := range value {
		if ch < 32 || ch > 126 || ch == '"' || ch == '\\' {
			return false
		}
	}
	return true
}
func membershipContains(values []string, candidates ...string) bool {
	for _, value := range values {
		for _, candidate := range candidates {
			if strings.EqualFold(value, candidate) {
				return true
			}
		}
	}
	return false
}
func membershipAliases(values map[string]any, names ...string) bool {
	for key := range values {
		for _, name := range names {
			if strings.EqualFold(key, name) && key != name {
				return true
			}
		}
	}
	return false
}
func membershipList(table map[string]any, key string) ([]string, error) {
	raw, present := table[key]
	if !present {
		return nil, nil
	}
	array, ok := raw.([]any)
	if !ok || len(array) > 128 {
		return nil, errGrokMembership
	}
	values := make([]string, 0, len(array))
	for _, raw := range array {
		value, ok := raw.(string)
		if !ok || !membershipText(value) || membershipContains(values, value) {
			return nil, errGrokMembership
		}
		values = append(values, value)
	}
	return values, nil
}
func membershipDocument(body []byte) (map[string]any, map[string][]string, error) {
	if len(body) > grokMembershipLimit {
		return nil, nil, errGrokMembership
	}
	values := make(map[string]any)
	if toml.Unmarshal(body, &values) != nil || membershipAliases(values, "plugins") {
		return nil, nil, errGrokMembership
	}
	table := map[string]any{}
	if raw, present := values["plugins"]; present {
		var ok bool
		table, ok = raw.(map[string]any)
		if !ok || membershipAliases(table, "enabled", "disabled") {
			return nil, nil, errGrokMembership
		}
	}
	lists := make(map[string][]string)
	for _, key := range []string{"enabled", "disabled"} {
		value, err := membershipList(table, key)
		if err != nil {
			return nil, nil, err
		}
		lists[key] = value
	}
	return values, lists, nil
}
func membershipAppended(before, after []string, pluginID string) bool {
	return len(after) == len(before)+1 && after[len(after)-1] == pluginID && slices.Equal(after[:len(before)], before)
}
func membershipForeignEqual(before, after map[string]any) bool {
	target := after["plugins"].(map[string]any) // The strict postparse proved this table.
	if raw, present := before["plugins"]; present {
		original := raw.(map[string]any)
		if enabled, exists := original["enabled"]; exists {
			target["enabled"] = enabled
		} else {
			delete(target, "enabled")
		}
	} else {
		delete(target, "enabled")
		if len(target) != 0 {
			return false
		}
		delete(after, "plugins")
	}
	return reflect.DeepEqual(before, after)
}
func membershipPluginTable(expression *unstable.Node) (bool, error) {
	keys := expression.Key()
	selected := keys.Next() && string(keys.Node().Data) == "plugins" && !keys.Next()
	if selected && expression.Kind != unstable.Table {
		return false, errGrokMembership
	}
	return selected, nil
}
func membershipEnabledKey(expression *unstable.Node) (bool, error) {
	keys := expression.Key()
	if !keys.Next() || string(keys.Node().Data) != "enabled" {
		return false, nil
	}
	if keys.Next() || expression.Value().Kind != unstable.Array {
		return false, errGrokMembership
	}
	return true, nil
}
func membershipTableStart(body []byte, expression *unstable.Node) (int, error) {
	keys := expression.Key()
	if !keys.Next() {
		return 0, errGrokMembership
	}
	// Table.Raw is empty in this pinned AST; its first key has the source range.
	offset := int(keys.Node().Raw.Offset)
	if offset > len(body) {
		return 0, errGrokMembership
	}
	return bytes.LastIndexByte(body[:offset], '\n') + 1, nil
}

type membershipTableCursor struct {
	inside, seen bool
	end          int
}

func (cursor *membershipTableCursor) visit(body []byte, expression *unstable.Node) error {
	if cursor.inside {
		end, err := membershipTableStart(body, expression)
		if err != nil {
			return err
		}
		cursor.end = end
	}
	selected, err := membershipPluginTable(expression)
	if err != nil || selected && cursor.seen {
		return errGrokMembership
	}
	cursor.inside = selected
	if selected {
		cursor.seen = true
		cursor.end = len(body)
	}
	return nil
}
func enabledInsertion(body []byte, count int, hasPlugins bool) (int, string, string, error) {
	var parser unstable.Parser
	parser.Reset(body)
	cursor := membershipTableCursor{end: len(body)}
	for parser.NextExpression() {
		expression := parser.Expression()
		if expression.Kind == unstable.Table || expression.Kind == unstable.ArrayTable {
			if err := cursor.visit(body, expression); err != nil {
				return 0, "", "", err
			}
			continue
		}
		if !cursor.inside || expression.Kind != unstable.KeyValue {
			continue
		}
		selected, err := membershipEnabledKey(expression)
		if err != nil {
			return 0, "", "", err
		}
		if selected {
			position, separator, err := singleLineArrayInsertion(&parser, expression, count)
			return position, separator, "", err
		}
	}
	if parser.Error() != nil || hasPlugins && !cursor.seen {
		return 0, "", "", errGrokMembership
	}
	return missingEnabledInsertion(body, cursor.end, cursor.seen)
}
func missingEnabledInsertion(body []byte, position int, tableSeen bool) (int, string, string, error) {
	newline := "\n"
	if bytes.Contains(body, []byte("\r\n")) {
		newline = "\r\n"
	}
	prefix := ""
	if position > 0 && body[position-1] != '\n' {
		prefix = newline
	}
	if !tableSeen {
		prefix += "[plugins]" + newline
	}
	return position, prefix + "enabled = [", "]" + newline, nil
}

func singleLineArrayInsertion(parser *unstable.Parser, expression *unstable.Node, count int) (int, string, error) {
	raw := parser.Raw(expression.Raw)
	equal := bytes.IndexByte(raw, '=')
	if equal < 0 || string(bytes.TrimSpace(raw[:equal])) != "enabled" {
		return 0, "", errGrokMembership
	}
	array := bytes.TrimSpace(raw[equal+1:])
	if len(array) < 2 || array[0] != '[' || array[len(array)-1] != ']' || bytes.ContainsAny(array, "\r\n") {
		return 0, "", errGrokMembership
	}
	position := int(expression.Raw.Offset) + int(expression.Raw.Length) - 1
	separator := ""
	children := expression.Value().Children()
	lastEnd, observed := 0, 0
	for children.Next() {
		child := children.Node()
		if child.Kind != unstable.String {
			return 0, "", errGrokMembership
		}
		lastEnd = int(child.Raw.Offset + child.Raw.Length)
		observed++
	}
	if observed != count {
		return 0, "", errGrokMembership
	}
	if count > 0 {
		tail := bytes.TrimSpace(parser.Data()[lastEnd:position])
		if len(tail) == 0 {
			separator = ", "
		} else if bytes.Equal(tail, []byte(",")) {
			separator = " "
		} else {
			return 0, "", errGrokMembership
		}
	}
	return position, separator, nil
}
