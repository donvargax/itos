package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/tools/selftest/scratch"
	"go.yaml.in/yaml/v3"
)

// The nightly's platform job runs a platform's features in shards when one job would near go
// test's ten minutes (T-308): every platform runs every feature exactly once across its shards.
type nightlyEntry struct{ OS, Shard string }

func nightlyPlatform(t *testing.T) (job struct {
	Strategy struct {
		Matrix struct{ Include []nightlyEntry }
	}
	Steps []step
}) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(scratch.Root(t), ".github/workflows/nightly.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var w struct{ Jobs map[string]any }
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	platform, err := yaml.Marshal(w.Jobs["platform"])
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(platform, &job); err != nil {
		t.Fatal(err)
	}
	return job
}

func testNightlyShards(t *testing.T) {
	platform := nightlyPlatform(t)
	shardsOf := map[string][]string{}
	for _, e := range platform.Strategy.Matrix.Include {
		shardsOf[e.OS] = append(shardsOf[e.OS], e.Shard)
	}
	for _, name := range []string{"ubuntu-latest", "macos-latest", "windows-latest"} {
		shards, ok := shardsOf[name]
		if !ok {
			t.Fatalf("the nightly's platform job has no %s entry", name)
		}
		if err := completeShards(shards); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	var units, features *step
	for i, s := range platform.Steps {
		switch s.Name {
		case "Unit tests":
			units = &platform.Steps[i]
		case "Features":
			features = &platform.Steps[i]
		}
	}
	if units == nil || features == nil {
		t.Fatal("the nightly's platform job lacks its Unit tests or Features step")
	}
	for _, e := range platform.Strategy.Matrix.Include {
		dir := t.TempDir()
		scratch.WriteFiles(t, dir, map[string]string{"go": "#!/bin/sh\necho \"$*\" >> \"$LOG\"\n", "log": ""})
		if err := os.Chmod(filepath.Join(dir, "go"), 0o755); err != nil {
			t.Fatal(err)
		}
		set := []string{"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH"),
			"LOG=" + filepath.Join(dir, "log"), "RUNNER_TEMP=" + dir}
		for name, value := range features.Env {
			bound, err := matrixValue(value, e)
			if err != nil {
				t.Fatal(err)
			}
			set = append(set, name+"="+bound)
		}
		env := scratch.With(scratch.OutsideEnv(), set...)
		if r := scratch.Run(dir, env, "", "bash", "-eo", "pipefail", "-c", features.Run); r.Code != 0 {
			t.Fatalf("%v: the Features step exited %d\n%s", e, r.Code, r.Output())
		}
		log, err := os.ReadFile(filepath.Join(dir, "log"))
		if err != nil {
			t.Fatal(err)
		}
		want := "test ./features -count=1 -timings=" + dir + "/features-timings.txt\n"
		if e.Shard != "" {
			want = "test ./features -count=1 -shard=" + e.Shard + " -timings=" + dir + "/features-timings.txt\n"
		}
		if string(log) != want {
			t.Errorf("%v: the Features step ran %q, want %q", e, log, want)
		}
	}

	// The unit tests run once a platform: with no shard, or in the first.
	if want := "${{ !matrix.shard || startsWith(matrix.shard, '1/') }}"; units.If != want {
		t.Fatalf("the Unit tests step's condition is %q, want %q", units.If, want)
	}
	for name, shards := range shardsOf {
		ran := 0
		for _, shard := range shards {
			if shard == "" || strings.HasPrefix(shard, "1/") {
				ran++
			}
		}
		if ran != 1 {
			t.Errorf("%s runs the unit tests %d times, want once", name, ran)
		}
	}
}

// matrixValue is an env value of the Features step with its matrix expression bound to the entry's,
// so the step runs with what the workflow binds and nothing the test supplies.
func matrixValue(value string, e nightlyEntry) (string, error) {
	value = strings.ReplaceAll(value, "${{ matrix.shard }}", e.Shard)
	if strings.Contains(value, "${{") {
		return "", fmt.Errorf("the Features step binds %q, an expression this test does not evaluate", value)
	}
	return value, nil
}

func TestMatrixValue(t *testing.T) {
	e := nightlyEntry{Shard: "1/2"}
	if got, err := matrixValue("${{ matrix.shard }}", e); got != "1/2" || err != nil {
		t.Errorf("the shard expression gave %q, %v", got, err)
	}
	if got, err := matrixValue("${{ matrix.os }}", e); err == nil {
		t.Errorf("an unsupported expression gave %q, want an error", got)
	}
}

// completeShards is whether the shards of one platform are all absent (one job runs every
// feature) or every "i/n" of one n, 1 <= i <= n, once each.
func completeShards(shards []string) error {
	if len(shards) == 1 && shards[0] == "" {
		return nil
	}
	n := 0
	seen := map[int]bool{}
	for _, shard := range shards {
		i, count, found := strings.Cut(shard, "/")
		index, err1 := strconv.Atoi(i)
		total, err2 := strconv.Atoi(count)
		if !found || err1 != nil || err2 != nil || index < 1 || index > total {
			return fmt.Errorf("shard %q is not i/n with 1 <= i <= n", shard)
		}
		if n != 0 && total != n {
			return fmt.Errorf("shards of %d and of %d beside each other", n, total)
		}
		n = total
		if seen[index] {
			return fmt.Errorf("shard %q twice", shard)
		}
		seen[index] = true
	}
	if len(seen) != n {
		return fmt.Errorf("shards %q are not all %d of them", shards, n)
	}
	return nil
}

func TestCompleteShards(t *testing.T) {
	for _, tc := range []struct {
		shards []string
		ok     bool
	}{
		{[]string{""}, true},
		{[]string{"1/2", "2/2"}, true},
		{[]string{"2/2", "1/2"}, true},
		{[]string{"1/2"}, false},
		{[]string{"1/2", "1/2"}, false},
		{[]string{"1/2", "2/3"}, false},
		{[]string{"0/2", "1/2"}, false},
		{[]string{"", "1/2"}, false},
		{[]string{"x"}, false},
	} {
		if err := completeShards(tc.shards); (err == nil) != tc.ok {
			t.Errorf("%q: %v", tc.shards, err)
		}
	}
}
