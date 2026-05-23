package testutil

import "testing"

func TestStripANSIRemovesColorCodes(t *testing.T) {
	input := "\033[1;32mhello\033[0m"
	want := "hello"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

func TestStripANSIPreservesPlainText(t *testing.T) {
	input := "hello world"
	if got := StripANSI(input); got != input {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, input)
	}
}

func TestStripANSIHandlesEmptyString(t *testing.T) {
	if got := StripANSI(""); got != "" {
		t.Errorf("StripANSI(\"\") = %q, want %q", got, "")
	}
}

func TestStripANSIHandlesIncompleteSequence(t *testing.T) {
	input := "hello\033["
	if got := StripANSI(input); got != "hello" {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, "hello")
	}
}

func TestStripANSIMultipleSequences(t *testing.T) {
	input := "\033[31mred\033[32mgreen\033[0m"
	want := "redgreen"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

func TestStripANSIEscapeOnly(t *testing.T) {
	input := "\033"
	if got := StripANSI(input); got != "\033" {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, "\033")
	}
}
