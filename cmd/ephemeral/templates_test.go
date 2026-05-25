package main

import "testing"

func TestParseTemplates(t *testing.T) {
	if _, err := parseTemplates(); err != nil {
		t.Fatalf("parseTemplates(): %v", err)
	}
}
