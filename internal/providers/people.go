// Package providers is what itos asks of the world outside the repository
// (tools/itos/providers.ts), each a provider chosen in itos.yaml: where a
// push's range starts (ci.range), who a session works for (work.identity)
// and who may own work (work.people). The port has the range providers
// (range.go: none, command and github), which ci range asks, the identity
// providers (identity.go: command, none and github, the last gh's answer),
// which work asks, and the people, which the work registry's check reads.
package providers

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/source"
	"github.com/donvargax/itos/internal/value"
)

var contributorsList = regexp.MustCompile(`(?s)<!-- ALL-CONTRIBUTORS-LIST:START[^>]*-->(.*?)<!-- ALL-CONTRIBUTORS-LIST:END`)

// unique is a list without its repeats, in first-seen order.
func unique(list []string) []string {
	seen := map[string]bool{}
	kept := []string{}
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			kept = append(kept, s)
		}
	}
	return kept
}

// AllContributorsMd are the logins of an All Contributors table: each
// person's cell links to their profile (loginFrom,
// https://github.com/{login} by default) between the list's markers.
func AllContributorsMd(markdown string, loginFrom *string) []string {
	from := "https://github.com/{login}"
	if loginFrom != nil {
		from = *loginFrom
	}
	parts := strings.Split(from, "{login}")
	before, after := parts[0], ""
	if len(parts) > 1 {
		after = parts[1]
	}
	link := regexp.MustCompile(`href="` + regexp.QuoteMeta(before) + `([^"/?#` + value.SpaceChars + `]+)` + regexp.QuoteMeta(after) + `"`)
	list := ""
	if m := contributorsList.FindStringSubmatch(markdown); m != nil {
		list = m[1]
	}
	var logins []string
	for _, m := range link.FindAllStringSubmatch(list, -1) {
		logins = append(logins, m[1])
	}
	return unique(logins)
}

// AllContributorsRc are the logins of an .all-contributorsrc: its
// contributors[].login.
func AllContributorsRc(text string) ([]string, error) {
	var rc any
	if err := json.Unmarshal([]byte(text), &rc); err != nil {
		return nil, err
	}
	object, _ := rc.(map[string]any)
	contributors, ok := object["contributors"].([]any)
	if !ok {
		return nil, errors.New("has no contributors list")
	}
	var logins []string
	for i, c := range contributors {
		person, _ := c.(map[string]any)
		login, ok := person["login"].(string)
		if !ok || login == "" {
			return nil, fmt.Errorf("contributors[%d] has no login", i)
		}
		logins = append(logins, login)
	}
	return unique(logins), nil
}

// YAMLLogins are the logins of a YAML file that is a list of them.
func YAMLLogins(text string) ([]string, error) {
	parsed, err := value.Parse(text)
	if err != nil {
		return nil, err
	}
	list, ok := parsed.([]any)
	if !ok {
		return nil, errors.New("is not a list of logins")
	}
	var logins []string
	for i, l := range list {
		login, ok := l.(string)
		if !ok || login == "" {
			return nil, fmt.Errorf("entry %d is not a login", i)
		}
		logins = append(logins, login)
	}
	return unique(logins), nil
}

// People are who may own work, read from the file the people source names.
func People(p config.People) ([]string, error) {
	text, err := source.Read(p.File)
	if err != nil {
		return nil, err
	}
	var logins []string
	switch p.Source {
	case "all-contributorsrc":
		logins, err = AllContributorsRc(text)
	case "yaml":
		logins, err = YAMLLogins(text)
	default:
		logins = AllContributorsMd(text, p.LoginFrom)
	}
	if err != nil {
		return nil, fmt.Errorf("%s (work.people, %s) %s", p.File, p.Source, err)
	}
	return logins, nil
}
