package jobs

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestJobKeepsLinesAndProgressBars(t *testing.T) {
	j := &Job{}
	j.Write([]byte("→ api: build\npulling 10%\rpulling 50%\rpulling 100%\npart"))
	j.Write([]byte("ial\r\n"))
	if got := strings.Join(j.Output(), "|"); got != "→ api: build|pulling 100%|partial" {
		t.Fatalf("output = %q", got)
	}
	if _, last, _ := j.State(); last != "partial" {
		t.Fatalf("last = %q", last)
	}
}

func TestJobWaitsForInputAndTakesIt(t *testing.T) {
	r, w := io.Pipe()
	j := &Job{Stdin: w}
	j.Write([]byte("step 1\npassword: "))
	j.wrote = time.Now().Add(-time.Second)
	if p, ok := j.Waiting(); !ok || p != "password:" {
		t.Fatalf("waiting = %q %v", p, ok)
	}
	got := make(chan string)
	go func() { b := make([]byte, 64); n, _ := r.Read(b); got <- string(b[:n]) }()
	if err := j.Answer("s3cret"); err != nil {
		t.Fatal(err)
	}
	if s := <-got; s != "s3cret\n" {
		t.Fatalf("stdin got %q", s)
	}
	if _, ok := j.Waiting(); ok {
		t.Fatal("still waiting after the answer")
	}
}

func TestManager(t *testing.T) {
	var m Manager
	m.Keep = 2
	ctx, j := m.Start(context.Background(), "a")
	io.WriteString(Out(ctx), "hi\n")
	m.Start(context.Background(), "b")
	m.Start(context.Background(), "c")
	if l := m.List(); len(l) != 2 || l[0].Label != "b" || m.Get(j.ID) != nil {
		t.Fatalf("kept %v", l)
	}
	j.Finish(nil)
	if ctx.Err() == nil || j.Output()[0] != "hi" {
		t.Fatal("finish did not cancel, or Out lost the line")
	}
}
