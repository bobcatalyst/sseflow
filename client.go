package sseflow

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/bobcatalyst/flow"
	"net/http"
	"sync"
	"time"
)

type ErrClosed struct {
	Retry uint
	ID    string
}

func (err *ErrClosed) GetRetry() time.Duration {
	return time.Duration(err.Retry) * time.Millisecond
}

func (err *ErrClosed) Error() string {
	return fmt.Sprintf("last-id(%s) retry(%s)", err.ID, err.GetRetry())
}

type Client struct {
	events flow.Stream[*Message]
	err    error
	done   chan struct{}
	start  func()
	stop   func()
}

func NewClient(ctx context.Context, r *http.Response) (*Client, error) {
	if ct := r.Header.Get("Content-Type"); ct != "text/event-stream" {
		return nil, fmt.Errorf("invalid content type %q, expected text/event-stream", ct)
	}

	start := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	cl := &Client{
		done:  make(chan struct{}),
		start: sync.OnceFunc(func() { close(start) }),
		stop:  cancel,
	}

	go func() {
		defer close(cl.done)
		defer cl.stop()
		defer cl.events.Close()
		select {
		case <-ctx.Done():
		case <-start:
			sc := bufio.NewScanner(r.Body)
			ec := ErrClosed{Retry: uint((time.Second * 5) * time.Millisecond)}
			err := cl.parseMessages(ctx, sc, &ec)
			cl.err = errors.Join(err, sc.Err(), &ec)
		}
	}()
	return cl, nil
}

func (cl *Client) parseMessages(ctx context.Context, sc *bufio.Scanner, ec *ErrClosed) error {
	var msg *Message
	for sc.Scan() && ctx.Err() == nil {
		if len(sc.Bytes()) == 0 {
			if msg != nil {
				cl.events.Push(msg)
				msg = nil
			}
			continue
		} else if msg == nil {
			msg = new(Message)
		}

		if _, err := msg.Write(sc.Bytes()); err != nil {
			return err
		}

		if msg.Retry > 0 {
			ec.Retry = msg.Retry
		}
		if msg.ID != nil {
			ec.ID = *msg.ID
		}
	}
	return nil
}

func (cl *Client) Listen(ctx context.Context) <-chan *Message {
	return cl.events.Listen(ctx)
}

func (cl *Client) Start() {
	cl.start()
}

func (cl *Client) Close() error {
	cl.stop()
	<-cl.done
	return cl.err
}
