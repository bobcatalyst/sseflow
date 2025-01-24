package sseflow

import (
	"context"
	"github.com/bobcatalyst/flow"
	"net/http"
)

type Server struct {
	events flow.Stream[*Message]
	err    error
	done   chan struct{}
}

func Upgrade(w http.ResponseWriter, r *http.Request) *Server {
	fw := getFw(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ctx, cancel := context.WithCancel(r.Context())
	srv := &Server{
		done: make(chan struct{}),
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

type nopFlusher struct{}

func (nopFlusher) Flush() {}

func getFw(w http.ResponseWriter) http.Flusher {
	if f, ok := w.(http.Flusher); ok {
		return f
	}
	return nopFlusher{}
}

func (srv *Server) Push(v ...*Message) { srv.events.Push(v...) }
