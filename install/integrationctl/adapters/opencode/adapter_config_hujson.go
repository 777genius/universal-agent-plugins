package opencode

import (
	"encoding/json"

	"github.com/tailscale/hujson"
)

func setTopLevelMember(obj *hujson.Object, key string, value any) error {
	memberValue, err := valueToHuJSONValue(value)
	if err != nil {
		return err
	}
	for i := range obj.Members {
		name := obj.Members[i].Name.Value.(hujson.Literal).String()
		if name == key {
			memberValue.BeforeExtra = obj.Members[i].Value.BeforeExtra
			memberValue.AfterExtra = obj.Members[i].Value.AfterExtra
			obj.Members[i].Value = memberValue
			return nil
		}
	}
	nameValue := hujson.Value{Value: hujson.String(key)}
	memberValue.BeforeExtra = []byte("\n  ")
	memberValue.AfterExtra = []byte{}
	obj.Members = append(obj.Members, hujson.ObjectMember{Name: nameValue, Value: memberValue})
	return nil
}

func valueToHuJSONValue(value any) (hujson.Value, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return hujson.Value{}, err
	}
	parsed, err := hujson.Parse(body)
	if err != nil {
		return hujson.Value{}, err
	}
	return parsed, nil
}

func removeTopLevelMember(obj *hujson.Object, key string) {
	filtered := obj.Members[:0]
	for i := range obj.Members {
		name := obj.Members[i].Name.Value.(hujson.Literal).String()
		if name != key {
			filtered = append(filtered, obj.Members[i])
		}
	}
	obj.Members = filtered
}

func decodeConfigMap(body []byte) (map[string]any, error) {
	body, err := hujson.Standardize(body)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}
