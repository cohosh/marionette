package marionette_test

import (
	"io/ioutil"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redjack/marionette"
)

func TestStreamSet_Create(t *testing.T) {
	ss := marionette.NewStreamSet()
	defer ss.Close()

	var callbackInvoked bool
	ss.OnNewStream = func(s *marionette.Stream) {
		callbackInvoked = true
	}
	if stream := ss.Create(); stream == nil {
		t.Fatal("expected stream")
	} else if stream.ID() == 0 {
		t.Fatal("expected stream id")
	} else if !callbackInvoked {
		t.Fatal("expected callback invocation")
	}
}

func TestStreamSet_Enqueue(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		ss := marionette.NewStreamSet()
		defer ss.Close()

		var callbackInvoked bool
		ss.OnNewStream = func(s *marionette.Stream) {
			if s.ID() != 100 && s.ID() != 101 {
				t.Fatalf("unexpected stream id (callback): %d", s.ID())
			}
			callbackInvoked = true
		}

		if err := ss.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 0, Payload: []byte("foo")}); err != nil {
			t.Fatal(err)
		} else if err := ss.Enqueue(&marionette.Cell{StreamID: 101, SequenceID: 0, Payload: []byte("bar")}); err != nil {
			t.Fatal(err)
		} else if err := ss.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 1, Payload: []byte("baz")}); err != nil {
			t.Fatal(err)
		}

		if stream := ss.Stream(100); stream == nil {
			t.Fatal("expected stream")
		} else if err := stream.CloseRead(); err != nil {
			t.Fatal(err)
		} else if buf, err := ioutil.ReadAll(stream); err != nil {
			t.Fatal(err)
		} else if string(buf) != "foobaz" {
			t.Fatalf("unexpected stream data: %s", buf)
		} else if !callbackInvoked {
			t.Fatal("expected callback invocation")
		}

		if err := ss.Close(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("EmptyCell", func(t *testing.T) {
		ss := marionette.NewStreamSet()
		defer ss.Close()
		ss.OnNewStream = func(s *marionette.Stream) {
			t.Fatal("unexpected callback invocation")
		}

		if err := ss.Enqueue(&marionette.Cell{StreamID: 0}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestStreamSet_Dequeue(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		ss := marionette.NewStreamSet()
		defer ss.Close()

		// Create two streams.
		stream0, stream1 := ss.Create(), ss.Create()

		// Write to both.
		if _, err := stream0.Write([]byte("foo")); err != nil {
			t.Fatal(err)
		} else if _, err := stream1.Write([]byte("bar")); err != nil {
			t.Fatal(err)
		}

		// Dequeue twice. Map sorting is unordered so we must sort afterward.
		cells := []*marionette.Cell{ss.Dequeue(0), ss.Dequeue(0)}
		sort.Slice(cells, func(i, j int) bool { return cells[i].StreamID < cells[j].StreamID })

		exp := []*marionette.Cell{
			{Type: marionette.CellTypeNormal, StreamID: stream0.ID(), SequenceID: 0, Payload: []byte("foo"), Length: 28},
			{Type: marionette.CellTypeNormal, StreamID: stream1.ID(), SequenceID: 0, Payload: []byte("bar"), Length: 28},
		}
		sort.Slice(exp, func(i, j int) bool { return exp[i].StreamID < exp[j].StreamID })

		if diff := cmp.Diff(exp, cells); diff != "" {
			t.Fatal(diff)
		}

		// Dequeuing with no data should return nil.
		if ss.Dequeue(0) != nil {
			t.Fatal("expected no cell")
		}

		// Closing a stream should cause an end-of-stream dequeue.
		if err := stream0.Close(); err != nil {
			t.Fatal(err)
		} else if diff := cmp.Diff(ss.Dequeue(0), &marionette.Cell{Type: marionette.CellTypeEOS, StreamID: stream0.ID(), SequenceID: 1, Length: 25}); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		ss := marionette.NewStreamSet()
		defer ss.Close()
		if ss.Dequeue(0) != nil {
			t.Fatal("expected no cell")
		}
	})
}
