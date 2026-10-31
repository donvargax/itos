package reviewers

import (
	"reflect"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
)

// The commits Bugbot reviewed on PRs 241 and 242, and the one only the
// person's empty review is of on 242.
const (
	commit241      = "d7b087079e258a63846eab1c367ccc2405c5a884"
	commit241Stale = "411cbf305e971fd325c4bc053d6978bfc85b140c"
	commit242      = "a1786308c13d16fbec642365ed91c4b0a2e68add"
	commit242Bare  = "07299baf0958f624b68e3e11c181418f2631183f"
)

// bugbotMissing is what Missing says of Bugbot alone, configured as
// cursor[bot], over the reviews, at the head.
func bugbotMissing(reviews []providers.Authored, head string) []string {
	return lines(Missing([]config.Reviewer{bugbotReviewer}, providers.PullState{Reviews: reviews}, head))
}

var bugbotAsked = []string{bugbotLogin + " @cursor review"}

// The reviews Bugbot wrote on PRs 241, 242 and #243 (testdata/bugbot), by
// the login GraphQL gives, cursor, for the config's cursor[bot]: each answers
// the request at its own commit and at no other. The review 241 left stale
// still starts with the marker, so it answers for its commit as any other.
func TestBugbotReadsItsRecordedReviews(t *testing.T) {
	for name, commit := range map[string]string{
		"241-d7b0870-no-issues":  commit241,
		"241-411cbf3-stale":      commit241Stale,
		"242-a178630-no-issues":  commit242,
		"243-64da3fb-two-issues": head243,
	} {
		reviews := []providers.Authored{{Login: bugbotAuthor, Body: recorded(t, "bugbot", name), Commit: commit}}
		if got := bugbotMissing(reviews, commit); len(got) != 0 {
			t.Errorf("%s at its commit: missing %q", name, got)
		}
		if got := bugbotMissing(reviews, older); !reflect.DeepEqual(got, bugbotAsked) {
			t.Errorf("%s at another head: missing %q", name, got)
		}
	}
}

// Only the configured login's review counts: Bugbot's own body, marker and
// commit, written by another login, answers nothing (spec review round 2), nor
// does the review of a login configured as another kind.
func TestBugbotCountsOnlyTheConfiguredLogin(t *testing.T) {
	body := recorded(t, "bugbot", "243-64da3fb-two-issues")
	for _, login := range []string{"donvargax", codexLogin, "cursor-bot"} {
		if got := bugbotMissing([]providers.Authored{{Login: login, Body: body, Commit: head243}}, head243); !reflect.DeepEqual(got, bugbotAsked) {
			t.Errorf("Bugbot's review by %s: missing %q", login, got)
		}
	}
	both := Missing([]config.Reviewer{{Login: "someone", Kind: "bugbot"}, bugbotReviewer},
		providers.PullState{Reviews: []providers.Authored{{Login: bugbotAuthor, Body: body, Commit: head243}}}, head243)
	if got := lines(both); !reflect.DeepEqual(got, []string{"someone @cursor review"}) {
		t.Errorf("two Bugbot logins, one reviewed: missing %q", got)
	}
}

// PR 242 as GraphQL gives its reviews: the person's empty reviews (thread
// replies), Codex's reviews and Bugbot's at a1786308. At a1786308 Bugbot
// answered; at 07299baf, where only the person and Codex reviewed, and at
// 08d8cad, where only the person did, the request is missing. A review by
// cursor without the marker, and one GitHub gives no commit, answer nothing.
func TestBugbotPassesOverOtherReviews(t *testing.T) {
	codexReview := recorded(t, "codex", "242-f0be6ec-review-with-suggestions")
	reviews := []providers.Authored{
		{Login: "donvargax", Commit: commit242Bare},
		{Login: codexLogin, Body: codexReview, Commit: commit242Bare},
		{Login: "donvargax", Commit: commit242},
		{Login: codexLogin, Body: codexReview, Commit: commit242},
		{Login: "donvargax", Commit: head242},
		{Login: bugbotAuthor, Body: recorded(t, "bugbot", "242-a178630-no-issues"), Commit: commit242},
	}
	if got := bugbotMissing(reviews, commit242); len(got) != 0 {
		t.Errorf("at a1786308: missing %q", got)
	}
	for _, head := range []string{commit242Bare, head242} {
		if got := bugbotMissing(reviews, head); !reflect.DeepEqual(got, bugbotAsked) {
			t.Errorf("at %s: missing %q", head, got)
		}
	}
	for name, review := range map[string]providers.Authored{
		"no marker": {Login: bugbotAuthor, Body: "Bugbot found no new issues.", Commit: head243},
		"no commit": {Login: bugbotAuthor, Body: recorded(t, "bugbot", "243-64da3fb-two-issues")},
	} {
		head := head243
		if name == "no commit" {
			head = ""
		}
		if got := bugbotMissing([]providers.Authored{review}, head); !reflect.DeepEqual(got, bugbotAsked) {
			t.Errorf("%s: missing %q", name, got)
		}
	}
}
