package model

import (
	"encoding/json"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

const Resource = "mapping"

var ErrMappingNotFound = apperrors.NotFound("Mapping not found.")

var ErrTestNotFound = apperrors.NotFound("Mapping test not found.")

func RawObject(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func RawAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func JsonArg(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage("null")
	}
	raw, _ := json.Marshal(v)
	return raw
}

func NullableRaw(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func TrimName(name string) string {
	for len(name) > 0 && (name[0] == ' ' || name[0] == '\t' || name[0] == '\n') {
		name = name[1:]
	}
	for len(name) > 0 && (name[len(name)-1] == ' ' || name[len(name)-1] == '\t' || name[len(name)-1] == '\n') {
		name = name[:len(name)-1]
	}
	return name
}

func JsonEqual(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	var av, bv any
	_ = json.Unmarshal(ab, &av)
	_ = json.Unmarshal(bb, &bv)
	return reflectJSONEqual(av, bv)
}

func reflectJSONEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	var av, bv any
	_ = json.Unmarshal(ab, &av)
	_ = json.Unmarshal(bb, &bv)
	return string(mustJSON(av)) == string(mustJSON(bv))
}

func mustJSON(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}
