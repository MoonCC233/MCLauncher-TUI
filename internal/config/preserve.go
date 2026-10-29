package config

import (
	"encoding/json"
	"reflect"
	"strings"
)

// The launcher writes the whole config back on every change, so any key this
// build does not know about would be dropped. Both Config and Settings keep the
// unknown keys and write them out again, which keeps a newer launcher's
// preferences alive when the user goes back to an older build.

// preservedFields holds JSON keys that were present in the file but are not part
// of this build's schema.
type preservedFields map[string]json.RawMessage

// knownJSONFields lists the JSON names a struct type declares.
func knownJSONFields(v any) map[string]bool {
	known := map[string]bool{}
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return known
	}
	for i := 0; i < t.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		known[name] = true
	}
	return known
}

// unknownJSONFields returns the keys of data that known does not declare.
func unknownJSONFields(data []byte, known map[string]bool) preservedFields {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	for key := range raw {
		if known[key] {
			delete(raw, key)
		}
	}
	if len(raw) == 0 {
		return nil
	}
	return raw
}

// marshalWithPreserved marshals v and re-adds the preserved keys, without
// overwriting values that v itself provides. When nothing is preserved the
// output is the plain struct, so existing files keep their layout.
func marshalWithPreserved(v any, preserved *preservedFields) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil || preserved == nil || len(*preserved) == 0 {
		return data, err
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &fields); err != nil {
		return data, nil
	}
	for key, value := range *preserved {
		if _, ok := fields[key]; !ok {
			fields[key] = value
		}
	}
	return json.Marshal(fields)
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type plain Config
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = Config(decoded)
	c.preserved = keepUnknown(data, plain{})
	return nil
}

func (c *Config) MarshalJSON() ([]byte, error) {
	type plain Config
	return marshalWithPreserved((*plain)(c), c.preserved)
}

func (s *Settings) UnmarshalJSON(data []byte) error {
	type plain Settings
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = Settings(decoded)
	s.preserved = keepUnknown(data, plain{})
	return nil
}

func (s Settings) MarshalJSON() ([]byte, error) {
	type plain Settings
	return marshalWithPreserved(plain(s), s.preserved)
}

// keepUnknown wraps the unknown keys of data so the struct stays comparable.
func keepUnknown(data []byte, schema any) *preservedFields {
	fields := unknownJSONFields(data, knownJSONFields(schema))
	if fields == nil {
		return nil
	}
	return &fields
}
