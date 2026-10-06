//go:build !windows

package main

// holdTree does nothing: script-exe runs only on windows, where a step needs
// a program that is a shell script (holdTree in job_windows.go).
func holdTree() {}
