package appcache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryIsolatedValuesExpiryAndPrefixInvalidation(t *testing.T) {
	c := NewMemory(32)
	if err := c.Set(context.Background(), "user:one:0:/progress", []byte("one"), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(context.Background(), "user:two:0:/progress", []byte("two"), time.Second); err != nil {
		t.Fatal(err)
	}
	b, ok, err := c.Get(context.Background(), "user:one:0:/progress")
	if err != nil || !ok || string(b) != "one" {
		t.Fatalf("unexpected first value %q, %v, %v", b, ok, err)
	}
	b[0] = 'X'
	b, ok, _ = c.Get(context.Background(), "user:one:0:/progress")
	if !ok || string(b) != "one" {
		t.Fatalf("cache returned aliased data: %q", b)
	}
	if err := c.DeletePrefix(context.Background(), "user:one:"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Get(context.Background(), "user:one:0:/progress"); ok {
		t.Fatal("deleted user's value remained")
	}
	b, ok, _ = c.Get(context.Background(), "user:two:0:/progress")
	if !ok || string(b) != "two" {
		t.Fatalf("other user's value was invalidated: %q", b)
	}
	if err := c.Set(context.Background(), "short", []byte("x"), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, ok, _ := c.Get(context.Background(), "short"); ok {
		t.Fatal("expired value remained")
	}
}
