package marionette_test

import (
	"reflect"
	"testing"

	"github.com/redjack/marionette"
)

func TestCell_Compare(t *testing.T) {
	cell := &marionette.Cell{SequenceID: 2}

	if cmp := cell.Compare(&marionette.Cell{SequenceID: 1}); cmp != 1 {
		t.Fatalf("unexpected comparison: %d", cmp)
	}
	if cmp := cell.Compare(&marionette.Cell{SequenceID: 2}); cmp != 0 {
		t.Fatalf("unexpected comparison: %d", cmp)
	}
	if cmp := cell.Compare(&marionette.Cell{SequenceID: 3}); cmp != -1 {
		t.Fatalf("unexpected comparison: %d", cmp)
	}
}

func TestCell_Equal(t *testing.T) {
	cell := &marionette.Cell{
		Type:       marionette.CellTypeNormal,
		Payload:    []byte("foo"),
		SequenceID: 1,
		StreamID:   3,
		UUID:       4,
		InstanceID: 5,
	}

	t.Run("OK", func(t *testing.T) {
		other := *cell
		if !cell.Equal(&other) {
			t.Fatal("expected equality")
		}
	})

	t.Run("PayloadMismatch", func(t *testing.T) {
		other := *cell
		other.Payload = []byte("bar")
		if cell.Equal(&other) {
			t.Fatal("expected inequality")
		}
	})

	t.Run("StreamIDMismatch", func(t *testing.T) {
		other := *cell
		other.StreamID = 100
		if cell.Equal(&other) {
			t.Fatal("expected inequality")
		}
	})

	t.Run("UUIDMismatch", func(t *testing.T) {
		other := *cell
		other.UUID = 100
		if cell.Equal(&other) {
			t.Fatal("expected inequality")
		}
	})

	t.Run("InstanceIDMismatch", func(t *testing.T) {
		other := *cell
		other.InstanceID = 100
		if cell.Equal(&other) {
			t.Fatal("expected inequality")
		}
	})

	t.Run("SequenceIDMismatch", func(t *testing.T) {
		other := *cell
		other.SequenceID = 100
		if cell.Equal(&other) {
			t.Fatal("expected inequality")
		}
	})
}

func TestCell_MarshalBinary(t *testing.T) {
	cell := &marionette.Cell{
		Type:       marionette.CellTypeNormal,
		Payload:    []byte("foo"),
		Length:     28,
		SequenceID: 1,
		StreamID:   3,
		UUID:       4,
		InstanceID: 5,
	}

	var other marionette.Cell
	if buf, err := cell.MarshalBinary(); err != nil {
		t.Fatal(err)
	} else if err := other.UnmarshalBinary(buf); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(cell, &other) {
		t.Fatalf("mismatch: %#v", &other)
	}
}
