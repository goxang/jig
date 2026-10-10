package main

import "testing"

func TestGreet(t *testing.T) {
	if got := greet("jig"); got != "hello, jig" {
		t.Fatalf("greet(jig) = %q", got)
	}
}
