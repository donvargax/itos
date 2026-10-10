package value

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Renaming and removing a key (slice 113): itos upgrade applies to the config
// a person wrote the renames and removals each release's published
// itos.schema.json declares. Like the other edits, each changes only the
// bytes it must, every comment and other line kept, and is made in Want,
// which Text holds the edited text to. Each is made on the text as opened:
// a caller applying several in turn opens the edited text again between them.

// pathStep is one key of a path through block mappings: its key node and
// its value node.
type pathStep struct{ key, value *yaml.Node }

// Rename moves the value of the key the path from names to the key the path
// to names, and says whether from had one: a from that is not there is no
// edit. Within one mapping the key's name alone changes, where it is
// written. Into another mapping the key's lines go, with every line below it
// that its value holds, and are written after the last key of to's mapping,
// renamed and indented to their new place, the mappings to lacks added on the
// way, a line each. A mapping the move leaves empty goes too, with its key, up
// to the first that still holds something, so no section is left null.
// Refused, nothing edited: a path that passes through a flow collection, an
// anchor, an alias or a tag, a value holding one of the last three (a layout
// no edit of a few bytes keeps), a to that is set already, and two paths one
// of which holds the other.
func (d *Doc) Rename(from, to []string) (bool, error) {
	if len(from) == 0 || len(to) == 0 {
		return false, fmt.Errorf("no path to rename")
	}
	if holds(from, to) || holds(to, from) {
		return false, fmt.Errorf("%s and %s are one inside the other", dotted(from), dotted(to))
	}
	val, ok := wantAt(d.Want, from)
	if !ok {
		return false, nil
	}
	if _, set := wantAt(d.Want, to); set {
		return false, fmt.Errorf("%s is set already, beside %s", dotted(to), dotted(from))
	}
	chain, err := d.chain(from)
	if err != nil {
		return false, err
	}
	last := chain[len(chain)-1]
	common := 0
	for common < len(from)-1 && common < len(to)-1 && from[common] == to[common] {
		common++
	}
	if common == len(from)-1 && common == len(to)-1 {
		if err := d.renameKey(last.key, to[len(to)-1]); err != nil {
			return false, fmt.Errorf("%s: %w", dotted(from), err)
		}
		holder, _ := wantAt(d.Want, from[:len(from)-1])
		m := holder.(*Map)
		m.Delete(from[len(from)-1])
		m.Set(to[len(to)-1], val)
		return true, nil
	}
	moved, column, err := d.keyText(last.key)
	if err != nil {
		return false, fmt.Errorf("%s: %w", dotted(from), err)
	}
	drop := emptied(chain, common)
	cutFrom, cutEnd, err := d.keyLines(chain[drop].key)
	if err != nil {
		return false, fmt.Errorf("%s: %w", dotted(from), err)
	}
	// The insertion is recorded first: written where the cut starts or ends,
	// it must come after the cut there, and the editor makes the later of
	// two edits at one offset first.
	if err := d.insert(to, moved, column); err != nil {
		return false, err
	}
	d.e.edits = append(d.e.edits, edit{at: cutFrom, cut: cutEnd - cutFrom})
	holder, _ := wantAt(d.Want, from[:drop])
	holder.(*Map).Delete(from[drop])
	setWant(d.Want, to, val)
	return true, nil
}

// Remove deletes the key the path names, its line and every line below it
// that its value holds, and gives the value it held and whether it had one:
// a key that is not there is no edit. A mapping the removal leaves empty goes
// too, as Rename's does. Refused, nothing edited, as Rename refuses a layout.
func (d *Doc) Remove(path []string) (any, bool, error) {
	if len(path) == 0 {
		return nil, false, fmt.Errorf("no path to remove")
	}
	val, ok := wantAt(d.Want, path)
	if !ok {
		return nil, false, nil
	}
	chain, err := d.chain(path)
	if err != nil {
		return nil, false, err
	}
	drop := emptied(chain, 0)
	if err := d.cutKey(chain[drop].key); err != nil {
		return nil, false, fmt.Errorf("%s: %w", dotted(path), err)
	}
	holder, _ := wantAt(d.Want, path[:drop])
	holder.(*Map).Delete(path[drop])
	return val, true, nil
}

// chain is the key and value nodes of each key of the path, every mapping it
// passes through a block one, and the last value holding no anchor, alias or
// tag.
func (d *Doc) chain(path []string) ([]pathStep, error) {
	n := d.root
	steps := make([]pathStep, 0, len(path))
	for i, key := range path {
		if err := editable(n, path[:i]); err != nil {
			return nil, err
		}
		k, v := lookup(n, key)
		if k == nil {
			return nil, fmt.Errorf("%s is not written in its mapping", dotted(path[:i+1]))
		}
		if !plain(k) {
			return nil, fmt.Errorf("%s has an anchor, an alias or a tag", dotted(path[:i+1]))
		}
		steps = append(steps, pathStep{k, v})
		n = v
	}
	if !plain(n) {
		return nil, fmt.Errorf("%s has an anchor, an alias or a tag", dotted(path))
	}
	return steps, nil
}

// editable refuses a node a key of the path below it cannot be edited in: a
// flow collection, an anchor, an alias, a tag, or not a mapping.
func editable(n *yaml.Node, at []string) error {
	name := dotted(at)
	if len(at) == 0 {
		name = "the top"
	}
	switch {
	case n.Kind == yaml.AliasNode || n.Anchor != "" || n.Style&yaml.TaggedStyle != 0:
		return fmt.Errorf("%s has an anchor, an alias or a tag", name)
	case n.Kind != yaml.MappingNode:
		return fmt.Errorf("%s is not a mapping", name)
	case n.Style&yaml.FlowStyle != 0:
		return fmt.Errorf("%s is a flow mapping", name)
	}
	return nil
}

// plain is whether a node and everything in it has no anchor, alias or tag.
func plain(n *yaml.Node) bool {
	if n.Kind == yaml.AliasNode || n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 {
		return false
	}
	for _, c := range n.Content {
		if !plain(c) {
			return false
		}
	}
	return true
}

// emptied is the index in the chain of the key to cut so that no mapping is
// left empty: the last key's, or that of the highest mapping below the
// first floor keys whose value holds nothing but the chain's next key.
func emptied(chain []pathStep, floor int) int {
	drop := len(chain) - 1
	for i := len(chain) - 2; i >= floor && len(chain[i].value.Content) == 2; i-- {
		drop = i
	}
	return drop
}

// renameKey writes name over a key of a block mapping, in the key's style.
func (d *Doc) renameKey(k *yaml.Node, name string) error {
	start, end, err := d.keySpan(k)
	if err != nil {
		return err
	}
	d.e.edits = append(d.e.edits, edit{at: start, cut: end - start, put: token(name, k.Style)})
	return nil
}

// keySpan is where a key of a block mapping, a scalar on one line (the one
// kind of key lookup finds), is in the text.
func (d *Doc) keySpan(k *yaml.Node) (int, int, error) {
	if k.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
		return d.e.span(k, false)
	}
	start, err := d.e.offset(k.Line, k.Column)
	if err != nil {
		return 0, 0, err
	}
	end := start + len(k.Value)
	if end > len(d.e.text) || d.e.text[start:end] != k.Value {
		return 0, 0, fmt.Errorf("the key on line %d is not on one line", k.Line)
	}
	return start, end, nil
}

// keyText is what a key of a block mapping holds, as text: from just past
// its colon to the end of the last line its value holds, ending with a line
// break; and the key's column, from 0.
func (d *Doc) keyText(k *yaml.Node) (string, int, error) {
	colon, err := d.afterColon(k)
	if err != nil {
		return "", 0, err
	}
	_, end, err := d.keyLines(k)
	if err != nil {
		return "", 0, err
	}
	text := d.e.text[colon:end]
	if !strings.HasSuffix(text, "\n") {
		text += d.e.nl
	}
	return text, k.Column - 1, nil
}

// cutKey cuts a key of a block mapping: its line and the lines below it its
// value holds.
func (d *Doc) cutKey(k *yaml.Node) error {
	from, end, err := d.keyLines(k)
	if err != nil {
		return err
	}
	d.e.edits = append(d.e.edits, edit{at: from, cut: end - from})
	return nil
}

// insert writes the key the path names after the last key of the mapping
// the path's other keys lead to, its value the text held, as keyText gives
// it, written at column; mappings the path names but the text lacks are
// added on the way, a line each, two columns deeper than the one above.
func (d *Doc) insert(path []string, held string, column int) error {
	n, have := d.root, 0
	for have < len(path)-1 {
		if err := editable(n, path[:have]); err != nil {
			return err
		}
		k, v := lookup(n, path[have])
		if k == nil {
			break
		}
		n = v
		have++
	}
	if err := editable(n, path[:have]); err != nil {
		return err
	}
	at := n.Content[0].Column - 1
	block := ""
	for i := have + 1; i < len(path); i++ {
		block += d.e.nl + strings.Repeat(" ", at+2*(i-have)) + token(path[i], 0) + ":"
	}
	shifted := shift(held, at+2*(len(path)-1-have)-column)
	if err := d.addLast(n, false, token(path[have], 0), block+shifted, ""); err != nil {
		return fmt.Errorf("%s: %w", dotted(path), err)
	}
	return nil
}

// shift is text indented by delta columns more (fewer when negative), its
// first line, the rest of a key's, and its blank lines as they were. Every
// other line of a key's value is indented past the key by spaces (keyLines),
// so moving it left to a key's new column never cuts more than spaces; a line
// that is not is left as it is, and Text refuses what it reads back as.
func shift(text string, delta int) string {
	lines := strings.SplitAfter(text, "\n")
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
		case delta > 0:
			lines[i] = strings.Repeat(" ", delta) + line
		case strings.HasPrefix(line, strings.Repeat(" ", -delta)):
			lines[i] = line[-delta:]
		}
	}
	return strings.Join(lines, "")
}

// holds is whether the path outer holds inner, or is it.
func holds(outer, inner []string) bool {
	if len(outer) > len(inner) {
		return false
	}
	for i := range outer {
		if outer[i] != inner[i] {
			return false
		}
	}
	return true
}

// dotted is a path as the messages name it.
func dotted(path []string) string { return strings.Join(path, ".") }

// wantAt is the value at the path of mappings' keys, and whether there is
// one.
func wantAt(v any, path []string) (any, bool) {
	for _, key := range path {
		m, ok := v.(*Map)
		if !ok || !m.Has(key) {
			return nil, false
		}
		v = m.At(key)
	}
	return v, true
}

// setWant sets the value at the path, adding the mappings it lacks.
func setWant(v any, path []string, val any) {
	m := v.(*Map)
	for _, key := range path[:len(path)-1] {
		next, ok := m.At(key).(*Map)
		if !ok {
			next = NewMap()
			m.Set(key, next)
		}
		m = next
	}
	m.Set(path[len(path)-1], val)
}
