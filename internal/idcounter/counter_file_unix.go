//go:build !windows

package idcounter

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
