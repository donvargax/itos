//go:build windows

package idcounter

import "golang.org/x/sys/windows"

func replaceFile(source, destination string) error {
	return replaceFileWith(source, destination, windows.UTF16PtrFromString, windows.MoveFileEx,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
