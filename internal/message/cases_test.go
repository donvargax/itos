package message

import (
	"slices"
	"testing"
)

// A break is any space or punctuation, in Latin-1 or past it, and any Latin-1
// character that is no letter or digit, a symbol among them.
func TestIsBreak(t *testing.T) {
	for r, want := range map[rune]bool{
		' ': true, '-': true, ';': true, '_': true, '\t': true, '+': true, '$': true,
		'\u00a0': true, '\u00d7': true, '\u2014': true, '\u3000': true,
		'a': false, 'Z': false, '0': false, '\u00e9': false,
	} {
		if got := isBreak(r); got != want {
			t.Errorf("isBreak(%U) = %v, want %v", r, got, want)
		}
	}
}

// Capitals before a capital that starts a lower-case word are a word of their
// own.
func TestWordsEndAnAcronymBeforeAWord(t *testing.T) {
	if got, want := words("XMLHttp"), []string{"XML", "Http"}; !slices.Equal(got, want) {
		t.Errorf("words = %q, want %q", got, want)
	}
}
