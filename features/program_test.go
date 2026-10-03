// Programs that are shell scripts, on every platform. A scenario's fake
// programs (an extension, a release's itos, the config's shell) are shell
// scripts, which Linux and macOS run by their #! line and windows does not:
// there a program is found by its extension (PATHEXT) and must be one windows
// can start. So on windows a step writes <name>.exe instead, script-exe
// (testdata/script-exe) with the script after it, which runs the script with
// Git for Windows' sh; elsewhere it writes the script itself.
package features

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
)

// scriptMarker is the line between script-exe and its script (marker in
// testdata/script-exe).
const scriptMarker = "\n#itos-features-script\n"

var (
	scriptExeOnce  sync.Once
	scriptExeBytes []byte
	scriptExeErr   error
)

// scriptExe is script-exe's bytes, built once a run from the module at root.
func scriptExe(root string) ([]byte, error) {
	scriptExeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "itos-features-script-exe-")
		if err != nil {
			scriptExeErr = err
			return
		}
		defer os.RemoveAll(dir)
		out := filepath.Join(dir, "script-exe.exe")
		cmd := exec.Command("go", "build", "-o", out, "./features/testdata/script-exe")
		cmd.Dir = root
		if text, err := cmd.CombinedOutput(); err != nil {
			scriptExeErr = fmt.Errorf("building script-exe: %v\n%s", err, text)
			return
		}
		scriptExeBytes, scriptExeErr = os.ReadFile(out)
	})
	return scriptExeBytes, scriptExeErr
}

// program is a program that runs the script: the script itself, or on
// windows script-exe with the script after it.
func (w *world) program(script string) ([]byte, error) {
	if runtime.GOOS != "windows" {
		return []byte(script), nil
	}
	exe, err := scriptExe(w.root)
	if err != nil {
		return nil, err
	}
	return append(append(slices.Clone(exe), scriptMarker...), script...), nil
}

// programPath is the file a program at path is: path.exe on windows.
func programPath(path string) string {
	if runtime.GOOS == "windows" {
		return path + ".exe"
	}
	return path
}

// writeProgram writes, executable, a program at path (path.exe on windows)
// that runs the script.
func (w *world) writeProgram(path, script string) error {
	text, err := w.program(script)
	if err != nil {
		return err
	}
	return os.WriteFile(programPath(path), text, 0o755)
}
