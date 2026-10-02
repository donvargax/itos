package value

import "testing"

// JSON.parse's reading: JavaScript's key order, a repeated key keeping its
// first place and its last value, and one value or an error.
func TestParseJSON(t *testing.T) {
	v, err := ParseJSON(` {"b": 1, "2": [true, null, "x"], "a": {}, "b": 1e400} `)
	if err != nil {
		t.Fatal(err)
	}
	if got := JSON(v); got != `{"2":[true,null,"x"],"b":null,"a":{}}` {
		t.Errorf("JSON %s", got)
	}
	for _, text := range []string{"", "{} {}", "{}x", "{'a': 1}", "[1,]"} {
		if _, err := ParseJSON(text); err == nil {
			t.Errorf("%q parsed", text)
		}
	}
}
