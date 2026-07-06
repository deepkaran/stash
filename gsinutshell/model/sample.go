package model

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Sample is a dynamically key-addressed view of one JSON stat payload.
//
// We deliberately avoid typed structs for stats. The node/index stats are
// owned by indexer/stats_manager.go and the storage stats are owned by plasma;
// both drift across releases. By key-addressing every payload with tolerant
// accessors, a missing/renamed key simply makes a rule report "n/a" instead of
// breaking the parser.
type Sample map[string]any

// ParseSample decodes a JSON object payload into a Sample. Numbers decode to
// float64 (encoding/json default); the typed getters coerce as needed.
func ParseSample(raw []byte) (Sample, error) {
	var s Sample
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return s, nil
}

// get resolves a possibly dotted path (e.g. "slice_0.BackStore.memory_size")
// against nested objects.
func (s Sample) get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}
	if !strings.Contains(key, ".") {
		v, ok := s[key]
		return v, ok
	}
	var cur any = map[string]any(s)
	for _, part := range strings.Split(key, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// Int returns an integer value, coercing from float64/json.Number/string.
func (s Sample) Int(key string) (int64, bool) {
	v, ok := s.get(key)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case json.Number:
		i, err := t.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return i, err == nil
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// Float returns a floating point value, coercing where possible.
func (s Sample) Float(key string) (float64, bool) {
	v, ok := s.get(key)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	}
	return 0, false
}

// Str returns a string value.
func (s Sample) Str(key string) (string, bool) {
	v, ok := s.get(key)
	if !ok {
		return "", false
	}
	if str, ok := v.(string); ok {
		return str, true
	}
	return "", false
}

// Bool returns a boolean value.
func (s Sample) Bool(key string) (bool, bool) {
	v, ok := s.get(key)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// Sub returns a nested object as a Sample.
func (s Sample) Sub(key string) (Sample, bool) {
	v, ok := s.get(key)
	if !ok {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return Sample(m), true
	}
	return nil, false
}
