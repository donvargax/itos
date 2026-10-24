package main

import "time"

// localZone is the zone the run formats times in: on Windows, where Go takes
// it from the OS and ignores TZ, the TZ a loader reads; the OS's zone (the
// current time.Local) when TZ is empty or unreadable, or on any other OS,
// where Go reads TZ itself.
func localZone(goos, tz string, load func(string) (*time.Location, error)) *time.Location {
	if goos != "windows" || tz == "" {
		return time.Local
	}
	loc, err := load(tz)
	if err != nil {
		return time.Local
	}
	return loc
}
