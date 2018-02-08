package marionette

import (
	"errors"
	"io"

	"github.com/redjack/marionette"
	"go.uber.org/zap"
)

func init() {
	marionette.RegisterPlugin("fte", "recv", Recv)
	marionette.RegisterPlugin("fte", "recv_async", RecvAsync)
}

// Recv receives data from a connection.
func Recv(fsm marionette.FSM, args []interface{}) (success bool, err error) {
	return recv(fsm, args)
}

// RecvAsync receives data from a connection without blocking.
func RecvAsync(fsm marionette.FSM, args []interface{}) (success bool, err error) {
	return recv(fsm, args)
}

func recv(fsm marionette.FSM, args []interface{}) (success bool, err error) {
	logger := marionette.Logger.With(zap.String("party", fsm.Party()))

	if len(args) < 2 {
		return false, errors.New("fte.send: not enough arguments")
	}

	regex, ok := args[0].(string)
	if !ok {
		return false, errors.New("fte.send: invalid regex argument type")
	}
	msgLen, ok := args[1].(int)
	if !ok {
		return false, errors.New("fte.send: invalid msg_len argument type")
	}

	logger.Debug("fte.recv: reading buffer")

	// Retrieve data from the connection.
	conn := fsm.Conn()
	ciphertext, err := conn.Peek(-1)
	if err != nil {
		return false, err
	} else if len(ciphertext) == 0 {
		return false, nil
	}

	logger.Debug("fte.recv: buffer read", zap.Int("n", len(ciphertext)))

	// Decode ciphertext.
	cipher, err := fsm.Cipher(regex, msgLen)
	if err != nil {
		return false, err
	}
	plaintext, remainder, err := cipher.Decrypt(ciphertext)
	if err != nil {
		return false, err
	}
	logger.Debug("fte.recv: buffer decoded", zap.Int("plaintext", len(plaintext)), zap.Int("remainder", len(remainder)))

	// Unmarshal data.
	var cell marionette.Cell
	if err := cell.UnmarshalBinary(plaintext); err != nil {
		return false, err
	}

	logger.Info("fte.recv: received cell", zap.Int("n", len(cell.Payload)))

	assert(fsm.UUID() == cell.UUID)
	initRequired := fsm.InstanceID() == 0
	fsm.SetInstanceID(cell.InstanceID)
	if initRequired || cell.StreamID == 0 {
		return false, nil
	}

	// Write plaintext to a cell decoder pipe.
	if err := fsm.StreamSet().Enqueue(&cell); err != nil {
		return false, err
	}

	// Move buffer forward by bytes consumed by the cipher.
	if _, err := conn.Seek(int64(len(ciphertext)-len(remainder)), io.SeekCurrent); err != nil {
		return false, err
	}

	return true, nil
}

func assert(condition bool) {
	if !condition {
		panic("assertion failed")
	}
}
