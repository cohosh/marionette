package marionette

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"time"

	"github.com/redjack/marionette/fte"
	"github.com/redjack/marionette/mar"
	"go.uber.org/zap"
)

var (
	// ErrNoTransition is returned from FSM.Next() when no transition is available.
	ErrNoTransition = errors.New("no matching transition")

	ErrUUIDMismatch = errors.New("uuid mismatch")
)

// FSM represents an interface for the Marionette state machine.
type FSM interface {
	// Document & FSM identifiers.
	UUID() int
	SetInstanceID(int)
	InstanceID() int

	// Party & networking.
	Party() string
	Host() string
	Port() int

	// The current state in the FSM.
	State() string

	// Returns true if State() == 'dead'
	Dead() bool

	// Moves to the next available state.
	// Returns ErrNoTransition if there is no state to move to.
	Next(ctx context.Context) error

	// Moves through the entire state machine until it reaches 'dead' state.
	Execute(ctx context.Context) error

	// Restarts the FSM so it can be reused.
	Reset()

	// Returns an FTE cipher from the cache or creates a new one.
	Cipher(regex string, msgLen int) (Cipher, error)

	// Returns the network connection attached to the FSM.
	Conn() *BufferedConn

	// Listen opens a new listener to accept data and drains into the buffer.
	Listen() (int, error)

	// Returns the stream set attached to the FSM.
	StreamSet() *StreamSet

	// Sets and retrieves key/values from the FSM.
	SetVar(key string, value interface{})
	Var(key string) interface{}

	// Returns a copy of the FSM with a different format.
	Clone(doc *mar.Document) FSM
}

// Ensure implementation implements interface.
var _ FSM = &fsm{}

// fsm is the default implementation of the FSM.
type fsm struct {
	doc   *mar.Document
	host  string
	party string

	conn      *BufferedConn
	streamSet *StreamSet
	listeners []net.Listener

	state string
	stepN int
	rand  *rand.Rand

	// Lookup of transitions by src state.
	transitions map[string][]*mar.Transition

	vars    map[string]interface{}
	ciphers map[cipherKey]*fte.Cipher

	// Set by the first sender and used to seed PRNG.
	instanceID int
}

// NewFSM returns a new FSM. If party is the first sender then the instance id is set.
func NewFSM(doc *mar.Document, host, party string, conn net.Conn, streamSet *StreamSet) FSM {
	fsm := &fsm{
		doc:       doc,
		host:      host,
		party:     party,
		conn:      NewBufferedConn(conn, MaxCellLength),
		streamSet: streamSet,
	}
	fsm.Reset()
	fsm.buildTransitions()
	fsm.initFirstSender()
	return fsm
}

func (fsm *fsm) buildTransitions() {
	fsm.transitions = make(map[string][]*mar.Transition)
	for _, t := range fsm.doc.Transitions {
		fsm.transitions[t.Source] = append(fsm.transitions[t.Source], t)
	}
}

func (fsm *fsm) initFirstSender() {
	if fsm.party != fsm.doc.FirstSender() {
		return
	}
	fsm.instanceID = int(rand.Int31())
	fsm.rand = rand.New(rand.NewSource(int64(fsm.instanceID)))
}

func (fsm *fsm) Reset() {
	fsm.state = "start"
	fsm.vars = make(map[string]interface{})

	for _, c := range fsm.ciphers {
		if err := c.Close(); err != nil {
			fsm.logger().Error("cannot close cipher", zap.Error(err))
		}
	}
	fsm.ciphers = make(map[cipherKey]*fte.Cipher)

	for _, ln := range fsm.listeners {
		if err := ln.Close(); err != nil {
			fsm.logger().Error("cannot close listener", zap.Error(err))
		}
	}
	fsm.listeners = nil
}

// UUID returns the computed MAR document UUID.
func (fsm *fsm) UUID() int { return fsm.doc.UUID }

// InstanceID returns the ID for this specific FSM.
func (fsm *fsm) InstanceID() int { return fsm.instanceID }

// SetInstanceID sets the ID for the FSM.
func (fsm *fsm) SetInstanceID(id int) { fsm.instanceID = id }

// State returns the current state of the FSM.
func (fsm *fsm) State() string { return fsm.state }

// Conn returns the connection the FSM was initialized with.
func (fsm *fsm) Conn() *BufferedConn { return fsm.conn }

// StreamSet returns the stream set the FSM was initialized with.
func (fsm *fsm) StreamSet() *StreamSet { return fsm.streamSet }

// Host returns the hostname the FSM was initialized with.
func (fsm *fsm) Host() string { return fsm.host }

// Party returns "client" or "server" depending on who is initializing the FSM.
func (fsm *fsm) Party() string { return fsm.party }

// Port returns the port from the underlying document.
// If port is a named port then it is looked up in the local variables.
func (fsm *fsm) Port() int {
	if port, err := strconv.Atoi(fsm.doc.Port); err == nil {
		return port
	}

	if v := fsm.Var(fsm.doc.Port); v != nil {
		s, _ := v.(string)
		port, _ := strconv.Atoi(s)
		return port
	}

	return 0
}

// Dead returns true when the FSM is complete.
func (fsm *fsm) Dead() bool { return fsm.state == "dead" }

// Execute runs the the FSM to completion.
func (fsm *fsm) Execute(ctx context.Context) error {
	fsm.Reset()

	for !fsm.Dead() {
		if err := fsm.Next(ctx); err == ErrNoTransition {
			time.Sleep(100 * time.Millisecond)
			continue
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (fsm *fsm) Next(ctx context.Context) (err error) {
	logger := fsm.logger()
	logger.Debug("fsm: Next()", zap.String("state", fsm.state))

	// Generate a new PRNG once we have an instance ID.
	if err := fsm.init(); err != nil {
		logger.Debug("fsm: cannot initialize fsm", zap.Error(err))
		return err
	}

	// If we have a successful transition, update our state info.
	// Exit if no transitions were successful.
	if nextState, err := fsm.next(); err != nil {
		logger.Debug("fsm: cannot move to next state")
		return err
	} else if nextState == "" {
		logger.Debug("fsm: no transition available")
		return ErrNoTransition
	} else {
		fsm.stepN += 1
		fsm.state = nextState
		logger.Debug("fsm: transition successful", zap.String("state", fsm.state), zap.Int("step", fsm.stepN))
	}

	return nil
}

func (fsm *fsm) next() (nextState string, err error) {
	logger := fsm.logger()

	// Find all possible transitions from the current state.
	transitions := mar.FilterTransitionsBySource(fsm.doc.Transitions, fsm.state)
	errorTransitions := mar.FilterErrorTransitions(transitions)

	// Then filter by PRNG (if available) or return all (if unavailable).
	transitions = mar.FilterNonErrorTransitions(transitions)
	transitions = mar.ChooseTransitions(transitions, fsm.rand)
	assert(len(transitions) > 0)

	logger.Debug("fsm: evaluating transitions", zap.Int("n", len(transitions)))

	// Add error transitions back in after selection.
	transitions = append(transitions, errorTransitions...)

	// Attempt each possible transition.
	for _, transition := range transitions {
		logger.Debug("fsm: evaluating transition", zap.String("src", transition.Source), zap.String("dest", transition.Destination))

		// If there's no action block then move to the next state.
		if transition.ActionBlock == "NULL" {
			logger.Debug("fsm: no action block, matched")
			return transition.Destination, nil
		}

		// Find all actions for this destination and current party.
		blk := fsm.doc.ActionBlock(transition.ActionBlock)
		if blk == nil {
			return "", fmt.Errorf("fsm.Next(): action block not found: %q", transition.ActionBlock)
		}
		actions := mar.FilterActionsByParty(blk.Actions, fsm.party)

		// Attempt to execute each action.
		logger.Debug("fsm: evaluating action block", zap.String("name", transition.ActionBlock), zap.Int("actions", len(actions)))
		if matched, err := fsm.evalActions(actions); err != nil {
			return "", err
		} else if matched {
			return transition.Destination, nil
		}
	}
	return "", nil
}

// init initializes the PRNG if we now have a instance id.
func (fsm *fsm) init() (err error) {
	if fsm.rand != nil || fsm.instanceID == 0 {
		return nil
	}

	logger := fsm.logger()
	logger.Debug("fsm: initializing fsm")

	// Create new PRNG.
	fsm.rand = rand.New(rand.NewSource(int64(fsm.instanceID)))

	// Restart FSM from the beginning and iterate until the current step.
	fsm.state = "start"
	for i := 0; i < fsm.stepN; i++ {
		fsm.state, err = fsm.next()
		if err != nil {
			return err
		}
		assert(fsm.state != "")
	}
	return nil
}

func (fsm *fsm) evalActions(actions []*mar.Action) (bool, error) {
	logger := fsm.logger()

	if len(actions) == 0 {
		logger.Debug("fsm: no actions, matched")
		return true, nil
	}

	for _, action := range actions {
		logger.Debug("fsm: evaluating action", zap.String("name", action.Module+"."+action.Method), zap.String("regex", action.Regex))

		// If there is no matching regex then simply evaluate action.
		if action.Regex != "" {
			// Compile regex.
			re, err := regexp.Compile(action.Regex)
			if err != nil {
				return false, err
			}

			// Only evaluate action if buffer matches.
			buf, err := fsm.conn.Peek(-1)
			if err != nil {
				return false, err
			} else if !re.Match(buf) {
				logger.Debug("fsm: no regex match, skipping")
				continue
			}
		}

		if success, err := fsm.evalAction(action); err != nil {
			return false, err
		} else if success {
			return true, nil
		}
		continue
	}

	return false, nil
}

func (fsm *fsm) evalAction(action *mar.Action) (bool, error) {
	fn := FindPlugin(action.Module, action.Method)
	if fn == nil {
		return false, fmt.Errorf("fsm.evalAction(): action not found: %s", action.Name())
	}
	fsm.logger().Debug("fsm: execute plugin", zap.String("name", action.Name()))
	return fn(fsm, action.ArgValues()...)
}

func (fsm *fsm) Var(key string) interface{} {
	switch key {
	case "model_instance_id":
		return fsm.InstanceID
	case "model_uuid":
		return fsm.doc.UUID
	case "party":
		return fsm.party
	default:
		return fsm.vars[key]
	}
}

func (fsm *fsm) SetVar(key string, value interface{}) {
	fsm.vars[key] = value
}

// Cipher returns a cipher with the given settings.
// If no cipher exists then a new one is created and returned.
func (fsm *fsm) Cipher(regex string, msgLen int) (_ Cipher, err error) {
	key := cipherKey{regex, msgLen}
	cipher := fsm.ciphers[key]
	if cipher != nil {
		return cipher, nil
	}

	cipher = fte.NewCipher(regex)
	if err := cipher.Open(); err != nil {
		return nil, err
	}

	fsm.ciphers[key] = cipher
	return cipher, nil
}

func (fsm *fsm) Listen() (port int, err error) {
	ln, err := net.Listen("tcp", fsm.host)
	if err != nil {
		return 0, err
	}
	fsm.listeners = append(fsm.listeners, ln)

	go fsm.handleListener(ln)

	return ln.Addr().(*net.TCPAddr).Port, nil
}

func (fsm *fsm) handleListener(ln net.Listener) {
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go fsm.handleConn(conn)
	}
}

func (fsm *fsm) handleConn(conn net.Conn) {
	panic("TODO: Drain connection into buffer?")
}

func (f *fsm) Clone(doc *mar.Document) FSM {
	other := &fsm{
		doc:   doc,
		host:  f.host,
		party: f.party,

		conn:      f.conn,
		streamSet: f.streamSet,

		instanceID: f.instanceID,
		vars:       f.vars,
	}
	other.Reset()
	other.buildTransitions()
	other.initFirstSender()
	return other
}

func (fsm *fsm) logger() *zap.Logger {
	return Logger.With(zap.String("party", fsm.party))
}

type cipherKey struct {
	regex  string
	msgLen int
}
