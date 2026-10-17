package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// followup add saves the thread and says so: who it is with and its title,
// or under --json the thread itself.
func TestFollowAddSaysTheThreadItOpened(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	code, stdout, stderr := run("followup", "add", "sync-ana", "--with", "ana", "--title", "The sync design", "--note", "Started.")
	if code != 0 || stdout != "sync-ana opened, with ana: The sync design\n" {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	code, stdout, stderr = run("followup", "add", "flaky-bo", "--with", "bo", "--title", "The flaky test", "--note", "Started.", "--json")
	var got struct {
		OK     bool `json:"ok"`
		Thread struct {
			ID, With, Title, Status string
		} `json:"thread"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); code != 0 || err != nil || !got.OK ||
		got.Thread.ID != "flaky-bo" || got.Thread.With != "bo" || got.Thread.Title != "The flaky test" || got.Thread.Status != "open" {
		t.Fatalf("--json: exit %d, %v, %+v\n%s%s", code, err, got, stdout, stderr)
	}
	if code, stdout, stderr := run("followup"); code != 0 || !strings.Contains(stdout, "sync-ana") || !strings.Contains(stdout, "flaky-bo") {
		t.Errorf("followup: exit %d\n%s%s", code, stdout, stderr)
	}
}
