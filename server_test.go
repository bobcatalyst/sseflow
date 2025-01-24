package sseflow

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestUpgrade(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
	w := httptest.NewRecorder()
	u := Upgrade(w, r)

	u.Push(&Message{
		Data: "foo",
	})
	u.Push(&Message{
		Event: "foo",
		Data:  "bar",
	})
	u.Push(&Message{
		ID:    func(s string) *string { return &s }("baz"),
		Retry: 32,
		Event: "bar",
		Data:  "foo",
	})
	u.Push(&Message{
		Comment: []string{
			"hello world",
		},
	})
	u.Push(&Message{
		Comment: []string{""},
	})

	if u.Close() != nil {
		t.Errorf("closed with error: %v", u.Close())
	}

	if w.Body.String() != `data:foo

event:foo
data:bar

id:baz
retry:32
event:bar
data:foo

:hello world

:

` {
		t.Error("invalid stream body")
		t.Log(w.Body.String())
	}
}
