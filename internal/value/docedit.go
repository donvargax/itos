package value

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The edits work add and work edit make (slice 54), beside Set and Lead: an
// item added to the end of a list, a list replaced, and a paragraph added
// to a text. Like those, each writes the few bytes it must, every comment,
// quote and line of the rest kept, and is made in Want, which Text holds the
// edited text to.

// Width is the column a folded text's lines are wrapped at, as the work
// registry's whys are written.
const Width = 100

// Wrap is text broken into lines of at most width characters, at spaces, as
// a commit body or a folded text is written; a word longer than width has a
// line to itself.
func Wrap(text string, width int) string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len([]rune(line))+1+len([]rune(word)) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// words is text's words joined by single spaces: what a folded block of
// them, wrapped, reads back as before its line break.
func words(text string) string { return strings.Join(strings.Fields(text), " ") }

// indented is text wrapped to end by Width, each line indented and ended by
// nl.
func indented(text, indent, nl string) string {
	var b strings.Builder
	for _, line := range strings.Split(Wrap(text, max(Width-len(indent), 40)), "\n") {
		b.WriteString(indent + line + nl)
	}
	return b.String()
}

// mapAt is the mapping node and Want's mapping that hold the path's last
// step, a key, and whether a flow collection holds it.
func (d *Doc) mapAt(path []any) (*yaml.Node, *Map, string, bool, error) {
	if len(path) == 0 {
		return nil, nil, "", false, fmt.Errorf("no path to edit")
	}
	parent, flow, holder, err := d.find(path[:len(path)-1])
	if err != nil {
		return nil, nil, "", false, err
	}
	key, ok := path[len(path)-1].(string)
	m, isMap := holder.(*Map)
	if !ok || parent.Kind != yaml.MappingNode || !isMap {
		return nil, nil, "", false, fmt.Errorf("%s is not a key of a mapping", where(path))
	}
	return parent, m, key, flow, nil
}

// Append adds item to the end of the block list the path names (no path:
// the document's top), as a block mapping of its keys in their order:
// written after the list's last item, its dash in that item's column, with a
// blank line before it when one parts the list's last two items. An empty
// flow list ([]) outside any flow collection, as itos init writes the
// registry's items and the stealth ledger (bug 12), becomes a block list of
// the item, as if it had always been there (appendFirst). How each value is
// written is BlockItem's.
func (d *Doc) Append(path []any, item *Map, folded ...string) error {
	list, _, want, err := d.find(path)
	if err != nil {
		return err
	}
	var holder *Map
	var keyNode *yaml.Node
	key, inFlow := "", false
	if len(path) > 0 {
		var parent *yaml.Node
		if parent, holder, key, inFlow, err = d.mapAt(path); err != nil {
			return err
		}
		keyNode, _ = lookup(parent, key)
	}
	items, isList := want.([]any)
	if list.Kind != yaml.SequenceNode || !isList {
		return fmt.Errorf("%s is not a list", where(path))
	}
	grow := func(wrote *Map) {
		grown := append(append([]any{}, items...), wrote)
		if holder == nil {
			d.Want = grown
		} else {
			holder.Set(key, grown)
		}
	}
	if len(list.Content) == 0 && list.Style&yaml.FlowStyle != 0 && !inFlow {
		wrote, err := d.appendFirst(list, keyNode, item, folded)
		if err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
		grow(wrote)
		return nil
	}
	if list.Style&yaml.FlowStyle != 0 || inFlow || len(list.Content) == 0 {
		return fmt.Errorf("%s is not a block list with an item, nor an empty one ([])", where(path))
	}
	last := list.Content[len(list.Content)-1]
	start, err := d.e.offset(last.Line, 1)
	if err != nil {
		return err
	}
	head := d.e.text[start:d.e.lineEnd(start)]
	dash := len(head) - len(strings.TrimLeft(head, " "))
	if !strings.HasPrefix(head[dash:], "-") {
		return fmt.Errorf("%s's last item does not start its line", where(path))
	}
	at, lead := d.e.afterLine(start)
	for next := at; next < len(d.e.text); {
		line := d.e.text[next:d.e.lineEnd(next)]
		after, more := d.e.afterLine(next)
		if strings.TrimSpace(line) != "" {
			if len(line)-len(strings.TrimLeft(line, " ")) <= dash {
				break
			}
			at, lead = after, more
		}
		next = after
	}
	put, wrote, err := blockItem(item, dash, d.e.nl, folded)
	if err != nil {
		return err
	}
	if len(list.Content) > 1 && d.partedByBlank(start) {
		put = d.e.nl + put
	}
	d.e.edits = append(d.e.edits, edit{at: at, put: lead + put})
	grow(wrote)
	return nil
}

// appendFirst writes item as the one item of an empty flow list ([]) on one
// line, in block context, and is the item as it reads back: the brackets
// are taken out, with the spaces before them, and the item is written as a
// block list below their line, its dash two columns in from its key's (none
// at the document's top); a line that held nothing but the brackets is
// replaced by the item. A comment after the brackets stays on its line.
func (d *Doc) appendFirst(list, key *yaml.Node, item *Map, folded []string) (*Map, error) {
	start, end, err := d.flowSpan(list)
	if err != nil {
		return nil, err
	}
	lineStart, err := d.e.offset(list.Line, 1)
	if err != nil {
		return nil, err
	}
	from := start
	for from > lineStart && (d.e.text[from-1] == ' ' || d.e.text[from-1] == '\t') {
		from--
	}
	dash := 0
	if key != nil {
		dash = key.Column - 1 + 2
	}
	put, wrote, err := blockItem(item, dash, d.e.nl, folded)
	if err != nil {
		return nil, err
	}
	after, lead := d.e.afterLine(end)
	if from == lineStart && strings.TrimSpace(d.e.text[end:d.e.lineEnd(end)]) == "" {
		d.e.edits = append(d.e.edits, edit{at: from, cut: after - from, put: put})
		return wrote, nil
	}
	d.e.edits = append(d.e.edits, edit{at: from, cut: end - from}, edit{at: after, put: lead + put})
	return wrote, nil
}

// BlockItem is item as the one item of a block list at the top of a
// document, as Append writes an item (a ledger file of one task, slice 55):
// a string plain where it reads back as itself, else double quoted, and a
// key named in folded as a folded block (>) of its words wrapped at Width,
// which reads back as them joined by spaces with a line break after; nil is
// null, a number plain, a list of strings a flow list on its key's line, and
// a list of mappings a block list below its key, each item's keys so
// written. The text ends with a line break and reads back as the list.
func BlockItem(item *Map, folded ...string) (string, error) {
	text, wrote, err := blockItem(item, 0, "\n", folded)
	if err != nil {
		return "", err
	}
	now, err := Parse(text)
	if err != nil {
		return "", fmt.Errorf("the item would not read back: %w", err)
	}
	if !same(now, []any{wrote}) {
		return "", fmt.Errorf("the item would read back as %s, not %s", JSON(now), JSON([]any{wrote}))
	}
	return text, nil
}

// blockItem is item as a block list's item whose dash is in the column
// dash, each line ended by nl, and the item as it reads back.
func blockItem(item *Map, dash int, nl string, folded []string) (string, *Map, error) {
	var b strings.Builder
	indent := strings.Repeat(" ", dash+2)
	wrote := NewMap()
	for i, k := range item.Keys() {
		prefix := indent
		if i == 0 {
			prefix = strings.Repeat(" ", dash) + "- "
		}
		v := item.At(k)
		var put string
		switch t := v.(type) {
		case nil:
			put = " null" + nl
		case string:
			if !slices.Contains(folded, k) {
				put = " " + token(t, 0) + nl
				break
			}
			if words(t) == "" {
				return "", nil, fmt.Errorf("%s has no words to fold", k)
			}
			put = " >" + nl + indented(t, indent+"  ", nl)
			v = words(t) + "\n"
		case float64:
			put = " " + Number(t) + nl
		case bool:
			put = fmt.Sprintf(" %t%s", t, nl)
		case []any:
			if maps, ok := mappings(t); ok {
				var nested strings.Builder
				read := make([]any, len(maps))
				for j, m := range maps {
					text, back, err := blockItem(m, dash+4, nl, nil)
					if err != nil {
						return "", nil, fmt.Errorf("%s: %w", k, err)
					}
					nested.WriteString(text)
					read[j] = back
				}
				put, v = nl+nested.String(), read
				break
			}
			flowList, err := flowStrings(t)
			if err != nil {
				return "", nil, fmt.Errorf("%s: %w", k, err)
			}
			put = " " + flowList + nl
			v = Copy(t)
		default:
			return "", nil, fmt.Errorf("%s is neither a text, a number, null nor a list", k)
		}
		b.WriteString(prefix + k + ":" + put)
		wrote.Set(k, v)
	}
	return b.String(), wrote, nil
}

// mappings is a list that is not empty and holds only mappings, as them.
func mappings(list []any) ([]*Map, bool) {
	maps := make([]*Map, len(list))
	for i, v := range list {
		m, ok := v.(*Map)
		if !ok {
			return nil, false
		}
		maps[i] = m
	}
	return maps, len(maps) > 0
}

// partedByBlank is whether a blank line comes between the item starting at
// start and the line before it that is no comment: the list's items are
// parted by blank lines.
func (d *Doc) partedByBlank(start int) bool {
	for end := start - 1; end > 0; {
		from := strings.LastIndexByte(d.e.text[:end], '\n') + 1
		line := strings.TrimSpace(d.e.text[from:end])
		switch {
		case line == "":
			return true
		case !strings.HasPrefix(line, "#"):
			return false
		}
		end = from - 1
	}
	return false
}

// flowStrings is a list of strings as a flow list, each plain where it reads
// back as itself there, else double quoted.
func flowStrings(list []any) (string, error) {
	parts := make([]string, len(list))
	for i, v := range list {
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("the list holds %s, not a text", JSON(v))
		}
		parts[i] = flowToken(s, 0, true)
	}
	return "[" + strings.Join(parts, ", ") + "]", nil
}

// SetList gives the key the path names the list of strings, written whole
// as one flow list (bug 14): over a value of nothing (null, ~ or none), added
// as a flow list when the mapping lacks the key (as Set adds one), and over a
// list of any shape, every byte of it replaced. A flow list on its key's line
// is replaced where it is; one that starts on a line below its key (the
// formatter wraps a long one so, over several lines) and a block list are
// taken out from just past the key's colon to their last line, and the flow
// list written on the key's line. Comments inside the old list go with it.
func (d *Doc) SetList(path []any, list []string) error {
	parent, m, key, flow, err := d.mapAt(path)
	if err != nil {
		return err
	}
	values := make([]any, len(list))
	for i, s := range list {
		values[i] = s
	}
	flowList, err := flowStrings(values)
	if err != nil {
		return err
	}
	k, v := lookup(parent, key)
	switch {
	case v == nil:
		if err := d.addLast(parent, flow, key, " "+flowList+d.e.nl, flowList); err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
	case isEmpty(v) && v.Kind == yaml.ScalarNode:
		start, end, err := d.e.span(v, flow)
		if err != nil {
			return err
		}
		put := flowList
		if start == end {
			put = " " + put
		}
		d.e.edits = append(d.e.edits, edit{at: start, cut: end - start, put: put})
	case v.Kind == yaml.SequenceNode:
		start, end, put := 0, 0, flowList
		if flow || v.Style&yaml.FlowStyle != 0 {
			if start, end, err = d.flowSpan(v); err != nil {
				return fmt.Errorf("%s: %w", where(path), err)
			}
		} else if end, err = d.blockEnd(v); err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
		if !flow && (v.Style&yaml.FlowStyle == 0 || v.Line != k.Line) {
			if start, err = d.afterColon(k); err != nil {
				return fmt.Errorf("%s: %w", where(path), err)
			}
			put = " " + flowList
		}
		d.e.edits = append(d.e.edits, edit{at: start, cut: end - start, put: put})
	default:
		return fmt.Errorf("%s is not a list", where(path))
	}
	m.Set(key, values)
	return nil
}

// afterColon is the offset just past the colon that ends a key of a block
// mapping, a scalar on one line.
func (d *Doc) afterColon(k *yaml.Node) (int, error) {
	var end int
	if k.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
		_, quoted, err := d.e.span(k, false)
		if err != nil {
			return 0, err
		}
		end = quoted
	} else {
		start, err := d.e.offset(k.Line, k.Column)
		if err != nil {
			return 0, err
		}
		end = start + len(k.Value)
		if end > len(d.e.text) || d.e.text[start:end] != k.Value {
			return 0, fmt.Errorf("the key on line %d is not on one line", k.Line)
		}
	}
	for end < len(d.e.text) && (d.e.text[end] == ' ' || d.e.text[end] == '\t') {
		end++
	}
	if end >= len(d.e.text) || d.e.text[end] != ':' {
		return 0, fmt.Errorf("the key on line %d has no colon after it", k.Line)
	}
	return end + 1, nil
}

// blockEnd is where a block list's text ends: its last item's line and every
// line after it indented past the items' dashes, up to the last such line
// that is not blank, before its line break.
func (d *Doc) blockEnd(v *yaml.Node) (int, error) {
	if len(v.Content) == 0 {
		return 0, fmt.Errorf("the list on line %d has no item", v.Line)
	}
	dash := v.Column - 1
	at, err := d.e.offset(v.Content[len(v.Content)-1].Line, 1)
	if err != nil {
		return 0, err
	}
	end := d.e.lineEnd(at)
	for next, _ := d.e.afterLine(at); next < len(d.e.text); next, _ = d.e.afterLine(next) {
		line := d.e.text[next:d.e.lineEnd(next)]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " ")) <= dash {
			break
		}
		end = d.e.lineEnd(next)
	}
	return end, nil
}

// addLast writes a key the mapping lacks after its last key's value: in a
// block mapping a line of its own, the key at its keys' column and block
// written after the colon (a line's rest, and the lines below it, ending
// with a line break), after every line the last value holds; in a flow one
// ", key: " and the token flowPut before the closing brace.
func (d *Doc) addLast(m *yaml.Node, flow bool, key, block, flowPut string) error {
	if len(m.Content) == 0 {
		return fmt.Errorf("an empty mapping is not added to")
	}
	if flow || m.Style&yaml.FlowStyle != 0 {
		_, end, err := d.flowSpan(m)
		if err != nil {
			return err
		}
		at := end - 1
		for at > 0 && (d.e.text[at-1] == ' ' || d.e.text[at-1] == '\t') {
			at--
		}
		d.e.edits = append(d.e.edits, edit{at: at, put: ", " + key + ": " + flowPut})
		return nil
	}
	last := m.Content[len(m.Content)-2]
	column := last.Column - 1
	start, err := d.e.offset(last.Line, last.Column)
	if err != nil {
		return err
	}
	at, lead := d.e.afterLine(start)
	for next := at; next < len(d.e.text); {
		line := d.e.text[next:d.e.lineEnd(next)]
		after, more := d.e.afterLine(next)
		if strings.TrimSpace(line) != "" {
			if len(line)-len(strings.TrimLeft(line, " ")) <= column {
				break
			}
			at, lead = after, more
		}
		next = after
	}
	d.e.edits = append(d.e.edits, edit{at: at, put: lead + strings.Repeat(" ", column) + key + ":" + block})
	return nil
}

// flowSpan is where a flow collection is in the text: its opening bracket
// and just past its closing one, on its line or on a line below (bug 14: the
// formatter wraps a long flow list over several lines). Quoted text and
// comments are passed over.
func (d *Doc) flowSpan(n *yaml.Node) (int, int, error) {
	start, err := d.e.offset(n.Line, n.Column)
	if err != nil {
		return 0, 0, err
	}
	text := d.e.text
	if start >= len(text) || (text[start] != '[' && text[start] != '{') {
		return 0, 0, fmt.Errorf("line %d does not open a flow collection", n.Line)
	}
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '"':
			for i++; i < len(text) && text[i] != '"'; i++ {
				if text[i] == '\\' {
					i++
				}
			}
		case '\'':
			for i++; i < len(text); i++ {
				if text[i] == '\'' {
					if i+1 < len(text) && text[i+1] == '\'' {
						i++
						continue
					}
					break
				}
			}
		case '#':
			if c := text[i-1]; c == ' ' || c == '\t' || c == '\n' {
				i = d.e.lineEnd(i)
			}
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				return start, i + 1, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("the flow collection on line %d is not closed", n.Line)
}

// Note adds a paragraph to the text the path names, a key of a mapping: a
// folded block (>) or a literal one (|) gains a blank line and the
// paragraph's words wrapped at Width after its last line, read back as its
// text, a line break and the paragraph; a one-line text in a block mapping
// becomes a folded block of its words, a blank line and the paragraph; one
// in a flow mapping gains a line break and the paragraph, double quoted; a
// key with no value, or none at all, is the paragraph. A block that keeps
// its trailing lines (|+ or >+) or gives its indentation is an error.
func (d *Doc) Note(path []any, paragraph string) error {
	parent, m, key, flow, err := d.mapAt(path)
	if err != nil {
		return err
	}
	note := words(paragraph)
	if note == "" {
		return fmt.Errorf("no words to note")
	}
	k, v := lookup(parent, key)
	old := m.At(key)
	if v == nil && !flow {
		indent := strings.Repeat(" ", parent.Content[len(parent.Content)-2].Column-1+2)
		if err := d.addLast(parent, false, key, " >"+d.e.nl+indented(note, indent, d.e.nl), ""); err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
		m.Set(key, note+"\n")
		return nil
	}
	if v == nil {
		if err := d.addLast(parent, true, key, "", Quote(note)); err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
		m.Set(key, note)
		return nil
	}
	if old == nil || old == "" {
		return d.Set(path, note)
	}
	text, isText := old.(string)
	if !isText || v.Kind != yaml.ScalarNode {
		return fmt.Errorf("%s is not a text", where(path))
	}
	if v.Anchor != "" || v.Style&yaml.TaggedStyle != 0 {
		return fmt.Errorf("%s has an anchor or a tag", where(path))
	}
	if v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 {
		if flow {
			return d.Set(path, text+"\n"+note)
		}
		start, end, err := d.e.span(v, false)
		if err != nil {
			return err
		}
		indent := strings.Repeat(" ", k.Column-1+2)
		at, lead := d.e.afterLine(end)
		d.e.edits = append(d.e.edits,
			edit{at: start, cut: end - start, put: ">"},
			edit{at: at, put: lead + indented(text, indent, d.e.nl) + d.e.nl + indented(note, indent, d.e.nl)})
		m.Set(key, words(text)+"\n"+note+"\n")
		return nil
	}
	at, err := d.e.offset(v.Line, v.Column)
	if err != nil {
		return err
	}
	header := strings.TrimSpace(d.e.text[at:d.e.lineEnd(at)])
	if i := strings.IndexByte(header, '#'); i >= 0 {
		header = strings.TrimSpace(header[:i])
	}
	if strings.ContainsAny(header, "123456789+") {
		return fmt.Errorf("%s keeps its trailing lines or gives its block's indentation", where(path))
	}
	strip := strings.Contains(header, "-")
	first, _ := d.e.afterLine(at)
	indent := ""
	end, lead := first, ""
	for next := first; next < len(d.e.text); {
		line := d.e.text[next:d.e.lineEnd(next)]
		after, more := d.e.afterLine(next)
		if strings.TrimSpace(line) != "" {
			lineIndent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			if indent == "" {
				indent = lineIndent
			}
			if indent == "" || len(lineIndent) < len(indent) {
				break
			}
			end, lead = after, more
		}
		next = after
	}
	if indent == "" {
		return fmt.Errorf("%s's block has no indented line", where(path))
	}
	d.e.edits = append(d.e.edits, edit{at: end, put: lead + d.e.nl + indented(note, indent, d.e.nl)})
	base := text
	if !strip {
		base = strings.TrimSuffix(text, "\n")
	}
	joined := base + "\n" + note
	if v.Style&yaml.LiteralStyle != 0 {
		joined = base + "\n\n" + Wrap(note, max(Width-len(indent), 40))
	}
	if !strip {
		joined += "\n"
	}
	m.Set(key, joined)
	return nil
}

// SetBlockList gives the key the path names, a key of a block mapping, the
// list of strings written as a block list, an item a line below the key, its
// dashes indented two past the key or in the old block list's column, or []
// on the key's line when the list is empty. A block list is the one shape a
// formatter leaves as it is however long it grows, which a flow list is not
// (the registry's queue, slice 66). An old value of nothing or a list of any
// shape is replaced whole, from just past the key's colon to its last line.
// A key the mapping lacks is added before the key before, above the comment
// lines that lead into it, a blank line after it when one is before it; or
// after the mapping's last key when it has no key before.
func (d *Doc) SetBlockList(path []any, list []string, before string) error {
	parent, m, key, flow, err := d.mapAt(path)
	if err != nil {
		return err
	}
	if flow || parent.Style&yaml.FlowStyle != 0 {
		return fmt.Errorf("%s is in a flow mapping", where(path))
	}
	values := make([]any, len(list))
	for i, s := range list {
		values[i] = s
	}
	block := func(indent string) string {
		if len(list) == 0 {
			return " []"
		}
		var b strings.Builder
		for _, s := range list {
			b.WriteString(d.e.nl + indent + "- " + token(s, 0))
		}
		return b.String()
	}
	k, v := lookup(parent, key)
	if k == nil {
		bk, _ := lookup(parent, before)
		if bk == nil {
			if err := d.addLast(parent, false, key, block("  ")+d.e.nl, ""); err != nil {
				return fmt.Errorf("%s: %w", where(path), err)
			}
			m.Set(key, values)
			return nil
		}
		at, err := d.e.offset(bk.Line, 1)
		if err != nil {
			return err
		}
		for at > 0 {
			start := strings.LastIndexByte(d.e.text[:at-1], '\n') + 1
			if !strings.HasPrefix(strings.TrimSpace(d.e.text[start:at]), "#") {
				break
			}
			at = start
		}
		lead := strings.Repeat(" ", bk.Column-1)
		put := lead + key + ":" + block(lead+"  ") + d.e.nl
		if at > 0 {
			start := strings.LastIndexByte(d.e.text[:at-1], '\n') + 1
			if strings.TrimSpace(d.e.text[start:at]) == "" {
				put += d.e.nl
			}
		}
		d.e.edits = append(d.e.edits, edit{at: at, put: put})
		m.Set(key, values)
		return nil
	}
	start, err := d.afterColon(k)
	if err != nil {
		return fmt.Errorf("%s: %w", where(path), err)
	}
	indent := strings.Repeat(" ", k.Column-1+2)
	end := 0
	switch {
	case isEmpty(v) && v.Kind == yaml.ScalarNode:
		if _, end, err = d.e.span(v, false); err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
	case v.Kind == yaml.SequenceNode && v.Style&yaml.FlowStyle != 0:
		if _, end, err = d.flowSpan(v); err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
	case v.Kind == yaml.SequenceNode:
		if end, err = d.blockEnd(v); err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
		indent = strings.Repeat(" ", v.Column-1)
	default:
		return fmt.Errorf("%s is not a list", where(path))
	}
	d.e.edits = append(d.e.edits, edit{at: start, cut: end - start, put: block(indent)})
	m.Set(key, values)
	return nil
}

// Add gives the mapping the path leads to the key its last step names, its
// value the string s, written as Set writes one: after the mapping's last
// key and every line its value holds, in a block mapping a line of its own
// at its keys' column, in a flow one before the closing brace. It is Set for
// a mapping whose keys may all hold collections (itos init --policy writes
// commits.since into a policy's commits, slice 108); a key the mapping has
// already is refused, and so is an empty mapping.
func (d *Doc) Add(path []any, s string) error {
	parent, m, key, flow, err := d.mapAt(path)
	if err != nil {
		return err
	}
	if k, _ := lookup(parent, key); k != nil {
		return fmt.Errorf("%s is there already", where(path))
	}
	if err := d.addLast(parent, flow, key, " "+token(s, 0)+d.e.nl, Quote(s)); err != nil {
		return fmt.Errorf("%s: %w", where(path), err)
	}
	m.Set(key, s)
	return nil
}

// Drop removes the key the path names from its mapping, with its value
// (slice 76: work done drops the why of the item it closes). In a block
// mapping the key's line goes, with every line below it that its value
// holds (those indented past the key, up to the last that is not blank), so
// a blank line parting it from what follows stays; the key must start its
// line. In a flow mapping its pair goes with the comma before it, or after
// it when it is the first, its value a one-line scalar or a flow
// collection. A key the mapping lacks is no edit.
func (d *Doc) Drop(path []any) error {
	parent, m, key, flow, err := d.mapAt(path)
	if err != nil {
		return err
	}
	k, v := lookup(parent, key)
	if k == nil {
		return nil
	}
	if k.Anchor != "" || v.Anchor != "" || v.Style&yaml.TaggedStyle != 0 {
		return fmt.Errorf("%s has an anchor or a tag", where(path))
	}
	start, err := d.e.offset(k.Line, k.Column)
	if err != nil {
		return err
	}
	if flow {
		err = d.dropPair(v, start)
	} else {
		err = d.dropLines(k, start)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", where(path), err)
	}
	m.Delete(key)
	return nil
}

// dropLines cuts a block mapping's key that starts at start, its line and
// the lines below it its value holds.
func (d *Doc) dropLines(k *yaml.Node, start int) error {
	from, err := d.e.offset(k.Line, 1)
	if err != nil {
		return err
	}
	if strings.TrimLeft(d.e.text[from:start], " ") != "" {
		return fmt.Errorf("the key does not start its line")
	}
	column := k.Column - 1
	end, _ := d.e.afterLine(start)
	for next := end; next < len(d.e.text); {
		line := d.e.text[next:d.e.lineEnd(next)]
		after, _ := d.e.afterLine(next)
		if strings.TrimSpace(line) != "" {
			if len(line)-len(strings.TrimLeft(line, " ")) <= column {
				break
			}
			end = after
		}
		next = after
	}
	d.e.edits = append(d.e.edits, edit{at: from, cut: end - from})
	return nil
}

// dropPair cuts a flow mapping's pair whose key starts at start, and the
// comma that parts it from the pair before it, or from the one after it
// when it is the first.
func (d *Doc) dropPair(v *yaml.Node, start int) error {
	var end int
	switch {
	case v.Kind == yaml.ScalarNode:
		_, e, err := d.e.span(v, true)
		if err != nil {
			return err
		}
		end = e
	case v.Style&yaml.FlowStyle != 0:
		_, e, err := d.flowSpan(v)
		if err != nil {
			return err
		}
		end = e
	default:
		return fmt.Errorf("the value on line %d is neither a one-line scalar nor a flow collection", v.Line)
	}
	text := d.e.text
	before := start
	for before > 0 && strings.IndexByte(" \t\r\n", text[before-1]) >= 0 {
		before--
	}
	if before > 0 && text[before-1] == ',' {
		d.e.edits = append(d.e.edits, edit{at: before - 1, cut: end - (before - 1)})
		return nil
	}
	after := end
	for after < len(text) && strings.IndexByte(" \t\r\n", text[after]) >= 0 {
		after++
	}
	if after < len(text) && text[after] == ',' {
		after++
		for after < len(text) && strings.IndexByte(" \t", text[after]) >= 0 {
			after++
		}
		end = after
	}
	d.e.edits = append(d.e.edits, edit{at: start, cut: end - start})
	return nil
}
