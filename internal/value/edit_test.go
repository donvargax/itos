package value

import "testing"

func TestSetScalars(t *testing.T) {
	sum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	digits := "0123456789012345678901234567890123456789012345678901234567890123"
	pin := [][2]string{{"version", "9.2.0"}, {"checksums", sum}}
	cases := []struct{ name, text, want string }{
		{"a block pin, its comments kept",
			"version: 1 # one\n# the pin\npin:\n  version: \"9.1.0\" # was\n  checksums: 'aa'\nci: x\n",
			"version: 1 # one\n# the pin\npin:\n  version: \"9.2.0\" # was\n  checksums: '" + sum + "'\nci: x\n"},
		{"a flow pin",
			"version: 1\npin: { version: \"9.1.0\", checksums: \"aa\" }\nci: x\n",
			"version: 1\npin: { version: \"9.2.0\", checksums: \"" + sum + "\" }\nci: x\n"},
		{"plain values stay plain",
			"pin:\n  version: 9.1.0\n  checksums: aa\n",
			"pin:\n  version: 9.2.0\n  checksums: " + sum + "\n"},
		{"no pin: after the version line",
			"version: 1\n# ledger\nledger: {}\n",
			"version: 1\npin:\n  version: 9.2.0\n  checksums: " + sum + "\n# ledger\nledger: {}\n"},
		{"no pin and no version: at the end, a line break first",
			"ledger: {}",
			"ledger: {}\npin:\n  version: 9.2.0\n  checksums: " + sum + "\n"},
		{"an empty pin",
			"version: 1\npin:\nci: x\n",
			"version: 1\npin:\n  version: 9.2.0\n  checksums: " + sum + "\nci: x\n"},
		{"a pin of {}, its comment kept",
			"pin: {} # later\nci: x\n",
			"pin:  # later\n  version: 9.2.0\n  checksums: " + sum + "\nci: x\n"},
		{"a pin missing its checksums, in a block",
			"pin:\n    version: \"9.1.0\"\nci: x\n",
			"pin:\n    version: \"9.2.0\"\n    checksums: " + sum + "\nci: x\n"},
		{"a pin missing its checksums, in a flow",
			"pin: { version: \"9.1.0\" }\n",
			"pin: { version: \"9.2.0\", checksums: \"" + sum + "\" }\n"},
		{"a key with no value",
			"pin:\n  version:\n  checksums: aa\n",
			"pin:\n  version: 9.2.0\n  checksums: " + sum + "\n"},
		{"line breaks of CRLF",
			"version: 1\r\nci: x\r\n",
			"version: 1\r\npin:\r\n  version: 9.2.0\r\n  checksums: " + sum + "\r\nci: x\r\n"},
	}
	for _, c := range cases {
		got, err := SetScalars(c.text, "pin", pin)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
	got, err := SetScalars("pin:\n  checksums: aa\n", "pin", [][2]string{{"checksums", digits}})
	if want := "pin:\n  checksums: \"" + digits + "\"\n"; err != nil || got != want {
		t.Errorf("a checksum of digits alone is quoted, to read as a string: %q, %v", got, err)
	}
	for name, text := range map[string]string{
		"a list":            "pin: [a]\n",
		"a value on lines":  "pin:\n  version: \"9.1\n    .0\"\n",
		"a flow document":   "{pin: {version: 9.1.0}}\n",
		"a tagged value":    "pin:\n  version: !!str 9.1.0\n",
		"an alias":          "x: &a 9.1.0\npin:\n  version: *a\n",
		"a block value":     "pin:\n  version: |\n    9.1.0\n",
		"not YAML":          "pin: [\n",
		"another key there": "pin:\n  version: 9.1.0\n  extra: { a: [1,\n    2] }\n",
	} {
		if got, err := SetScalars(text, "pin", pin); err == nil {
			t.Errorf("%s: edited to %q", name, got)
		}
	}
}
