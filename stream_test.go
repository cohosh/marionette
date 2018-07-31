package marionette_test

import (
	"bytes"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/redjack/marionette"
)

func TestStream_ID(t *testing.T) {
	stream := marionette.NewStream(100)
	if stream.ID() != 100 {
		t.Fatalf("unexpected id: %d", stream.ID())
	}
}

func TestStream_Enqueue(t *testing.T) {
	t.Run("InOrder", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 0, Payload: []byte("hello")}); err != nil {
			t.Fatal(err)
		} else if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 1, Payload: []byte("goodbye")}); err != nil {
			t.Fatal(err)
		}

		// Check read buffer length.
		if n := stream.ReadBufferLen(); n != 12 {
			t.Fatalf("unexpected read buffer length: %d", n)
		}

		// Read first chunk.
		buf := make([]byte, 10)
		if n, err := stream.Read(buf); err != nil {
			t.Fatal(err)
		} else if n != 10 {
			t.Fatalf("unexpected n: %d", n)
		} else if string(buf) != "hellogoodb" {
			t.Fatalf("unexpected data: %s", buf)
		}

		// Read remainder.
		if n, err := stream.Read(buf); err != nil {
			t.Fatal(err)
		} else if n != 2 {
			t.Fatalf("unexpected n: %d", n)
		} else if string(buf[:n]) != "ye" {
			t.Fatalf("unexpected data: %s", buf)
		}
	})

	t.Run("OutOfOrder", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 1, Payload: []byte("bar")}); err != nil {
			t.Fatal(err)
		} else if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 0, Payload: []byte("foo")}); err != nil {
			t.Fatal(err)
		} else if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 2, Payload: []byte("baz")}); err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, 9)
		if n, err := stream.Read(buf); err != nil {
			t.Fatal(err)
		} else if n != 9 {
			t.Fatalf("unexpected n: %d", n)
		} else if string(buf) != "foobarbaz" {
			t.Fatalf("unexpected data: %s", buf)
		}
	})

	t.Run("DuplicateCell", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 1, Payload: []byte("bar")}); err != nil {
			t.Fatal(err)
		} else if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 0, Payload: []byte("foo")}); err != nil {
			t.Fatal(err)
		} else if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 1, Payload: []byte("bar")}); err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, 9)
		if n, err := stream.Read(buf); err != nil {
			t.Fatal(err)
		} else if n != 6 {
			t.Fatalf("unexpected n: %d", n)
		} else if string(buf[:n]) != "foobar" {
			t.Fatalf("unexpected data: %s", buf)
		}
	})

	t.Run("FullBuffer", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		// Generate data that is 2/3's of the buffer size.
		// This will require the second cell to go on the read queue.
		data := bytes.Repeat([]byte("x"), ((marionette.MaxCellLength * 2) / 3))
		if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 0, Payload: data}); err != nil {
			t.Fatal(err)
		} else if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 1, Payload: data}); err != nil {
			t.Fatal(err)
		} else if err := stream.CloseRead(); err != nil {
			t.Fatal(err)
		}

		// Read all the data off.
		exp := bytes.Repeat([]byte("x"), len(data)*2)
		if buf, err := ioutil.ReadAll(stream); err != nil {
			t.Fatal(err)
		} else if !bytes.Equal(buf, exp) {
			t.Fatalf("unexpected read: %d <=> %d", len(buf), len(exp))
		}
	})

	t.Run("PendingRead", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		// Slowly inject data in a separate goroutine.
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 0, Payload: []byte("foo")}); err != nil {
				t.Fatal(err)
			}

			time.Sleep(100 * time.Millisecond)
			if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 2, Payload: []byte("baz")}); err != nil {
				t.Fatal(err)
			}

			time.Sleep(100 * time.Millisecond)
			if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 1, Payload: []byte("bar")}); err != nil {
				t.Fatal(err)
			}

			time.Sleep(100 * time.Millisecond)
			if err := stream.CloseRead(); err != nil {
				t.Fatal(err)
			}
		}()

		// Slurp all data while data is being injected.
		if buf, err := ioutil.ReadAll(stream); err != nil {
			t.Fatal(err)
		} else if string(buf) != "foobarbaz" {
			t.Fatalf("unexpected data: %s", buf)
		}

		// Ensure goroutine closes.
		wg.Wait()
	})
}

func TestStream_Dequeue(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		// Write data to the stream.
		if n, err := stream.Write([]byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit")); err != nil {
			t.Fatal(err)
		} else if n != 55 {
			t.Fatalf("unexpected n: %d", n)
		}

		// Check write buffer length.
		if n := stream.WriteBufferLen(); n != 55 {
			t.Fatalf("unexpected write buffer length: %d", n)
		}

		// Dequeue from stream as a cell.
		if diff := cmp.Diff(stream.Dequeue(0), &marionette.Cell{
			Type:       marionette.CellTypeNormal,
			Length:     marionette.CellHeaderSize + 55,
			StreamID:   100,
			SequenceID: 0,
			Payload:    []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit"),
		}); diff != "" {
			t.Fatal(diff)
		}

		// Dequeuing again should return an empty cell.
		if diff := cmp.Diff(stream.Dequeue(0), &marionette.Cell{
			Type:       marionette.CellTypeNormal,
			Length:     marionette.CellHeaderSize,
			StreamID:   100,
			SequenceID: 1,
		}); diff != "" {
			t.Fatal(diff)
		}

		// Closing and dequeuing should return an end-of-stream cell.
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		} else if diff := cmp.Diff(stream.Dequeue(0), &marionette.Cell{
			Type:       marionette.CellTypeEOS,
			Length:     marionette.CellHeaderSize,
			StreamID:   100,
			SequenceID: 2,
		}); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Padded", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		// Write data to the stream.
		if n, err := stream.Write([]byte("Lorem ipsum")); err != nil {
			t.Fatal(err)
		} else if n != 11 {
			t.Fatalf("unexpected n: %d", n)
		}

		// Dequeue from stream as a cell with padding.
		if diff := cmp.Diff(stream.Dequeue(marionette.CellHeaderSize+20), &marionette.Cell{
			Type:       marionette.CellTypeNormal,
			Length:     marionette.CellHeaderSize + 20,
			StreamID:   100,
			SequenceID: 0,
			Payload:    []byte("Lorem ipsum"),
		}); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("PaddingTooLarge", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		// Write data to the stream.
		if n, err := stream.Write([]byte("Lorem ipsum")); err != nil {
			t.Fatal(err)
		} else if n != 11 {
			t.Fatalf("unexpected n: %d", n)
		}

		// The cell length should be capped at the max.
		if diff := cmp.Diff(stream.Dequeue(marionette.MaxCellLength+20), &marionette.Cell{
			Type:       marionette.CellTypeNormal,
			Length:     marionette.MaxCellLength,
			StreamID:   100,
			SequenceID: 0,
			Payload:    []byte("Lorem ipsum"),
		}); diff != "" {
			t.Fatal(diff)
		}
	})
}

func TestStream_Write(t *testing.T) {
	t.Run("ErrStreamClosed", func(t *testing.T) {
		stream := marionette.NewStream(100)
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		} else if _, err := stream.Write([]byte("foo")); err != marionette.ErrStreamClosed {
			t.Fatal(err)
		}
	})

	t.Run("ErrWriteTooLarge", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		data := bytes.Repeat([]byte("x"), marionette.MaxCellLength+1)
		if _, err := stream.Write(data); err != marionette.ErrWriteTooLarge {
			t.Fatal(err)
		}
	})

	t.Run("FullBuffer", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		// Generate data that is 2/3's of the buffer size.
		data := bytes.Repeat([]byte("x"), ((marionette.MaxCellLength * 2) / 3))

		// First write should succeed.
		if _, err := stream.Write(data); err != nil {
			t.Fatal(err)
		}

		// Slowly inject data in a separate goroutine.
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			stream.Dequeue(0)
		}()

		// Second write should wait until data is dequeued from the buffer.
		if _, err := stream.Write(data); err != nil {
			t.Fatal(err)
		}
	})
}

func TestStream_ReadNotify(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		notify := stream.ReadNotify()
		if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 0, Payload: []byte("foo")}); err != nil {
			t.Fatal(err)
		}

		select {
		case <-notify:
		default:
			t.Fatal("expected notification after in-order enqueue")
		}
	})

	t.Run("OutOfOrder", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		notify := stream.ReadNotify()
		if err := stream.Enqueue(&marionette.Cell{StreamID: 100, SequenceID: 1, Payload: []byte("bar")}); err != nil {
			t.Fatal(err)
		}

		select {
		case <-notify:
			t.Fatal("unexpected notification after out-of-order enqueue")
		default:
		}
	})
}

func TestStream_WriteNotify(t *testing.T) {
	t.Run("AfterWrite", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		notify := stream.WriteNotify()
		if _, err := stream.Write([]byte("foo")); err != nil {
			t.Fatal(err)
		}

		select {
		case <-notify:
		default:
			t.Fatal("expected notification after successful write")
		}
	})

	t.Run("AfterDequeue", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		if _, err := stream.Write([]byte("foo")); err != nil {
			t.Fatal(err)
		}

		notify := stream.WriteNotify()
		stream.Dequeue(0)

		select {
		case <-notify:
		default:
			t.Fatal("expected notification after successful dequeue")
		}
	})

	t.Run("AfterEmptyDequeue", func(t *testing.T) {
		stream := marionette.NewStream(100)
		defer stream.Close()

		notify := stream.WriteNotify()
		stream.Dequeue(0)

		select {
		case <-notify:
			t.Fatal("unexpected notification after empty dequeue")
		default:
		}
	})

	t.Run("AfterClosedDequeue", func(t *testing.T) {
		stream := marionette.NewStream(100)
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		}

		notify := stream.WriteNotify()
		stream.Dequeue(0)

		select {
		case <-notify:
			t.Fatal("unexpected notification after closed dequeue")
		default:
		}
	})
}

func TestStream_Closed(t *testing.T) {
	stream := marionette.NewStream(100)
	if stream.Closed() {
		t.Fatal("expected open")
	} else if err := stream.CloseWrite(); err != nil {
		t.Fatal(err)
	} else if err := stream.CloseRead(); err != nil {
		t.Fatal(err)
	} else if !stream.Closed() {
		t.Fatal("expected closed")
	}
}

func TestStream_LocalAddr(t *testing.T) {
	stream := marionette.NewStream(100)
	defer stream.Close()
	if stream.LocalAddr() != nil {
		t.Fatal("expected nil addr")
	}
}

func TestStream_RemoteAddr(t *testing.T) {
	stream := marionette.NewStream(100)
	defer stream.Close()
	if stream.RemoteAddr() != nil {
		t.Fatal("expected nil addr")
	}
}

func TestStream_SetDeadline(t *testing.T) {
	stream := marionette.NewStream(100)
	defer stream.Close()
	if err := stream.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
}

func TestStream_SetReadDeadline(t *testing.T) {
	stream := marionette.NewStream(100)
	defer stream.Close()
	if err := stream.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
}

func TestStream_SetWriteDeadline(t *testing.T) {
	stream := marionette.NewStream(100)
	defer stream.Close()
	if err := stream.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
}
