package value

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Doc is YAML text edited in place by paths into it, as SetScalars edits a
// section (edit.go), for an edit deeper in the document: an item of a list
// and a value of a list (the work registry, which itos work take and promote
// write, slice 52). Every edit changes the few bytes that hold the value,
// leaving every comment, quote and line as it was, and is made in Want too,
// the document as it should read after; Text refuses the edits unless the
// edited text reads back as Want.
type Doc struct {
	e    editor
	root *yaml.Node
	// Want is the document as the edits so far make it read: the text's
	// value when opened, each edit made in it.
	Want any
}

// OpenDoc is the YAML text ready to edit: one document, its top a block
// mapping, or a block list (a ledger file, which itos task add appends a
// task to, slice 55), or an empty list ([]), which Append makes a block
// list (bug 12: init writes the stealth ledger so).
func OpenDoc(text string) (*Doc, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("its top is neither a block mapping nor a block list")
	}
	top := doc.Content[0]
	emptyList := top.Kind == yaml.SequenceNode && len(top.Content) == 0
	if (top.Kind != yaml.MappingNode && top.Kind != yaml.SequenceNode) || (top.Style&yaml.FlowStyle != 0 && !emptyList) {
		return nil, fmt.Errorf("its top is neither a block mapping, a block list nor an empty list")
	}
	want, err := Parse(text)
	if err != nil {
		return nil, err
	}
	d := &Doc{e: editor{text: text, nl: "\n"}, root: doc.Content[0], Want: want}
	if strings.Contains(text, "\r\n") {
		d.e.nl = "\r\n"
	}
	return d, nil
}

// A step of a path is a mapping's key (a string) or a list's index (an int).

// find is the node the path leads to from the top, whether a flow collection
// holds it, and the value in Want it stands for.
func (d *Doc) find(path []any) (*yaml.Node, bool, any, error) {
	n, flow, want := d.root, false, d.Want
	for _, step := range path {
		if n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 {
			return nil, false, nil, fmt.Errorf("%s has an anchor or a tag", where(path))
		}
		flow = flow || n.Style&yaml.FlowStyle != 0
		switch s := step.(type) {
		case string:
			if n.Kind != yaml.MappingNode {
				return nil, false, nil, fmt.Errorf("%s: %s is not in a mapping", where(path), s)
			}
			if _, v := lookup(n, s); v != nil {
				n = v
			} else {
				return nil, false, nil, fmt.Errorf("%s: no %s", where(path), s)
			}
			want = Prop(want, s)
		case int:
			list, ok := want.([]any)
			if n.Kind != yaml.SequenceNode || !ok || s < 0 || s >= len(n.Content) || s >= len(list) {
				return nil, false, nil, fmt.Errorf("%s: no item %d", where(path), s)
			}
			n, want = n.Content[s], list[s]
		default:
			return nil, false, nil, fmt.Errorf("%s: a step is a key or an index", where(path))
		}
	}
	return n, flow || n.Style&yaml.FlowStyle != 0, want, nil
}

// where is a path as the errors name it.
func where(path []any) string {
	parts := make([]string, len(path))
	for i, step := range path {
		parts[i] = fmt.Sprint(step)
	}
	return strings.Join(parts, ".")
}

// Set gives the value the path names the string s: the last step a key of
// the mapping the rest leads to, its value replaced in the style it is
// written in, or added when the mapping lacks it (after the last of its keys
// whose value is a one-line scalar, a line of its own in a block mapping, ",
// key: value" in a flow one); or an index of a list, its one-line scalar
// replaced.
func (d *Doc) Set(path []any, s string) error {
	if len(path) == 0 {
		return fmt.Errorf("no path to set")
	}
	parent, flow, holder, err := d.find(path[:len(path)-1])
	if err != nil {
		return err
	}
	last := path[len(path)-1]
	if key, ok := last.(string); ok && parent.Kind == yaml.MappingNode {
		m, isMap := holder.(*Map)
		if !isMap {
			return fmt.Errorf("%s is not a mapping", where(path[:len(path)-1]))
		}
		if _, v := lookup(parent, key); v == nil {
			put := token(s, 0)
			if flow {
				put = Quote(s)
			}
			if err := d.addToken(parent, flow, key, put); err != nil {
				return fmt.Errorf("%s: %w", where(path), err)
			}
			m.Set(key, s)
			return nil
		}
		m.Set(key, s)
	} else if i, ok := last.(int); ok && parent.Kind == yaml.SequenceNode {
		list, isList := holder.([]any)
		if !isList || i < 0 || i >= len(list) {
			return fmt.Errorf("%s: no item %d", where(path), i)
		}
		list[i] = s
	}
	n, flow, _, err := d.find(path)
	if err != nil {
		return err
	}
	return d.replace(n, flow, s)
}

// replace writes s over a one-line scalar, in its style.
func (d *Doc) replace(n *yaml.Node, flow bool, s string) error {
	start, end, err := d.e.span(n, flow)
	if err != nil {
		return err
	}
	put := flowToken(s, n.Style, flow)
	if start == end {
		put = " " + put
	}
	d.e.edits = append(d.e.edits, edit{at: start, cut: end - start, put: put})
	return nil
}

// flowToken is token, quoting in a flow collection a string that holds one
// of its indicators, which would end a plain one there.
func flowToken(s string, old yaml.Style, flow bool) string {
	if flow && old&(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle) == 0 && strings.ContainsAny(s, ",[]{}") {
		return Quote(s)
	}
	return token(s, old)
}

// addToken writes a key the mapping lacks, its value the token put, after
// the last of its keys whose value is a one-line scalar on the key's line.
func (d *Doc) addToken(m *yaml.Node, flow bool, key, put string) error {
	for i := len(m.Content) - 2; i >= 0; i -= 2 {
		k, v := m.Content[i], m.Content[i+1]
		if v.Kind != yaml.ScalarNode || v.Line != k.Line {
			continue
		}
		_, end, err := d.e.span(v, flow)
		if err != nil {
			continue
		}
		if flow {
			d.e.edits = append(d.e.edits, edit{at: end, put: ", " + key + ": " + put})
			return nil
		}
		at, lead := d.e.afterLine(end)
		indent := strings.Repeat(" ", k.Column-1)
		d.e.edits = append(d.e.edits, edit{at: at, put: lead + indent + key + ": " + put + d.e.nl})
		return nil
	}
	return fmt.Errorf("no one-line value to write %s after", key)
}

// Lead puts a sentence before the text the path names: a key of a mapping
// whose value is a string, or nothing. A one-line value becomes the sentence,
// a space and the text it was; a folded block (>) gains the sentence as its
// first line, which folds into the same; a literal one (|) gains it as a
// line of its own; a key with no value, or none at all, is the sentence.
func (d *Doc) Lead(path []any, sentence string) error {
	if len(path) == 0 {
		return fmt.Errorf("no path to lead")
	}
	parent, _, holder, err := d.find(path[:len(path)-1])
	if err != nil {
		return err
	}
	key, ok := path[len(path)-1].(string)
	m, isMap := holder.(*Map)
	if !ok || parent.Kind != yaml.MappingNode || !isMap {
		return fmt.Errorf("%s is not a key of a mapping", where(path))
	}
	_, v := lookup(parent, key)
	old := m.At(key)
	if v == nil || old == nil || old == "" {
		return d.Set(path, sentence)
	}
	text, isText := old.(string)
	if !isText {
		return fmt.Errorf("%s is not a text", where(path))
	}
	if v.Kind != yaml.ScalarNode || v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 {
		return d.Set(path, sentence+" "+text)
	}
	if v.Anchor != "" || v.Style&yaml.TaggedStyle != 0 {
		return fmt.Errorf("%s has an anchor or a tag", where(path))
	}
	at, err := d.e.offset(v.Line, v.Column)
	if err != nil {
		return err
	}
	header := strings.TrimSpace(d.e.text[at:d.e.lineEnd(at)])
	if i := strings.IndexByte(header, '#'); i >= 0 {
		header = strings.TrimSpace(header[:i])
	}
	if strings.ContainsAny(header, "123456789") {
		return fmt.Errorf("%s gives its block's indentation", where(path))
	}
	first, _ := d.e.afterLine(at)
	rest := d.e.text[first:]
	indent := ""
	for _, line := range strings.Split(rest, "\n") {
		if strings.TrimSpace(line) != "" {
			indent = line[:len(line)-len(strings.TrimLeft(line, " "))]
			break
		}
	}
	if indent == "" {
		return fmt.Errorf("%s's block has no indented line", where(path))
	}
	d.e.edits = append(d.e.edits, edit{at: first, put: indent + sentence + d.e.nl})
	joined := sentence + " " + text
	if v.Style&yaml.LiteralStyle != 0 {
		joined = sentence + "\n" + text
	}
	m.Set(key, joined)
	return nil
}

// Text is the edited text: the edits made, every other byte as it was,
// refused unless it reads back as Want.
func (d *Doc) Text() (string, error) {
	edited := d.e.apply()
	now, err := Parse(edited)
	if err != nil {
		return "", fmt.Errorf("the edit would not read back: %w", err)
	}
	if !same(now, d.Want) {
		return "", fmt.Errorf("the edit would read back as %s, not %s", JSON(now), JSON(d.Want))
	}
	return edited, nil
}

// same is whether two values are equal, a mapping's keys in any order.
func same(a, b any) bool {
	switch x := a.(type) {
	case *Map:
		y, ok := b.(*Map)
		if !ok || x.Len() != y.Len() {
			return false
		}
		keys := x.Keys()
		other := y.Keys()
		slices.Sort(keys)
		slices.Sort(other)
		if !slices.Equal(keys, other) {
			return false
		}
		for _, k := range keys {
			if !same(x.At(k), y.At(k)) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !same(x[i], y[i]) {
				return false
			}
		}
		return true
	}
	return JSON(a) == JSON(b) && TypeOf(a) == TypeOf(b)
}
