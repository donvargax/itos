package out

import (
	"strings"
	"testing"
)

func TestEmitKeepsTheOrderAndSchemaFirst(t *testing.T) {
	var b strings.Builder
	err := Emit(&b,
		Field{"version", "1.0.0"},
		Field{"requires", ">=0.1.0 <2 & more"},
		Field{"problems", []Problem{{Rule: "r", Message: "m"}}},
		Field{"none", []string{}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "schema": 1,
  "version": "1.0.0",
  "requires": ">=0.1.0 <2 & more",
  "problems": [
    {
      "rule": "r",
      "message": "m"
    }
  ],
  "none": []
}
`
	if b.String() != want {
		t.Errorf("Emit wrote\n%s\nwant\n%s", b.String(), want)
	}
}
