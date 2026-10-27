package value

import (
	"errors"
	"testing"
)

func TestCheckWritten(t *testing.T) {
	for _, tc := range []struct {
		name, text, want string
	}{
		{"empty", "", ""},
		{"a null document", "~\n", ""},
		{"no null", "a:\n  - b\n  - c: 1\n", ""},
		{"a null value", "a: ~\nb:\n", ""},
		{"a quoted tilde", "\"~\": 1\nl: ['~', 'null']\n", ""},
		{"a null key", "a: 1\n~: 2\n", "line 2, column 1 has a null key, which no file itos writes has"},
		{"an empty key", "a: 1\n? \n: 2\n", "line 2, column 2 has a null key, which no file itos writes has"},
		{"a null key deep", "a:\n  - b:\n      null: 1\n", "line 3, column 7 has a null key, which no file itos writes has"},
		{"a null list item", "a:\n  -\n", "line 2, column 4 has a null list item, which no file itos writes has"},
		{"a null flow item", "a: [x, ~]\n", "line 1, column 8 has a null list item, which no file itos writes has"},
		{"a null item through an alias", "a: &n ~\nl: [x, *n]\n", "line 2, column 8 has a null list item, which no file itos writes has"},
		{"a null key under a null key", "~:\n  b: 1\n  b: 2\n", "Map keys must be unique at line 3, column 3"},
		{"a syntax error", "a: [\n", "yaml: line 1: did not find expected node content"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckWritten(tc.text)
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("CheckWritten(%q) = %v, want none", tc.text, err)
			case tc.want != "" && (err == nil || err.Error() != tc.want):
				t.Fatalf("CheckWritten(%q) = %v, want %q", tc.text, err, tc.want)
			}
		})
	}
}

func TestCheckWrittenSeveralDocuments(t *testing.T) {
	if err := CheckWritten("a: 1\n---\nb: 2\n"); !errors.Is(err, ErrMultipleDocuments) {
		t.Fatalf("CheckWritten of two documents = %v, want ErrMultipleDocuments", err)
	}
}
