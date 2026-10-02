// Package source is where itos reads its own data (tools/itos/source.ts): the
// config, the ledger, the work registry, the people and the smoke sets. Every
// reader of them comes here, so the commit-msg hook's check of the staged data
// can later read a tree git holds (the index, a commit) through the same
// readers; the port has the working tree alone so far.
//
// A read that fails says so in Node's words (`ENOENT: no such file or
// directory, open 'people.yaml'`), since what itos prints quotes them.
package source

import (
	"errors"
	"os"
	"syscall"
)

// Has is whether the working tree has the path (existsSync).
func Has(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Read is the file's text.
func Read(path string) (string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return "", nodeError(err, "open", path)
	}
	return string(text), nil
}

// List is the names of the entries directly in a folder, sorted.
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nodeError(err, "scandir", dir)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

// nodeError is a failed file operation as Node reports it: the code, what it
// means, the call and the path.
func nodeError(err error, call, path string) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err
	}
	switch errno {
	case syscall.ENOENT:
		return errors.New("ENOENT: no such file or directory, " + call + " '" + path + "'")
	case syscall.ENOTDIR:
		return errors.New("ENOTDIR: not a directory, " + call + " '" + path + "'")
	case syscall.EACCES:
		return errors.New("EACCES: permission denied, " + call + " '" + path + "'")
	case syscall.EISDIR:
		return errors.New("EISDIR: illegal operation on a directory, read")
	}
	return err
}
