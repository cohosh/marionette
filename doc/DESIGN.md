Design Document
===============

This document describes the overall design of the marionette system and how
each piece works together to form the whole program.


## Overview

From a high level, marionette exists as a client proxy and a server proxy.
These proxies allow the client to send data to a port and the proxies encrypt
data which can be formatted to look like other data (e.g. HTTP traffic, FTP
data, etc). The server proxy receives this formatted data and decrypts it and
sends the original data to an endpoint (a hostport or SOCKS5 proxy).

Initially, the user must start up both the `marionette client` &
`marionette server` using to connect to each other. Both must specify the same
MAR specification document. This document declares a deterministic, finite state
machine that each side will execute. By executing the same steps in the same
sequence, both sides can encrypt/decrypt data together.

The overall data flow works as such:

1. Client application connects to the _client proxy_'s incoming port.

2. The client proxy opens a new _stream_ which is given a unique identifier.

3. The client proxy has a continually running _finite state machine_ running
   internally. When this FSM executes a directive to send data, the FSM will
   request data from any incoming stream. This data is marshalled into a _cell_
   which specifies the stream identifier, stream sequence number, document hash,
   payload size, and payload.

4. The marshalled cell is then encoded using either `fte` (Format-Transforming
   Encryption) or `tg` (Template Grammar). The `fte` encoding works by passing
   a regular expression to `libfte` and it will format the cell data to match
   the expression. The template grammar uses more specific rules encoded within
   the `tg` plugin.

5. The encrypted, formatted data is sent over the connection to the server
   proxy.

6. The server proxy is also running the same finite state machine but instead of
   sending data it expects to receive data. When the FSM executes the directive
   to receive data, it will read from the incoming connection and pass to
   `libfte` to decrypt the formatted message to produce the original cell data.

7. The cell data is passed to a _stream set_ which multiplexes many streams
   over the single connection.

8. The original payload data is then forwarded onto a hostport or to a SOCKS5
   interface.


## Components

### Client Proxy

The client proxy is a simple proxy which opens an incoming port and waits for
new connections. When a new connection is opened, a new stream within the
client stream set is created using the `Dialer`. Separate goroutines are
started to copy the incoming connection data to the stream and to copy
incoming stream data to the connection.

By default, the client proxy opens a listener on port `8079`.


### Server Proxy

The server proxy is similar to the client proxy—it opens an incoming port and
waits for new connections. The port opened by the server proxy, however, is
defined by the MAR document format used. For example, `http_simple_blocking`
specifies port `8081` in its header:

```
connection(tcp, 8081):
```

Once a connection is received, it is handed off to a SOCKS5 server, if
specified, otherwise a network connection is opened to a specified hostport.
Separate goroutines are created to copy to & from the incoming connection and
outgoing connection.


### Dialer

The marionette dialer opens a single network connection to the marionette server
on initialization. It implements a `Dial()` method with the same signature as
Go's `net.Dialer.Dial()` so it can be used interchangeably. When `Dial()` is
invoked, the dialer obtains a new stream from the associated stream set which
handles multiplexing over the single connection.

The dialer handles the continuous execution of the FSM as well to ensure that
send & receieve directives are constantly being made available for any incoming
and outgoing data.


### Listener

The marionette listener works similar to the dialer but for the server side. It
implements the `net.Listener` interface. When a listener accepts a network
connection from a dialer, it creates a new stream set to multiplex individual
streams over that connection. A new FSM is also created and continually executed
so that it is in sync with the FSM on the dialer side.


### Stream & Cells

A stream represents a logical connection between the client and server. Streams
are multiplexed over a single network connection by using cells which are
essentially packets of data with additional identifying information.

In addition to the payload, cells have several fields:

- Type: Identifies cell as a normal payload or an end-of-stream.

- StreamID: Which stream this belongs to. Used for multiplexing.

- SequenceID: Allows for ordering of cells. Each new cell gets an incrementing
  sequence number. This allows for cells to arrive out of order.

- UUID: A hash of the MAR document. This ensures a connection is executing the
  same finite state machine.

- InstanceID: A randomly generated unique identifier for the connection. This
  is generated by the initiating party (typically the `client`).

On the read side, streams contain a sorted list of received cells. If a new cell
is the next expected sequence then it is unpacked and the payload is added to
the read buffer until it is full. When the user reads from the stream, the
buffer is drained and new cell data can be added to the end.

On the write side, data is added to a write buffer by the user. When the FSM
invokes a directive to send data then it requests a certain number of bytes
from the write buffer and those are wrapped into a cell with the appropriate
type, stream id, sequence id, UUID, & instance id.

The stream maintains notification channels (`ReadNotify()` & `WriteNotify()`)
to allow the FSM to determine when new data is made available on the read or
write side, respectively.


### Stream Set

The stream set contains the set of all open streams and performs the
multiplexing of streams over a single connection on both the client side and
server side. It also generates the random stream id on stream creation.

On the read side, the stream set chooses a random stream from the set of all
streams with pending data and extracts a cell. On the write side, the stream set
inspects the cell's stream id and delegates the cell to the appropriate stream
in the set.

The stream set also maintains a write notification channel to notify the user
when any stream in the set has a write available.


### FSM

The finite state machine (FSM) is the execution engine for the MAR document
format processed in unison by the client and server. The FSM is party aware
(e.g. `client` or `server`) and only executes actions for its party.

An FSM is executed from its `start` state to its `end` state and finally
transitioned to a `dead` state when it is complete. When `Execute()` is called,
the FSM continuously calls `Next()` to attempt to transition to the next
available state. A transition is successful when all actions in the transition's
action block for the party are completed without error. An error in an action
can occur for several reasons such as not having enough data received to
decrypt.

One special error state exists when the non-initiating party first receives a
cell with the instance id. The instance id is used by  both parties to seed a
PRNG (psuedo-random number generator) which is used to "randomly" select steps
in the MAR document where applicable. Because these "random" choices need to be
the same for both the client & server, a party that receives a new instance id
restarts the FSM from the beginning and replays all its steps that have
occurred.

MAR documents can have multiple transitions from a single state:

1. Non-error transitions with a given probability.
2. Error transition.

First, all non-error transitions are collected and one is chosen based on
probability. If it can be executed successfully then the state is transitioned
to the new destination state. Next, the error transition is executed if the
non-error transition fails. If the error transition is successful then the
state is moved to the error transition's destination state.

A transition's actions are executed in order. If any returns an error then the
whole transition fails. Some actions can be conditionally executed based on 
a regex match of incoming data. Actions are simply invocations of plugin
functions.

FSMs provide a few additional services to plugins such as a variable scope as
well as FTE cipher & DFA caches for faster encryption/decryption.


### regex2dfa

The `regex2dfa` library is used by marionette to generate a state transition
table for a given regular expression. This table, however, is opaquely generated
and passed to the `libfte` library and is not inspected by marionette.

The `regex2dfa.Regex2DFA()` function wraps this library to provide safe
concurrent access to the underlying C++ library.

This library relies on `OpenFST` and `re2` for converting regular expressions
to state transition tables.


### fte

The `libfte` library's `Rank()` and `Unrank()` functions are used by marionette
to convert to & from binary data and a `big.Int` in order to generate covertext
that matches a given input regular expression.

The `fte` package also provides an encryption layer before converting to
covertext using AES-ECB for the message length, AES-CTR for the plaintext body,
and a SHA512+HMAC signature.

This library relies on `gmp` for big number support.
