package value

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// CheckWritten is the refusal every reader of a file itos writes gives
// before it decodes the text into its structs (bug 152): what Parse refuses,
// and a null key or a null list item at any depth, which itos writes in no
// file and which yaml/v3 drops without a word when it decodes into structs,
// so a save would lose what was there. It is Parse's own error (including
// ErrMultipleDocuments, which a reader may answer in its own words), or an
// error naming the line and column of the first null found. An empty text
// holds none.
func CheckWritten(text string) error {
	if _, err := Parse(text); err != nil {
		return err
	}
	var doc yaml.Node
	// Parse read it without error.
	_ = yaml.Unmarshal([]byte(text), &doc)
	return firstNull(&doc)
}

// firstNull is the refusal of the first null key or null list item among
// the nodes under n, in the order of the text; an alias is not followed, its
// anchor's node is among them.
func firstNull(n *yaml.Node) error {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if isNull(n.Content[i]) {
				return fmt.Errorf("line %d, column %d has a null key, which no file itos writes has", n.Content[i].Line, n.Content[i].Column)
			}
		}
	case yaml.SequenceNode:
		for _, e := range n.Content {
			if isNull(e) {
				return fmt.Errorf("line %d, column %d has a null list item, which no file itos writes has", e.Line, e.Column)
			}
		}
	}
	for _, c := range n.Content {
		if err := firstNull(c); err != nil {
			return err
		}
	}
	return nil
}

// isNull is whether a node reads as null, through the alias that stands for
// it (an alias cycle is Parse's refusal, so none is left to follow).
func isNull(n *yaml.Node) bool {
	for n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n.Kind == yaml.ScalarNode && scalar(n) == nil
}
