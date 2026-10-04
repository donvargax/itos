package value

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// Editing YAML text in place (itos pin, slice 47): nothing else in itos
// writes YAML a person wrote, and re-encoding the document would lose its
// comments, its quoting and its layout. SetScalars changes the few bytes
// that hold the values and leaves every other byte as it was; what it cannot
// change that way is an error, and the text is left to the person.

// SetScalars is the YAML text with the top-level mapping's section holding
// each pair's key set to its value, a string, every other byte of the text
// as it was. A value already there is replaced in the style it is written
// in (plain, single or double quoted); a key the section does not have is
// added after its last one, a line of its own in a block mapping or ", key:
// value" in a flow one; a section that is not there is added as a block
// after the top-level version line (else at the end), and one that is empty
// (nothing, ~, null or {}) is written below its key. A layout it cannot edit
// that way (a value over several lines, a tag, an anchor, an alias, a section
// that is a list or a scalar, a flow document) is an error, as is any edit
// that would not read back with the values set and everything else
// unchanged.
func SetScalars(text, section string, pairs [][2]string) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return "", err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode || doc.Content[0].Style&yaml.FlowStyle != 0 {
		return "", fmt.Errorf("its top is not a block mapping")
	}
	e := editor{text: text, nl: "\n"}
	if strings.Contains(text, "\r\n") {
		e.nl = "\r\n"
	}
	root := doc.Content[0]
	key, val := lookup(root, section)
	var err error
	switch {
	case key == nil:
		err = e.addSection(root, section, pairs)
	case isEmpty(val):
		err = e.fillSection(key, val, pairs)
	case val.Kind == yaml.MappingNode && val.Anchor == "" && val.Style&yaml.TaggedStyle == 0:
		err = e.setIn(val, pairs)
	default:
		err = fmt.Errorf("%s is not a mapping itos can edit", section)
	}
	if err != nil {
		return "", err
	}
	edited := e.apply()
	if err := sameBut(text, edited, section, pairs); err != nil {
		return "", err
	}
	return edited, nil
}

// lookup is a mapping's key node and value node for the key, nil when it has
// none.
func lookup(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i]; k.Kind == yaml.ScalarNode && k.Value == key {
			return k, m.Content[i+1]
		}
	}
	return nil, nil
}

// isEmpty is whether a section's value holds nothing: no value, ~, null or {}.
func isEmpty(n *yaml.Node) bool {
	if n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 {
		return false
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Tag == "!!null" && n.Style&quotedStyles == 0
	case yaml.MappingNode:
		return len(n.Content) == 0 && n.Style&yaml.FlowStyle != 0
	}
	return false
}

// An edit replaces the bytes [at, at+cut) with put.
type edit struct {
	at, cut int
	put     string
}

type editor struct {
	text  string
	nl    string
	edits []edit
}

// apply is the text with the edits made, from the last one back so each
// offset still holds; of two at the same offset the later is made first, so
// what the earlier writes there comes before it.
func (e *editor) apply() string {
	order := make([]int, len(e.edits))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		if e.edits[a].at != e.edits[b].at {
			return e.edits[b].at - e.edits[a].at
		}
		return b - a
	})
	text := e.text
	for _, i := range order {
		d := e.edits[i]
		text = text[:d.at] + d.put + text[d.at+d.cut:]
	}
	return text
}

// offset is the byte offset of a node's line and column (both from 1, the
// column in characters).
func (e *editor) offset(line, column int) (int, error) {
	at := 0
	for l := 1; l < line; l++ {
		i := strings.IndexByte(e.text[at:], '\n')
		if i < 0 {
			return 0, fmt.Errorf("line %d is past the end", line)
		}
		at += i + 1
	}
	for c := 1; c < column; c++ {
		if at >= len(e.text) || e.text[at] == '\n' {
			return 0, fmt.Errorf("line %d has no column %d", line, column)
		}
		_, size := utf8.DecodeRuneInString(e.text[at:])
		at += size
	}
	return at, nil
}

// lineEnd is the offset of the end of the line holding at, before its line
// break.
func (e *editor) lineEnd(at int) int {
	i := strings.IndexByte(e.text[at:], '\n')
	if i < 0 {
		return len(e.text)
	}
	end := at + i
	if end > at && e.text[end-1] == '\r' {
		end--
	}
	return end
}

// afterLine is the offset just past the line break of the line holding at,
// and the break to write first when that line is the last and has none.
func (e *editor) afterLine(at int) (int, string) {
	i := strings.IndexByte(e.text[at:], '\n')
	if i < 0 {
		return len(e.text), e.nl
	}
	return at + i + 1, ""
}

// span is where a one-line scalar's token is in the text: its start and its
// end. An empty plain value is the empty span just after its key's colon.
func (e *editor) span(n *yaml.Node, flow bool) (int, int, error) {
	if n.Kind != yaml.ScalarNode || n.Anchor != "" || n.Style&(yaml.TaggedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return 0, 0, fmt.Errorf("line %d is not a one-line scalar", n.Line)
	}
	start, err := e.offset(n.Line, n.Column)
	if err != nil {
		return 0, 0, err
	}
	end := e.lineEnd(start)
	line := e.text[start:end]
	switch {
	case n.Style&yaml.DoubleQuotedStyle != 0:
		for i := 1; i < len(line); i++ {
			switch line[i] {
			case '\\':
				i++
			case '"':
				return start, start + i + 1, nil
			}
		}
	case n.Style&yaml.SingleQuotedStyle != 0:
		for i := 1; i < len(line); i++ {
			if line[i] == '\'' {
				if i+1 < len(line) && line[i+1] == '\'' {
					i++
					continue
				}
				return start, start + i + 1, nil
			}
		}
	case n.Value == "":
		return start, start, nil
	default:
		stop := len(line)
		for i := 0; i < len(line); i++ {
			if flow && strings.IndexByte(",}]", line[i]) >= 0 {
				stop = i
				break
			}
			if line[i] == '#' && i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') {
				stop = i
				break
			}
		}
		token := strings.TrimRight(line[:stop], " \t")
		if token == n.Value {
			return start, start + len(token), nil
		}
	}
	return 0, 0, fmt.Errorf("the value on line %d goes on past its line", n.Line)
}

// token is a string written in the style of the one it replaces: plain where
// that was plain and the string reads back as itself, else double quoted, or
// single quoted where that was.
func token(s string, old yaml.Style) string {
	switch {
	case old&yaml.SingleQuotedStyle != 0:
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	case old&yaml.DoubleQuotedStyle != 0:
		return Quote(s)
	}
	if _, isString := Resolve(s).(string); isString && !notPlain.MatchString(s) {
		return s
	}
	return Quote(s)
}

// block is the section's pairs as block lines, each indented by indent.
func (e *editor) block(indent string, pairs [][2]string) string {
	var b strings.Builder
	for _, p := range pairs {
		b.WriteString(indent + p[0] + ": " + token(p[1], 0) + e.nl)
	}
	return b.String()
}

// addSection adds the section as a block after the top-level version line,
// when version holds a one-line value, else at the end.
func (e *editor) addSection(root *yaml.Node, section string, pairs [][2]string) error {
	text := section + ":" + e.nl + e.block("  ", pairs)
	if k, v := lookup(root, "version"); k != nil && k.Column == 1 && v.Kind == yaml.ScalarNode && v.Line == k.Line {
		if start, end, err := e.span(v, false); err == nil && start <= end {
			at, lead := e.afterLine(end)
			e.edits = append(e.edits, edit{at: at, put: lead + text})
			return nil
		}
	}
	lead := ""
	if e.text != "" && !strings.HasSuffix(e.text, "\n") {
		lead = e.nl
	}
	e.edits = append(e.edits, edit{at: len(e.text), put: lead + text})
	return nil
}

// fillSection writes the pairs as a block below the key of a section that
// holds nothing, taking out the ~, null or {} it held.
func (e *editor) fillSection(key, val *yaml.Node, pairs [][2]string) error {
	if val.Line != key.Line {
		return fmt.Errorf("%s's value is not on its line", key.Value)
	}
	start, err := e.offset(val.Line, val.Column)
	if err != nil {
		return err
	}
	end := start
	if val.Kind == yaml.MappingNode {
		i := strings.IndexByte(e.text[start:e.lineEnd(start)], '}')
		if i < 0 {
			return fmt.Errorf("%s's {} goes on past its line", key.Value)
		}
		end = start + i + 1
	} else if _, end, err = e.span(val, false); err != nil {
		return err
	}
	// The space before the value goes with it, unless a comment follows.
	cut := start
	if strings.TrimLeft(e.text[end:e.lineEnd(end)], " \t") == "" {
		for cut > 0 && (e.text[cut-1] == ' ' || e.text[cut-1] == '\t') {
			cut--
		}
	}
	if end > cut {
		e.edits = append(e.edits, edit{at: cut, cut: end - cut})
	}
	indent := strings.Repeat(" ", key.Column-1+2)
	at, lead := e.afterLine(end)
	e.edits = append(e.edits, edit{at: at, put: lead + e.block(indent, pairs)})
	return nil
}

// setIn sets each pair in the section's mapping: replacing a value there,
// adding a key that is not after the last.
func (e *editor) setIn(m *yaml.Node, pairs [][2]string) error {
	flow := m.Style&yaml.FlowStyle != 0
	var missing [][2]string
	for _, p := range pairs {
		k, v := lookup(m, p[0])
		if k == nil {
			missing = append(missing, p)
			continue
		}
		start, end, err := e.span(v, flow)
		if err != nil {
			return err
		}
		put := token(p[1], v.Style)
		if start == end {
			put = " " + put
		}
		e.edits = append(e.edits, edit{at: start, cut: end - start, put: put})
	}
	if len(missing) == 0 {
		return nil
	}
	last := m.Content[len(m.Content)-1]
	_, end, err := e.span(last, flow)
	if err != nil {
		return err
	}
	if flow {
		var b strings.Builder
		for _, p := range missing {
			b.WriteString(", " + p[0] + ": " + token(p[1], yaml.DoubleQuotedStyle))
		}
		e.edits = append(e.edits, edit{at: end, put: b.String()})
		return nil
	}
	at, lead := e.afterLine(end)
	indent := strings.Repeat(" ", m.Content[0].Column-1)
	e.edits = append(e.edits, edit{at: at, put: lead + e.block(indent, missing)})
	return nil
}

// sameBut checks the edited text reads as the old one but for the section's
// pairs, which it holds as set.
func sameBut(old, edited, section string, pairs [][2]string) error {
	was, err := Parse(old)
	if err != nil {
		return err
	}
	now, err := Parse(edited)
	if err != nil {
		return fmt.Errorf("the edit would not read back: %w", err)
	}
	wasMap, ok1 := was.(*Map)
	nowMap, ok2 := now.(*Map)
	if !ok1 || !ok2 {
		return fmt.Errorf("the edit would not read back as a mapping")
	}
	wasSection, nowSection := wasMap.At(section), nowMap.At(section)
	for _, p := range pairs {
		if got := Prop(nowSection, p[0]); got != p[1] {
			return fmt.Errorf("the edit would read %s.%s as %s, not %s", section, p[0], JSON(got), p[1])
		}
	}
	wasRest, nowRest := Copy(wasMap).(*Map), Copy(nowMap).(*Map)
	wasRest.Delete(section)
	nowRest.Delete(section)
	if JSON(wasRest) != JSON(nowRest) {
		return fmt.Errorf("the edit would change more than %s", section)
	}
	if was, ok := wasSection.(*Map); ok {
		now, ok := nowSection.(*Map)
		if !ok {
			return fmt.Errorf("the edit would not read %s back as a mapping", section)
		}
		was, now = Copy(was).(*Map), Copy(now).(*Map)
		for _, p := range pairs {
			was.Delete(p[0])
			now.Delete(p[0])
		}
		if JSON(was) != JSON(now) {
			return fmt.Errorf("the edit would change more of %s than its values", section)
		}
	}
	return nil
}
