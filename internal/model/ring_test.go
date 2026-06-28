package model

import "testing"

func TestRingKeepsNewestBytes(t *testing.T) {
	r := NewRing(5)
	r.Write([]byte("abc"))
	r.Write([]byte("def"))
	if got := string(r.Bytes()); got != "bcdef" {
		t.Fatalf("ring bytes = %q, want %q", got, "bcdef")
	}
}

func TestRingLargeWrite(t *testing.T) {
	r := NewRing(4)
	r.Write([]byte("0123456789"))
	if got := string(r.Bytes()); got != "6789" {
		t.Fatalf("ring bytes = %q, want %q", got, "6789")
	}
}

func TestRingReset(t *testing.T) {
	r := NewRing(5)
	r.Write([]byte("abc"))
	r.Reset()
	if got := string(r.Bytes()); got != "" {
		t.Fatalf("ring bytes after reset = %q, want empty", got)
	}
}
