package sseflow

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"slices"
	"strconv"
	"strings"
)

type Message struct {
	Comment []string
	ID      *string
	Retry   uint
	Event   string
	Data    string
}

var (
	anyCharReplacer = strings.NewReplacer(
		"\n", "\\n",
		"\r", "\\r",
	)
	commentReplacer = anyCharReplacer
	dataReplacer    = strings.NewReplacer(
		"\n", "\ndata:",
		"\r", "\\r",
	)
	nopReplacer = strings.NewReplacer()
)

func (msg *Message) Write(b []byte) (int, error) {
	name, value, ok := bytes.Cut(b, []byte{':'})
	if ok && len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}

	switch v := string(value); string(name) {
	case "":
		msg.Comment = append(msg.Comment, v)
	case "id":
		msg.ID = &v
	case "retry":
		if i, err := strconv.ParseUint(v, 10, 32); err != nil {
			return 0, err
		} else if i > 0 {
			msg.Retry = uint(i)
		}
	case "event":
		if msg.Event = v; msg.Event == "" {
			msg.Event = "message"
		}
	case "data":
		nl := ""
		if len(msg.Data) > 0 {
			nl = "\n"
		}
		msg.Data += nl + v
	}
	return len(b), nil
}

func (msg *Message) WriteTo(w io.Writer) (n int64, _ error) {
	for writer := range msg.writers() {
		nn, err := writer.WriteTo(w)
		n += nn
		if err != nil {
			return n, err
		}
	}
	nn, err := w.Write([]byte{'\n'})
	n += int64(nn)
	return n, err
}

func (msg *Message) writers() iter.Seq[io.WriterTo] {
	return func(yield func(to io.WriterTo) bool) {
		if slices.ContainsFunc(msg.Comment, func(comment string) bool {
			return !yield(&command{value: comment, replacer: commentReplacer})
		}) {
			return
		}

		if !yieldIf(msg.ID != nil, yield, func() io.WriterTo {
			return &command{
				name:     "id",
				value:    *msg.ID,
				replacer: anyCharReplacer,
			}
		}) {
			return
		}

		if !yieldIf(msg.Retry > 0, yield, func() io.WriterTo {
			return &command{
				name:     "retry",
				value:    strconv.FormatUint(uint64(msg.Retry), 10),
				replacer: nopReplacer,
			}
		}) {
			return
		}

		if !yieldIf(len(msg.Event) > 0, yield, func() io.WriterTo {
			return &command{
				name:     "event",
				value:    msg.Event,
				replacer: anyCharReplacer,
			}
		}) {
			return
		}

		if !yieldIf(len(msg.Data) > 0, yield, func() io.WriterTo {
			return &command{
				name:     "data",
				value:    msg.Data,
				replacer: dataReplacer,
			}
		}) {
			return
		}
	}
}

func yieldIf(b bool, yield func(io.WriterTo) bool, cons func() io.WriterTo) bool {
	if b {
		return yield(cons())
	}
	return true
}

type command struct {
	name     string
	value    string
	replacer *strings.Replacer
}

func (cmd *command) WriteTo(w io.Writer) (n int64, err error) {
	div := ""
	if len(cmd.value) > 0 {
		div = ":"
		if cmd.value[0] == ' ' {
			div += " "
		}
	} else if len(cmd.name) == 0 {
		div = ":"
	}

	nn, err := fmt.Fprintf(w, "%s%s", cmd.name, div)
	n += int64(nn)
	if err != nil {
		return n, err
	}

	nn, err = cmd.replacer.WriteString(w, cmd.value)
	n += int64(nn)
	if err != nil {
		return n, err
	}

	nn, err = w.Write([]byte{'\n'})
	n += int64(nn)
	return n, err
}
