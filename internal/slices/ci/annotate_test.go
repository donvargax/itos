package ci

import "testing"

// ci run asks the run to annotate its failures when GITHUB_ACTIONS is "true"
// and no other value, for push, --tests and --nightly alike.
func TestRunAnnotatesUnderGitHubActionsOnly(t *testing.T) {
	for _, c := range []struct {
		value string
		want  bool
	}{{"true", true}, {"false", false}, {"True", false}, {"1", false}, {"", false}} {
		for _, args := range [][]string{{"run", start, "HEAD"}, {"run", "--nightly"}, {"run", "--nightly", "--tests"}} {
			p := over(t, nightlyText, nil)
			p.env = map[string]string{"GITHUB_ACTIONS": c.value}
			if _, _, _, err := call(t, p, args...); err != nil || len(p.options) != 1 || p.options[0].Annotate != c.want {
				t.Errorf("GITHUB_ACTIONS=%q, ci %v: %v, options %+v, want Annotate %v", c.value, args, err, p.options, c.want)
			}
		}
	}
}
