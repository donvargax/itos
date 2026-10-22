package cli

import "testing"

// An extension's command is the file's name after itos-, without its
// extension on windows alone; a file without the prefix names none.
func TestExtensionName(t *testing.T) {
	for _, c := range []struct {
		file, goos, want string
		ok               bool
	}{
		{"itos-hello", "linux", "hello", true},
		{"itos-hello.sh", "darwin", "hello.sh", true},
		{"itos-hello.exe", "linux", "hello.exe", true},
		{"itos-hello.exe", "windows", "hello", true},
		{"itos-hello", "windows", "hello", true},
		{"hello.exe", "windows", "hello", false},
		{"hello", "linux", "hello", false},
	} {
		if got, ok := extensionName(c.file, c.goos); got != c.want || ok != c.ok {
			t.Errorf("extensionName(%s, %s) = %q, %v", c.file, c.goos, got, ok)
		}
	}
}
