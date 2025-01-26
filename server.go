package sseflow

import (
	"context"
	"github.com/bobcatalyst/flow"
	"net/http"
	"time"
)

var _ interface {
	flow.Pushable[*Message]
	context.Context
} = (*Server)(nil)

type Server struct {
	events flow.Stream[*Message]
	err    error
	done   chan struct{}
	ctx    context.Context
}

func Upgrade(w http.ResponseWriter, r *http.Request) *Server {
	fw := getFw(w)
	w.Header().Set("Content-Type", "text/event-stream;charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	fw.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	srv := &Server{
		done: make(chan struct{}),
		ctx:  ctx,
	}

	ev := srv.events.Listen(ctx)
	go func() {
		defer close(srv.done)
		defer cancel()
		for msg := range ev {
			if _, err := msg.WriteTo(w); err != nil {
				srv.err = err
				return
			}
			fw.Flush()
		}
	}()

	return srv
}

func (srv *Server) Close() error {
	srv.events.Close()
	<-srv.done
	return srv.err
}

func (srv *Server) Done() <-chan struct{} {
	return srv.done
}

func (srv *Server) Deadline() (deadline time.Time, ok bool) {
	return srv.ctx.Deadline()
}

func (srv *Server) Value(k any) any {
	return srv.ctx.Value(k)
}

func (srv *Server) Err() error {
	select {
	case <-srv.done:
		return srv.err
	default:
		return srv.ctx.Err()
	}
}

type nopFlusher struct{}

func (nopFlusher) Flush() {}

func getFw(w http.ResponseWriter) http.Flusher {
	if f, ok := w.(http.Flusher); ok {
		return f
	}
	return nopFlusher{}
}

func (srv *Server) Push(v ...*Message) { srv.events.Push(v...) }
