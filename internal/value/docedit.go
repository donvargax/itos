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

// Append adds item to the end of the block list the path names, as a block
// mapping of its keys in their order: written after the list's last item,
// its dash in that item's column, with a blank line before it when one
// parts the list's last two items. A string is written plain where it reads
// back as itself, else double quoted, and a key named in folded as a folded
// block (>) of its words wrapped at Width, which reads back as them joined
// by spaces with a line break after; nil is null, a number plain, a list of
// strings a flow list on its key's line.
func (d *Doc) Append(path []any, item *Map, folded ...string) error {
	list, flow, want, err := d.find(path)
	if err != nil {
		return err
	}
	_, holder, key, _, err := d.mapAt(path)
	if err != nil {
		return err
	}
	items, isList := want.([]any)
	if list.Kind != yaml.SequenceNode || flow || !isList || len(list.Content) == 0 {
		return fmt.Errorf("%s is not a block list with an item", where(path))
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
	var b strings.Builder
	b.WriteString(lead)
	if len(list.Content) > 1 && d.partedByBlank(start) {
		b.WriteString(d.e.nl)
	}
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
			put = " null" + d.e.nl
		case string:
			if !slices.Contains(folded, k) {
				put = " " + token(t, 0) + d.e.nl
				break
			}
			if words(t) == "" {
				return fmt.Errorf("%s has no words to fold", k)
			}
			put = " >" + d.e.nl + indented(t, indent+"  ", d.e.nl)
			v = words(t) + "\n"
		case float64:
			put = " " + Number(t) + d.e.nl
		case bool:
			put = fmt.Sprintf(" %t%s", t, d.e.nl)
		case []any:
			flowList, err := flowStrings(t)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
			put = " " + flowList + d.e.nl
			v = Copy(t)
		default:
			return fmt.Errorf("%s is neither a text, a number, null nor a list", k)
		}
		b.WriteString(prefix + k + ":" + put)
		wrote.Set(k, v)
	}
	d.e.edits = append(d.e.edits, edit{at: at, put: b.String()})
	holder.Set(key, append(append([]any{}, items...), wrote))
	return nil
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

// SetList gives the key the path names the list of strings: a flow list on
// one line replaced, written over a value of nothing (null, ~ or none), or
// added as a flow list when the mapping lacks the key (as Set adds one); a
// block list has its items replaced in place, those past the new list's end
// taken out with their lines and new ones added after its last, each a line
// of its own. A block list is not emptied: that is an error.
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
	_, v := lookup(parent, key)
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
	case v.Kind == yaml.SequenceNode && (flow || v.Style&yaml.FlowStyle != 0):
		start, end, err := d.flowSpan(v)
		if err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
		d.e.edits = append(d.e.edits, edit{at: start, cut: end - start, put: flowList})
	case v.Kind == yaml.SequenceNode:
		if err := d.setBlockList(v, list); err != nil {
			return fmt.Errorf("%s: %w", where(path), err)
		}
	default:
		return fmt.Errorf("%s is not a list", where(path))
	}
	m.Set(key, values)
	return nil
}

// setBlockList writes list over a block list's items, each a one-line
// scalar on a line of its own.
func (d *Doc) setBlockList(v *yaml.Node, list []string) error {
	if len(list) == 0 {
		return fmt.Errorf("a block list is not emptied in place; write it as [] first")
	}
	for i, el := range v.Content {
		if el.Kind != yaml.ScalarNode {
			return fmt.Errorf("the list's item on line %d is not a text", el.Line)
		}
		if i < len(list) {
			if err := d.replace(el, false, list[i]); err != nil {
				return err
			}
			continue
		}
		start, err := d.e.offset(el.Line, 1)
		if err != nil {
			return err
		}
		end, _ := d.e.afterLine(start)
		d.e.edits = append(d.e.edits, edit{at: start, cut: end - start})
	}
	if len(list) <= len(v.Content) {
		return nil
	}
	last := v.Content[len(v.Content)-1]
	start, err := d.e.offset(last.Line, 1)
	if err != nil {
		return err
	}
	head := d.e.text[start:d.e.lineEnd(start)]
	dash := strings.Repeat(" ", len(head)-len(strings.TrimLeft(head, " ")))
	at, lead := d.e.afterLine(start)
	var b strings.Builder
	b.WriteString(lead)
	for _, s := range list[len(v.Content):] {
		b.WriteString(dash + "- " + token(s, 0) + d.e.nl)
	}
	d.e.edits = append(d.e.edits, edit{at: at, put: b.String()})
	return nil
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

// flowSpan is where a flow collection written on one line is in the text:
// its opening bracket and just past its closing one.
func (d *Doc) flowSpan(n *yaml.Node) (int, int, error) {
	start, err := d.e.offset(n.Line, n.Column)
	if err != nil {
		return 0, 0, err
	}
	line := d.e.text[start:d.e.lineEnd(start)]
	if line == "" || (line[0] != '[' && line[0] != '{') {
		return 0, 0, fmt.Errorf("line %d does not open a flow collection", n.Line)
	}
	depth := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			for i++; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' {
					i++
				}
			}
		case '\'':
			for i++; i < len(line); i++ {
				if line[i] == '\'' {
					if i+1 < len(line) && line[i+1] == '\'' {
						i++
						continue
					}
					break
				}
			}
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				return start, start + i + 1, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("the list on line %d goes on past its line", n.Line)
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
