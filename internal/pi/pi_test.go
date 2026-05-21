package pi

import (
	"reflect"
	"testing"
)

func TestBuildArgsDefaults(t *testing.T) {
	t.Parallel()
	got := BuildArgs(Options{})
	want := []string{"--mode", "rpc", "--no-session"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs(zero) = %v, want %v", got, want)
	}
}

func TestBuildArgsWithSession(t *testing.T) {
	t.Parallel()
	got := BuildArgs(Options{Session: "/tmp/s"})
	want := []string{"--mode", "rpc", "--session", "/tmp/s"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs(session) = %v, want %v", got, want)
	}
}

func TestBuildArgsFull(t *testing.T) {
	t.Parallel()
	got := BuildArgs(Options{
		Model:    "GLM 5.1",
		Thinking: "high",
		Tools:    "bash,read",
		Session:  "/tmp/s",
	})
	want := []string{
		"--mode", "rpc",
		"--session", "/tmp/s",
		"--model", "GLM 5.1",
		"--thinking", "high",
		"-t", "bash,read",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs(full) =\n  %v\nwant\n  %v", got, want)
	}
}

func TestBuildArgsOmitsEmptyFields(t *testing.T) {
	t.Parallel()
	got := BuildArgs(Options{Model: "GLM 5.1"})
	want := []string{"--mode", "rpc", "--no-session", "--model", "GLM 5.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs(model only) = %v, want %v", got, want)
	}
}
