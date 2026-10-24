package main

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// localZone is judged with "windows" on every OS: a readable TZ sets the
// zone, an empty or unreadable one keeps the OS's, and no other OS is asked.
func TestLocalZone(t *testing.T) {
	tokyo := time.FixedZone("Asia/Tokyo", 9*60*60)
	var asked []string
	load := func(name string) (*time.Location, error) {
		asked = append(asked, name)
		if name == "Asia/Tokyo" {
			return tokyo, nil
		}
		return nil, errors.New("unknown time zone " + name)
	}
	for _, c := range []struct {
		name, goos, tz string
		want           *time.Location
		asked          []string
	}{
		{"a readable TZ on Windows", "windows", "Asia/Tokyo", tokyo, []string{"Asia/Tokyo"}},
		{"an unreadable TZ on Windows", "windows", "Nowhere/Land", time.Local, []string{"Nowhere/Land"}},
		{"no TZ on Windows", "windows", "", time.Local, nil},
		{"a readable TZ on Linux", "linux", "Asia/Tokyo", time.Local, nil},
		{"a readable TZ on macOS", "darwin", "Asia/Tokyo", time.Local, nil},
	} {
		asked = nil
		if got := localZone(c.goos, c.tz, load); got != c.want {
			t.Errorf("%s: localZone = %v, want %v", c.name, got, c.want)
		}
		if !slices.Equal(asked, c.asked) {
			t.Errorf("%s: the loader was asked for %v, want %v", c.name, asked, c.asked)
		}
	}
}

// UTC needs no zone database, so time.LoadLocation reads it on Windows too.
func TestLocalZoneReadsUTC(t *testing.T) {
	if got := localZone("windows", "UTC", time.LoadLocation); got != time.UTC {
		t.Errorf("localZone(windows, UTC) = %v, want UTC", got)
	}
}
