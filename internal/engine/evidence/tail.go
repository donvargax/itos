package evidence

// tail keeps the last max bytes written to it, which is all of a record's
// output: a check that prints a great deal leaves its end.
type tail struct {
	max int
	buf []byte
	cut bool
}

// Write keeps p, dropping the oldest bytes once it holds twice what it keeps,
// so a long output costs no more than twice the tail.
func (t *tail) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*t.max {
		t.buf = append([]byte{}, t.buf[len(t.buf)-t.max:]...)
		t.cut = true
	}
	return len(p), nil
}

// Text is the tail, and whether bytes before it were dropped.
func (t *tail) Text() (string, bool) {
	if len(t.buf) > t.max {
		return string(t.buf[len(t.buf)-t.max:]), true
	}
	return string(t.buf), t.cut
}
