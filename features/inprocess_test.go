package features

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/cucumber/godog"

	"github.com/donvargax/itos/v8/internal/cli"
)

// cli.Main changes process-global state. Keep this runner serial until those
// values are passed into Main instead.
var inProcessMu sync.Mutex

// Scenarios that need behavior provided by cmd/itos rather than cli.Main.
// Keep the reasons here so an in-process run makes its omissions explicit.
var inProcessDeferred = map[string]string{
	"ID-GUARD-11": "a pinned older release needs the launcher",
}

func inProcessScenarioID(s *godog.Scenario) string {
	for _, tag := range s.Tags {
		if strings.HasPrefix(tag.Name, "@ID-") {
			return strings.TrimPrefix(tag.Name, "@")
		}
	}
	return ""
}

func (w *world) runInProcess(dir string, stdin io.Reader, args ...string) (resultErr error) {
	inProcessMu.Lock()
	defer inProcessMu.Unlock()

	oldDir, err := os.Getwd()
	if err != nil {
		return err
	}
	oldEnv, oldStdin := os.Environ(), os.Stdin
	restore := func() error {
		var errs []error
		if err := os.Chdir(oldDir); err != nil {
			errs = append(errs, err)
		}
		if err := replaceEnv(oldEnv); err != nil {
			errs = append(errs, err)
		}
		os.Stdin = oldStdin
		return errors.Join(errs...)
	}
	defer func() { resultErr = errors.Join(resultErr, restore()) }()

	if err := replaceEnv(w.env()); err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	r, pipeW, err := os.Pipe()
	if err != nil {
		return err
	}
	os.Stdin = r
	var inputErr error
	done := make(chan struct{})
	if stdin == nil {
		_ = pipeW.Close()
	} else {
		go func() {
			defer close(done)
			_, inputErr = io.Copy(pipeW, stdin)
			_ = pipeW.Close()
		}()
	}
	var stdout, stderr bytes.Buffer
	w.exit = cli.Main(cli.Run{Args: args, Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS, Dir: dir})
	_ = r.Close()
	if stdin != nil {
		<-done
		if inputErr != nil {
			return fmt.Errorf("feeding in-process stdin: %w", inputErr)
		}
	}
	w.stdout, w.stderr = stdout.String(), stderr.String()
	return nil
}

func replaceEnv(env []string) error {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if err := os.Unsetenv(name); err != nil {
			return err
		}
	}
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			return fmt.Errorf("invalid environment entry %q", entry)
		}
		if err := os.Setenv(name, value); err != nil {
			return err
		}
	}
	return nil
}
