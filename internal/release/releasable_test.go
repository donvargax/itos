package release

import "testing"

func TestReleasable(t *testing.T) {
	cases := map[string]bool{
		"feat: add the archive":                         true,
		"fix(cli): mend the archive":                    true,
		"docs: describe the archive":                    false,
		"refactor!: rename the archive":                 true,
		"chore: tidy\n\nWhy.\n\nBREAKING-CHANGE: gone":  true,
		"chore: tidy\n\nBREAKING CHANGE: gone\n\nWhy.":  false,
		"chore: tidy\n\nWhy.\n\nBREAKING CHANGE: gone":  true,
		"Merge branch 'main'":                           false,
		"feature: not a type the release cut counts":    false,
		"feat:no space after the colon is not a header": false,
	}
	for message, want := range cases {
		if got := Releasable(message); got != want {
			t.Errorf("Releasable(%q) = %v, want %v", message, got, want)
		}
	}
}

func TestNewest(t *testing.T) {
	cases := []struct {
		tags []string
		want string
	}{
		{nil, ""},
		{[]string{"latest", "1.2.0", "v1.2", "v1.2.0.1", "V1.2.0"}, ""},
		{[]string{"v1.2.0", "v1.10.0", "v1.9.9"}, "v1.10.0"},
		{[]string{"v2.0.0-rc.1", "v1.9.0"}, "v1.9.0"},
		{[]string{"v2.0.0-rc.1", "v2.0.0"}, "v2.0.0"},
		{[]string{"v2.0.0+build.1", "v1.9.0"}, "v1.9.0"},
		{[]string{"v2.0.0-rc.1", "v2.0.0+build.1"}, ""},
		{[]string{"v1.0.0", "v0.1.0", "v0.10.0"}, "v1.0.0"},
		{[]string{"v1.09.0", "v1.10.0"}, "v1.10.0"},
		{[]string{"v1.09.0", "v1.8.0"}, "v1.09.0"},
		{[]string{"v01.0.0", "v0.1.0"}, "v01.0.0"},
		{[]string{"v01.0.0", "v1.0.0", "v001.0.0"}, "v1.0.0"},
		{[]string{"v1.0.0", "v01.0.0"}, "v1.0.0"},
		{[]string{"v99999999999999999999.0.0", "v9.0.0"}, "v99999999999999999999.0.0"},
	}
	for _, c := range cases {
		if got := Newest(c.tags); got != c.want {
			t.Errorf("Newest(%q) = %q, want %q", c.tags, got, c.want)
		}
	}
}
