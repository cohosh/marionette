package marionette

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/redjack/marionette/mar"
	"go.uber.org/zap"
)

var (
	// ErrDialerClosed is returned when trying to operate on a closed dialer.
	ErrDialerClosed = errors.New("marionette: dialer closed")
)

// Dialer represents a client-side dialer that communicates over the marionette protocol.
type Dialer struct {
	mu        sync.RWMutex
	addr      string
	doc       *mar.Document
	fsm       FSM
	streamSet *StreamSet

	ctx    context.Context
	cancel func()

	closed bool
	wg     sync.WaitGroup

	// Underlying net.Dialer used for net connection.
	Dialer net.Dialer
}

// NewDialer returns a new instance of Dialer.
func NewDialer(doc *mar.Document, addr string, streamSet *StreamSet) *Dialer {
	// Run execution in a separate goroutine.
	d := &Dialer{
		addr:      addr,
		doc:       doc,
		streamSet: streamSet,
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	return d
}

// Open initializes the underlying connection.
func (d *Dialer) Open() error {
	conn, err := d.Dialer.DialContext(d.ctx, d.doc.Transport, net.JoinHostPort(d.addr, d.doc.Port))
	if err != nil {
		return err
	}
	d.fsm = NewFSM(d.doc, d.addr, PartyClient, conn, d.streamSet)

	d.wg.Add(1)
	go func() { defer d.wg.Done(); d.execute() }()
	return nil
}

// Close stops the dialer and its underlying connections.
func (d *Dialer) Close() error {
	err := d.close()
	d.wg.Wait()
	return err
}

func (d *Dialer) close() (err error) {
	d.mu.Lock()
	d.closed = true
	err = d.fsm.Close()
	d.mu.Unlock()

	d.cancel()
	return err
}

// Closed returns true if the dialer has been closed.
func (d *Dialer) Closed() bool {
	d.mu.RLock()
	closed := d.closed
	d.mu.RUnlock()
	return closed
}

// Dial returns a new stream from the dialer.
func (d *Dialer) Dial() (net.Conn, error) {
	if d.Closed() {
		return nil, ErrDialerClosed
	}
	return d.streamSet.Create(), nil
}

func (d *Dialer) execute() {
	defer d.close()

	for !d.Closed() {
		if err := d.fsm.Execute(d.ctx); err == ErrStreamClosed {
			continue
		} else if err != nil {
			Logger.Debug("dialer error", zap.Error(err))
			return
		}
		d.fsm.Reset()
	}
}
