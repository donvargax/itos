package corpus

import (
	"encoding/json"
	"net/http"
	"testing"
)

// get is the fake's status and JSON body for path, asked with the token.
func get(t *testing.T, address, path string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, address+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+fakeToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, body
}

func TestFakeGitHubRepository(t *testing.T) {
	for visibility, private := range map[string]bool{"public": false, "private": true, "internal": true} {
		address, stop, err := serveGitHub(map[string]any{"visibility": visibility})
		if err != nil {
			t.Fatal(err)
		}
		status, body := get(t, address, "/repos/"+fakeRepository)
		stop()
		if status != http.StatusOK || body["visibility"] != visibility || body["private"] != private {
			t.Errorf("visibility %s: answered %d %v", visibility, status, body)
		}
	}
	address, stop, err := serveGitHub(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if status, body := get(t, address, "/repos/"+fakeRepository); status != http.StatusNotFound {
		t.Errorf("no visibility: answered %d %v", status, body)
	}
	if status, _ := get(t, address, "/repos/other/repository"); status != http.StatusNotFound {
		t.Errorf("another repository: answered %d", status)
	}
}
