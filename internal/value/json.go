package value

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

// ParseJSON is JSON.parse: the text's one JSON value as this package holds
// values (a mapping in JavaScript's key order, a repeated key keeping its
// first place and its last value, every number a float64), or an error when
// the text is not one JSON value.
func ParseJSON(text string) (any, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.UseNumber()
	v, err := decodeJSON(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("not one JSON value")
	}
	return v, nil
}

func decodeJSON(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := t.(type) {
	case json.Delim:
		if t == '[' {
			list := []any{}
			for dec.More() {
				v, err := decodeJSON(dec)
				if err != nil {
					return nil, err
				}
				list = append(list, v)
			}
			_, err := dec.Token()
			return list, err
		}
		m := NewMap()
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := decodeJSON(dec)
			if err != nil {
				return nil, err
			}
			m.Set(k.(string), v)
		}
		_, err := dec.Token()
		return m, err
	case json.Number:
		// Out of range is ±Infinity, as JSON.parse reads 1e400.
		f, _ := strconv.ParseFloat(string(t), 64)
		return f, nil
	}
	return t, nil
}
