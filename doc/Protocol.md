Protocols and how to design them for obfuscation strategies
======

## Overview

The MAR language specifies a state machine that Marionette uses to obfuscate
communication between the client and server. Because the language is
deterministic, the client and server can both independently execute the same
MAR document and achieve the same result.

A list of pre-built MAR files can be found in the `mar/formats` folder of the
`marionette` repository.


## Structure

The MAR document contains 3 main parts:

1. Header
2. Transitions
3. Action Blocks


### Header

The header specifies the transport and port on the first line in the following
format:

```
connection(TRANSPORT, PORT):
```

- `TRANSPORT` must be `tcp` or `udp`.
- `PORT` must be a number in the valid TCP range (1-65535).

The `PORT` may also be a variable name that can be used by `channel.bind()` &
`model.spawn()`. An example of this can found in the `ftp_simple_blocking` &
`ftp_pasv_transfer` documents.


### Transitions

The lines that follow specify the FSM state machine transitions in the following
format:

```
SOURCE DESTINATION ACTION_BLOCK PROBABILITY
```

- `SOURCE` is the state to transition from.
- `DESTINATION` is the state to transition to.
- `ACTION_BLOCK` is the name of the action block to execute.
- `PROBABILITY` is the probability of execution, between `0.0` and `1.0`.  This format location can also take on the value `error`.

The state machine always starts in the `start` state and document should always
end up in the `end` state. A special `dead` state is added after the `end` state
by the FSM evaluator. The `action` state name is not allowed.

The probability for transitions that share a common source state should add up
to `1.0`.

The error transition is an epsilon state which is activated when none of the other states can be transitioned to.

### Action Blocks

Action blocks specify a series of actions to perform by either the client or
server when a transition is made. If any actions fail then the transition is
retried.

Action blocks are specified as follows:

```
action NAME:
  ACTIONS
```

- `NAME` is the name used by the transition.
- `ACTIONS` is zero or more lines of actions to be executed.

### Hello World (http\_simple\_blocking) Protocol

Given the above description of how to construct a mar file in general, let's look at one in particular.

#### http\_simple\_blocking.mar
```
connection(tcp, 8081):
  start      upstream   NULL     1.0
  upstream   downstream http_get 1.0
  downstream end        http_ok  1.0

action http_get:
  client fte.send("^GET\ \/([a-zA-Z0-9\.\/]*) HTTP/1\.1\r\n\r\n$", 128)

action http_ok:
  server fte.send("^HTTP/1\.1\ 200 OK\r\nContent-Type:\ ([a-zA-Z0-9]+)\r\n\r\n\C*$", 128)
```
This is the protocol `http_simple_blocking`.  The format file is called `http_simple_blocking.mar` and is found in the `mar/formats` directory.

#### Header Line

We can see that the server will open its connection on tcp port 8081, based on the header line.

#### Transitions
The next three lines are the transitions, which define the finite state machine. Let's go through it in detail.

These first line is the transition from the `start` state to the `upstream` state.  There is no action taken for this transition, which is what `NULL` means in the third column.  This transition happens with a probability 1.0.  Therefore, this is transition is certain to happen.  The only real purpose of this transition, is to get the system out of the `start` state and into the `upstream` state where something interesting can occur.

The second line is the transition from the `upstream` state to the `downstream` state.  This transition contains an action `http_get`.

Finally, the third line is a transition from the `downstream` state to the `end` state.  This transition contains an action `http_ok`.

There is a hidden fourth line, which occurs in all protocols that looks like this:

```
  end      start    NULL     1.0
```
This line closes the loop on the protocol, and allows the system to continue looping as long as there is data to transmit.  The system assumes that this line always exists, therefore we do not write it.

#### Actions
There are two actions.

The first is `http_get`:

```
action http_get:
  client fte.send("^GET\ \/([a-zA-Z0-9\.\/]*) HTTP/1\.1\r\n\r\n$", 128)
```
This line has two parts.  First, the `client` keyword means that the command on this line is to be considered from the client's perspective.  The second part is the `fte.send` command.  This command, provided by the `fte` plugin, translates a portion of the Marionette information into a tcp packet whose data satisfy the regular expression in the first argument.  The maximum length of the data is the second argument.  This command is interpreted on the server side as a command to receive a packet that satisfies the regex, and to decode it accordingly.

The second action is `http_ok`:

```
action http_ok:
  server fte.send("^HTTP/1\.1\ 200 OK\r\nContent-Type:\ ([a-zA-Z0-9]+)\r\n\r\n\C*$", 128)
```

This action is identical to the action `http_get`, but it uses `fte.send` initiated on the server side.  This allows the server to send its own information, in the form of the given regex, allowing the response to be decoded on the client side.

This handshake completes the connection between the client and the server, allowing information transfer in both directions.  Proper construction of regexes disguises the information transfer as a set of  http GET commands and responses.

#### Summary

To form basic protocol disguise based on sequences of packets formed by regular expressions driven by a sequence of states, perform the following:

* Make sure that each node of the sequence of states is uniquely labeled.
* Make sure that the initial node of the sequence of states is labeled `start` and that the terminal node is labeled `end`.
* Write down the sequence of transition edges between the states.  Include actions where appropriate.  Ensure that each line ends with a 1.0.
* Generate the list of actions.  Use client `fte.send` commands and server `fte.send` commands as appropriate with the desired regular expressions.
* Make sure to include at least one server `fte.send` command and one client `fte.send` command.

### Finite State Machine (http\_probabilistic\_blocking) Protocol

#### http\_probabilistic\_blocking.mar

```
connection(tcp, 8080):
  start http_get http_get 0.5
  start http_post http_post 0.5
  http_get http10_ok http10_ok 0.5
  http_get http11_ok http11_ok 0.5
  http_post http10_ok http10_ok 0.5
  http_post http11_ok http11_ok 0.5
  http10_ok http_get NULL 0.33
  http10_ok http_post NULL 0.33
  http10_ok end NULL 0.33
  http11_ok http_get NULL 0.33
  http11_ok http_post NULL 0.33
  http11_ok end NULL 0.33

action http_get:
  client fte.send("GET\s/.*", 128)

action http10_ok:
  server fte.send("HTTP/1\.0.*", 128)

action http_post:
  client fte.send("POST\s/.*", 128)

action http11_ok:
  server fte.send("HTTP/1\.1.*", 128)
```
Simple sequences of packets are all well and good, but protocols typically have more variety than that.  We need to make non-deterministic transitions between states.  Fortunately, Marionette has that ability.

By changing the value 1.0 at the end of each transition line to a smaller value, we make each transition a probabilistic change instead of a determined change.  In this manner, we can widen the variety of behaviors, and implement a complete finite state machine.

#### Transitions

There are two transitions from the `start` state.  One proceeds to the `http_get` state with 50% probability:

```
start http_get http_get 0.5
```
and the other proceeds to the `http_post` state with 50% probability:

```
start http_post http_post 0.5
```
because there are two possible actions that can be taken from the start state, behavioral variety beyond the variation due to regular expressions can be expressed.  This allows far more realistic behaviors to be generated.

Note, that although the transitions out of a state do not strictly have to add up to 1.0, (see `http11_ok`) it is recommended if it does not impact understandability.

#### Summary

* Use probabilistic transitions to model protocol behavior where the transmission types vary non-deterministically.
* Set the probabilities of transition to appear as realistic transitions, by either making reasonable estimates or using data to determine how often the transitions really occur.

### Active Probing (http\_active\_probing) Protocol

#### http\_active\_probing.mar

```
connection(tcp, 8080):
  start          upstream       NULL        1.0
  upstream       downstream     http_get    1.0
  upstream       downstream_err NULL        error
  downstream_err end            http_ok_err 1.0
  downstream     end            http_ok     1.0
  
action http_get:
  client fte.send("^GET\ \/([a-zA-Z0-9\.\/]*) HTTP/1\.1\r\n\r\n$", 128)

action http_ok:
  server fte.send("^HTTP/1\.1\ 200 OK\r\nContent-Type:\ ([a-zA-Z0-9]+)\r\n\r\n\C*$", 128)

action http_ok_err:
  server io.puts("HTTP/1.1 200 OK\r\n\r\nHello, World!
```

When the system receives information, it checks to see if any of the transitions that it can make are valid.  If there are no valid transitions that can be performed, the system fails unless there is an available error transition.

If the error transition is available, then it accepts the transition.  This can be exploited to reject scanning.  If additional packets are sent to the server port that do not belong, then the system can execute the error command in response.  Typically, the response is to present a banner or other fixed set of bits back to the offending location.

The action `http_ok_err` transmits back a fixed string using the `io.puts` command.  This command, instead of using a regular expression transmits a set string.  No actual encoded data is transferred across the connection, but it `io.puts` can be very useful for banners and other protocol information that can be considered static.

#### Summary

When you wish to have a good response for additional packets being injected into your flow, such as port scanning, use error transitions.  These transitions can send back information to offending systems that will make your ports appear normal.

### Secure (web_conn443) Protocol

#### web\_conn443.mar

```
connection(tcp, 443):
  start  c_hello NULL 1.0
  c_hello s_hello do_c_hello 1.0
  s_hello s_certificate do_s_hello 1.0
  s_certificate c_key_exchange do_s_certificate 1.0
  c_key_exchange s_change_cipher_spec do_c_key_exchange 1.0
  s_change_cipher_spec cstart do_s_change_cipher_spec 1.0
  cstart conn1 NULL 0.28
  cstart conn2 NULL 0.19
  cstart conn3 NULL 0.14
  cstart conn4 NULL 0.13
  cstart conn5 NULL 0.05
  cstart conn6 NULL 0.1
  cstart conn7 NULL 0.01
  cstart conn8 NULL 0.02
  cstart conn9 NULL 0.03
  cstart conn10 NULL 0.01
  cstart conn15 NULL 0.01
  cstart conn18 NULL 0.01
  cstart conn28 NULL 0.01
  cstart conn31 NULL 0.01
  conn1 requestx1x0 request 1.0
  requestx1x0 responsex1x1 response 1.0
  responsex1x1 end NULL 1.0
  conn2 requestx2x0 request 1.0
  requestx2x0 responsex2x1 response 1.0
  responsex2x1 requestx2x2 request 1.0
  requestx2x2 responsex2x3 response 1.0
  responsex2x3 end NULL 1.0
  conn3 requestx3x0 request 1.0
  requestx3x0 responsex3x1 response 1.0
  responsex3x1 requestx3x2 request 1.0
  requestx3x2 responsex3x3 response 1.0
  responsex3x3 end NULL 1.0
  conn4 requestx4x0 request 1.0
  requestx4x0 responsex4x1 response 1.0
  responsex4x1 requestx4x2 request 1.0
  requestx4x2 responsex4x3 response 1.0
  responsex4x3 requestx4x4 request 1.0
  requestx4x4 responsex4x5 response 1.0
  responsex4x5 end NULL 1.0
  conn5 requestx5x0 request 1.0
  requestx5x0 responsex5x1 response 1.0
  responsex5x1 requestx5x2 request 1.0
  requestx5x2 responsex5x3 response 1.0
  responsex5x3 requestx5x4 request 1.0
  requestx5x4 responsex5x5 response 1.0
  responsex5x5 end NULL 1.0
  conn6 requestx6x0 request 1.0
  requestx6x0 responsex6x1 response 1.0
  responsex6x1 requestx6x2 request 1.0
  requestx6x2 responsex6x3 response 1.0
  responsex6x3 requestx6x4 request 1.0
  requestx6x4 responsex6x5 response 1.0
  responsex6x5 requestx6x6 request 1.0
  requestx6x6 responsex6x7 response 1.0
  responsex6x7 end NULL 1.0
  conn7 requestx7x0 request 1.0
  requestx7x0 responsex7x1 response 1.0
  responsex7x1 requestx7x2 request 1.0
  requestx7x2 responsex7x3 response 1.0
  responsex7x3 requestx7x4 request 1.0
  requestx7x4 responsex7x5 response 1.0
  responsex7x5 requestx7x6 request 1.0
  requestx7x6 responsex7x7 response 1.0
  responsex7x7 end NULL 1.0
  conn8 requestx8x0 request 1.0
  requestx8x0 responsex8x1 response 1.0
  responsex8x1 requestx8x2 request 1.0
  requestx8x2 responsex8x3 response 1.0
  responsex8x3 requestx8x4 request 1.0
  requestx8x4 responsex8x5 response 1.0
  responsex8x5 requestx8x6 request 1.0
  requestx8x6 responsex8x7 response 1.0
  responsex8x7 requestx8x8 request 1.0
  requestx8x8 responsex8x9 response 1.0
  responsex8x9 end NULL 1.0
  conn9 requestx9x0 request 1.0
  requestx9x0 responsex9x1 response 1.0
  responsex9x1 requestx9x2 request 1.0
  requestx9x2 responsex9x3 response 1.0
  responsex9x3 requestx9x4 request 1.0
  requestx9x4 responsex9x5 response 1.0
  responsex9x5 requestx9x6 request 1.0
  requestx9x6 responsex9x7 response 1.0
  responsex9x7 requestx9x8 request 1.0
  requestx9x8 responsex9x9 response 1.0
  responsex9x9 end NULL 1.0
  conn10 requestx10x0 request 1.0
  requestx10x0 responsex10x1 response 1.0
  responsex10x1 requestx10x2 request 1.0
  requestx10x2 responsex10x3 response 1.0
  responsex10x3 requestx10x4 request 1.0
  requestx10x4 responsex10x5 response 1.0
  responsex10x5 requestx10x6 request 1.0
  requestx10x6 responsex10x7 response 1.0
  responsex10x7 requestx10x8 request 1.0
  requestx10x8 responsex10x9 response 1.0
  responsex10x9 requestx10x10 request 1.0
  requestx10x10 responsex10x11 response 1.0
  responsex10x11 end NULL 1.0
  conn15 requestx15x0 request 1.0
  requestx15x0 responsex15x1 response 1.0
  responsex15x1 requestx15x2 request 1.0
  requestx15x2 responsex15x3 response 1.0
  responsex15x3 requestx15x4 request 1.0
  requestx15x4 responsex15x5 response 1.0
  responsex15x5 requestx15x6 request 1.0
  requestx15x6 responsex15x7 response 1.0
  responsex15x7 requestx15x8 request 1.0
  requestx15x8 responsex15x9 response 1.0
  responsex15x9 requestx15x10 request 1.0
  requestx15x10 responsex15x11 response 1.0
  responsex15x11 requestx15x12 request 1.0
  requestx15x12 responsex15x13 response 1.0
  responsex15x13 requestx15x14 request 1.0
  requestx15x14 responsex15x15 response 1.0
  responsex15x15 end NULL 1.0
  conn18 requestx18x0 request 1.0
  requestx18x0 responsex18x1 response 1.0
  responsex18x1 requestx18x2 request 1.0
  requestx18x2 responsex18x3 response 1.0
  responsex18x3 requestx18x4 request 1.0
  requestx18x4 responsex18x5 response 1.0
  responsex18x5 requestx18x6 request 1.0
  requestx18x6 responsex18x7 response 1.0
  responsex18x7 requestx18x8 request 1.0
  requestx18x8 responsex18x9 response 1.0
  responsex18x9 requestx18x10 request 1.0
  requestx18x10 responsex18x11 response 1.0
  responsex18x11 requestx18x12 request 1.0
  requestx18x12 responsex18x13 response 1.0
  responsex18x13 requestx18x14 request 1.0
  requestx18x14 responsex18x15 response 1.0
  responsex18x15 requestx18x16 request 1.0
  requestx18x16 responsex18x17 response 1.0
  responsex18x17 requestx18x18 request 1.0
  requestx18x18 responsex18x19 response 1.0
  responsex18x19 end NULL 1.0
  conn28 requestx28x0 request 1.0
  requestx28x0 responsex28x1 response 1.0
  responsex28x1 requestx28x2 request 1.0
  requestx28x2 responsex28x3 response 1.0
  responsex28x3 requestx28x4 request 1.0
  requestx28x4 responsex28x5 response 1.0
  responsex28x5 requestx28x6 request 1.0
  requestx28x6 responsex28x7 response 1.0
  responsex28x7 requestx28x8 request 1.0
  requestx28x8 responsex28x9 response 1.0
  responsex28x9 requestx28x10 request 1.0
  requestx28x10 responsex28x11 response 1.0
  responsex28x11 requestx28x12 request 1.0
  requestx28x12 responsex28x13 response 1.0
  responsex28x13 requestx28x14 request 1.0
  requestx28x14 responsex28x15 response 1.0
  responsex28x15 requestx28x16 request 1.0
  requestx28x16 responsex28x17 response 1.0
  responsex28x17 requestx28x18 request 1.0
  requestx28x18 responsex28x19 response 1.0
  responsex28x19 requestx28x20 request 1.0
  requestx28x20 responsex28x21 response 1.0
  responsex28x21 requestx28x22 request 1.0
  requestx28x22 responsex28x23 response 1.0
  responsex28x23 requestx28x24 request 1.0
  requestx28x24 responsex28x25 response 1.0
  responsex28x25 requestx28x26 request 1.0
  requestx28x26 responsex28x27 response 1.0
  responsex28x27 requestx28x28 request 1.0
  requestx28x28 responsex28x29 response 1.0
  responsex28x29 end NULL 1.0
  conn31 requestx31x0 request 1.0
  requestx31x0 responsex31x1 response 1.0
  responsex31x1 requestx31x2 request 1.0
  requestx31x2 responsex31x3 response 1.0
  responsex31x3 requestx31x4 request 1.0
  requestx31x4 responsex31x5 response 1.0
  responsex31x5 requestx31x6 request 1.0
  requestx31x6 responsex31x7 response 1.0
  responsex31x7 requestx31x8 request 1.0
  requestx31x8 responsex31x9 response 1.0
  responsex31x9 requestx31x10 request 1.0
  requestx31x10 responsex31x11 response 1.0
  responsex31x11 requestx31x12 request 1.0
  requestx31x12 responsex31x13 response 1.0
  responsex31x13 requestx31x14 request 1.0
  requestx31x14 responsex31x15 response 1.0
  responsex31x15 requestx31x16 request 1.0
  requestx31x16 responsex31x17 response 1.0
  responsex31x17 requestx31x18 request 1.0
  requestx31x18 responsex31x19 response 1.0
  responsex31x19 requestx31x20 request 1.0
  requestx31x20 responsex31x21 response 1.0
  responsex31x21 requestx31x22 request 1.0
  requestx31x22 responsex31x23 response 1.0
  responsex31x23 requestx31x24 request 1.0
  requestx31x24 responsex31x25 response 1.0
  responsex31x25 requestx31x26 request 1.0
  requestx31x26 responsex31x27 response 1.0
  responsex31x27 requestx31x28 request 1.0
  requestx31x28 responsex31x29 response 1.0
  responsex31x29 requestx31x30 request 1.0
  requestx31x30 responsex31x31 response 1.0
  responsex31x31 end NULL 1.0

action request:
  client fte.send("^\x17\x03\x03\x01\xbe.*$", 426)

action response:
  server fte.send("^\x17\x03\x03\x01\xbe.*$", 426)

# Client Hello
action do_c_hello:
  client io.puts('\x16\x03\x01\x00\xcc\x01\x00\x00\xc8\x03\x03\x86\x4b\xde\x9d\x60\x7a\x88\x55\x37\xcc\xc8\x16\x88\x18\xd5\xab\x6b\x9d\x2d\xc0\x4b\x6c\x7f\x2b\x25\x1a\xe9\x94\x82\x16\xcd\x74\x00\x00\x22\xc0\x2b\xc0\x2f\x00\x9e\xcc\x14\xcc\x13\xcc\x15\xc0\x0a\xc0\x14\x00\x39\xc0\x09\xc0\x13\x00\x33\x00\x9c\x00\x35\x00\x2f\x00\x0a\x00\xff\x01\x00\x00\x7d\x00\x00\x00\x11\x00\x0f\x00\x00\x0c\x77\x77\x77\x2e\x69\x61\x6e\x61\x2e\x6f\x72\x67\x00\x17\x00\x00\x00\x23\x00\x00\x00\x0d\x00\x16\x00\x14\x06\x01\x06\x03\x05\x01\x05\x03\x04\x01\x04\x03\x03\x01\x03\x03\x02\x01\x02\x03\x00\x05\x00\x05\x01\x00\x00\x00\x00\x33\x74\x00\x00\x00\x12\x00\x00\x00\x10\x00\x1d\x00\x1b\x08\x68\x74\x74\x70\x2f\x31\x2e\x31\x08\x73\x70\x64\x79\x2f\x33\x2e\x31\x05\x68\x32\x2d\x31\x34\x02\x68\x32\x75\x50\x00\x00\x00\x0b\x00\x02\x01\x00\x00\x0a\x00\x06\x00\x04\x00\x17\x00\x18')

# Server Hello
action do_s_hello:
  server io.puts('\x16\x03\x03\x00\x55\x02\x00\x00\x51\x03\x03\x56\x73\x20\x01\x0b\x7b\xb5\x97\xa8\x88\xee\xac\x07\xcd\x25\xb1\x5b\xa2\xaf\x08\x14\x76\xee\x8e\x89\xdb\xb6\x09\x98\xe5\x2a\x6f\x20\x50\x40\x22\xdf\xd3\x3a\xe3\x19\x19\xcb\xcc\x3b\xb7\x96\xb8\x57\xb3\xbb\xd7\xd2\x8d\xee\xd7\xd0\x3d\x33\x33\x0f\x69\x0f\x48\x76\x00\x35\x00\x00\x09\xff\x01\x00\x01\x00\x00\x00\x00\x00')

# Server Certificate -
action do_s_certificate:
  server io.puts('\x16\x03\x03\x0b\x49\x0b\x00\x0b\x45\x00\x0b\x42\x00\x06\x87\x30\x82\x06\x83\x30\x82\x05\x6b\xa0\x03\x02\x01\x02\x02\x10\x09\xca\xbb\xe2\x19\x1c\x8f\x56\x9d\xd4\xb6\xdd\x25\x0f\x21\xd8\x30\x0d\x06\x09\x2a\x86\x48\x86\xf7\x0d\x01\x01\x0b\x05\x00\x30\x70\x31\x0b\x30\x09\x06\x03\x55\x04\x06\x13\x02\x55\x53\x31\x15\x30\x13\x06\x03\x55\x04\x0a\x13\x0c\x44\x69\x67\x69\x43\x65\x72\x74\x20\x49\x6e\x63\x31\x19\x30\x17\x06\x03\x55\x04\x0b\x13\x10\x77\x77\x77\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x31\x2f\x30\x2d\x06\x03\x55\x04\x03\x13\x26\x44\x69\x67\x69\x43\x65\x72\x74\x20\x53\x48\x41\x32\x20\x48\x69\x67\x68\x20\x41\x73\x73\x75\x72\x61\x6e\x63\x65\x20\x53\x65\x72\x76\x65\x72\x20\x43\x41\x30\x1e\x17\x0d\x31\x34\x31\x30\x32\x37\x30\x30\x30\x30\x30\x30\x5a\x17\x0d\x31\x38\x30\x31\x30\x33\x31\x32\x30\x30\x30\x30\x5a\x30\x81\xa3\x31\x0b\x30\x09\x06\x03\x55\x04\x06\x13\x02\x55\x53\x31\x13\x30\x11\x06\x03\x55\x04\x08\x13\x0a\x43\x61\x6c\x69\x66\x6f\x72\x6e\x69\x61\x31\x14\x30\x12\x06\x03\x55\x04\x07\x13\x0b\x4c\x6f\x73\x20\x41\x6e\x67\x65\x6c\x65\x73\x31\x3c\x30\x3a\x06\x03\x55\x04\x0a\x13\x33\x49\x6e\x74\x65\x72\x6e\x65\x74\x20\x43\x6f\x72\x70\x6f\x72\x61\x74\x69\x6f\x6e\x20\x66\x6f\x72\x20\x41\x73\x73\x69\x67\x6e\x65\x64\x20\x4e\x61\x6d\x65\x73\x20\x61\x6e\x64\x20\x4e\x75\x6d\x62\x65\x72\x73\x31\x16\x30\x14\x06\x03\x55\x04\x0b\x13\x0d\x49\x54\x20\x4f\x70\x65\x72\x61\x74\x69\x6f\x6e\x73\x31\x13\x30\x11\x06\x03\x55\x04\x03\x0c\x0a\x2a\x2e\x69\x61\x6e\x61\x2e\x6f\x72\x67\x30\x82\x02\x22\x30\x0d\x06\x09\x2a\x86\x48\x86\xf7\x0d\x01\x01\x01\x05\x00\x03\x82\x02\x0f\x00\x30\x82\x02\x0a\x02\x82\x02\x01\x00\x9d\xbd\xfd\xde\xb5\xca\xe5\x3a\x55\x97\x47\xe2\xfd\xa6\x37\x28\xe4\xab\xa6\x0f\x18\xb7\x9a\x69\xf0\x33\x10\xbf\x01\x64\xe5\xee\x7d\xb6\xb1\x5b\xf5\x6d\xf2\x3f\xdd\xba\xe6\xa1\xbb\x38\x44\x9b\x8c\x88\x3f\x18\x10\x2b\xbd\x8b\xb6\x55\xac\x0e\x2d\xac\x2e\xe3\xed\x5c\xf4\x31\x58\x68\xd2\xc5\x98\x06\x82\x84\x85\x4b\x24\x89\x4d\xcd\x4b\xd3\x78\x11\xf0\xad\x3a\x28\x2c\xd4\xb4\xe5\x99\xff\xd0\x7d\x8d\x2d\x3f\x24\x78\x55\x4f\x81\x02\x0b\x32\x0e\xe1\x2f\x44\x94\x8e\x2e\xa1\xed\xbc\x99\x0b\x83\x0c\xa5\xcc\xa6\xb4\xa8\x39\xfb\x27\xb5\x18\x50\xc9\x84\x7e\xac\x74\xf2\x66\x09\xeb\x24\x36\x5b\x97\x51\xfb\x1c\x32\x08\xf5\x69\x13\xba\xcb\xca\xe4\x92\x01\x34\x7c\x78\xb7\xe5\x4a\x9d\x99\x97\x94\x04\xc3\x7f\x00\xfb\x65\xdb\x84\x9f\xd7\x5e\x3a\x68\x77\x0c\x30\xf2\xab\xe6\x5b\x33\x25\x6f\xb5\x9b\x45\x00\x50\xb0\x0d\x81\x39\xd4\xd8\x0d\x36\xf7\xbc\x46\xda\xf3\x03\xe4\x8f\x0f\x07\x91\xb2\xfd\xd7\x2e\xc6\x0b\x2c\xb3\xad\x53\x3c\x3f\x28\x8c\x9c\x19\x4e\x49\x33\x7a\x69\xc4\x96\x73\x1f\x08\x6d\x4f\x1f\x98\x25\x90\x07\x13\xe2\xa5\x51\xd0\x5c\xb6\x05\x75\x67\x85\x0d\x91\xe6\x00\x1c\x4c\xe2\x71\x76\xf0\x95\x78\x73\xa9\x5b\x88\x0a\xcb\xec\x19\xe7\xbd\x9b\xcf\x12\x86\xd0\x45\x2b\x73\x78\x9c\x41\x90\x5d\xd4\x70\x97\x1c\xd7\x3a\xea\x52\xc7\x7b\x08\x0c\xd7\x79\xaf\x58\x23\x4f\x33\x72\x25\xc2\x6f\x87\xa8\xc1\x3e\x2a\x65\xe9\xdd\x4e\x03\xa5\xb4\x1d\x7e\x06\xb3\x35\x3f\x38\x12\x9b\x23\x27\xa5\x31\xec\x96\x27\xa2\x1d\xc4\x23\x73\x3a\xa0\x29\xd4\x98\x94\x48\xba\x33\x22\x89\x1c\x1a\x56\x90\xdd\xf2\xd2\x5c\x8e\xc8\xaa\xa8\x94\xb1\x4a\xa9\x21\x30\xc6\xb6\xd9\x69\xa2\x1f\xf6\x71\xb6\x0c\x4c\x92\x3a\x94\xa9\x3e\xa1\xdd\x04\x92\xc9\x33\x93\xca\x6e\xdd\x61\xf3\x3c\xa7\x7e\x92\x08\xd0\x1d\x6b\xd1\x51\x07\x66\x2e\xc0\x88\x73\x3d\xf4\xc8\x76\xa7\xe1\x60\x8b\x82\x97\x3a\x0f\x75\x92\xe8\x4e\xd1\x55\x79\xd1\x81\xe7\x90\x24\xae\x8a\x7e\x4b\x9f\x00\x78\xeb\x20\x05\xb2\x3f\x9d\x09\xa1\xdf\x1b\xbc\x7d\xe2\xa5\xa6\x08\x5a\x36\x46\xd9\xfa\xdb\x0e\x9d\xa2\x73\xa5\xf4\x03\xcd\xd4\x28\x31\xce\x6f\x0c\xa4\x68\x89\x58\x56\x02\xbb\x8b\xc3\x6b\xb3\xbe\x86\x1f\xf6\xd1\xa6\x2e\x35\x02\x03\x01\x00\x01\xa3\x82\x01\xe3\x30\x82\x01\xdf\x30\x1f\x06\x03\x55\x1d\x23\x04\x18\x30\x16\x80\x14\x51\x68\xff\x90\xaf\x02\x07\x75\x3c\xcc\xd9\x65\x64\x62\xa2\x12\xb8\x59\x72\x3b\x30\x1d\x06\x03\x55\x1d\x0e\x04\x16\x04\x14\xc7\xd0\xac\xef\x89\x8b\x20\xe4\xb9\x14\x66\x89\x33\x03\x23\x94\xf6\xbf\x3a\x61\x30\x1f\x06\x03\x55\x1d\x11\x04\x18\x30\x16\x82\x0a\x2a\x2e\x69\x61\x6e\x61\x2e\x6f\x72\x67\x82\x08\x69\x61\x6e\x61\x2e\x6f\x72\x67\x30\x0e\x06\x03\x55\x1d\x0f\x01\x01\xff\x04\x04\x03\x02\x05\xa0\x30\x1d\x06\x03\x55\x1d\x25\x04\x16\x30\x14\x06\x08\x2b\x06\x01\x05\x05\x07\x03\x01\x06\x08\x2b\x06\x01\x05\x05\x07\x03\x02\x30\x75\x06\x03\x55\x1d\x1f\x04\x6e\x30\x6c\x30\x34\xa0\x32\xa0\x30\x86\x2e\x68\x74\x74\x70\x3a\x2f\x2f\x63\x72\x6c\x33\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x2f\x73\x68\x61\x32\x2d\x68\x61\x2d\x73\x65\x72\x76\x65\x72\x2d\x67\x33\x2e\x63\x72\x6c\x30\x34\xa0\x32\xa0\x30\x86\x2e\x68\x74\x74\x70\x3a\x2f\x2f\x63\x72\x6c\x34\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x2f\x73\x68\x61\x32\x2d\x68\x61\x2d\x73\x65\x72\x76\x65\x72\x2d\x67\x33\x2e\x63\x72\x6c\x30\x42\x06\x03\x55\x1d\x20\x04\x3b\x30\x39\x30\x37\x06\x09\x60\x86\x48\x01\x86\xfd\x6c\x01\x01\x30\x2a\x30\x28\x06\x08\x2b\x06\x01\x05\x05\x07\x02\x01\x16\x1c\x68\x74\x74\x70\x73\x3a\x2f\x2f\x77\x77\x77\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x2f\x43\x50\x53\x30\x81\x83\x06\x08\x2b\x06\x01\x05\x05\x07\x01\x01\x04\x77\x30\x75\x30\x24\x06\x08\x2b\x06\x01\x05\x05\x07\x30\x01\x86\x18\x68\x74\x74\x70\x3a\x2f\x2f\x6f\x63\x73\x70\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x30\x4d\x06\x08\x2b\x06\x01\x05\x05\x07\x30\x02\x86\x41\x68\x74\x74\x70\x3a\x2f\x2f\x63\x61\x63\x65\x72\x74\x73\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x2f\x44\x69\x67\x69\x43\x65\x72\x74\x53\x48\x41\x32\x48\x69\x67\x68\x41\x73\x73\x75\x72\x61\x6e\x63\x65\x53\x65\x72\x76\x65\x72\x43\x41\x2e\x63\x72\x74\x30\x0c\x06\x03\x55\x1d\x13\x01\x01\xff\x04\x02\x30\x00\x30\x0d\x06\x09\x2a\x86\x48\x86\xf7\x0d\x01\x01\x0b\x05\x00\x03\x82\x01\x01\x00\x70\x31\x4c\x38\xe7\xc0\x2f\xd8\x08\x10\x50\x0b\x9d\xf6\xda\xe8\x5d\xe9\xb2\x3e\x29\xfb\xd6\x8b\xfd\xb5\xf2\x34\x11\xc8\x9a\xcf\xaf\x9a\xe0\x5a\xf9\x12\x3a\x8a\xa6\xbc\xe6\x95\x4a\x4e\x68\xdc\x7c\xfc\x48\x0a\x65\xd7\x6f\x22\x9c\x4b\xd5\xf5\x67\x4b\x0c\x9a\xc6\xd0\x6a\x37\xa1\xa1\xc1\x45\xc3\x95\x61\x20\xb8\xef\xe6\x7c\x88\x7a\xb4\xff\x7d\x6a\xa9\x50\xff\x36\x98\xf2\x7c\x4a\x19\xd5\x9d\x93\xa3\x9a\xca\x5a\x7b\x6d\x6c\x75\xe3\x49\x74\xe5\x0f\x5a\x59\x00\x05\xb3\xcb\x66\x5d\xdb\xd7\x07\x4f\x9f\xcb\xcb\xf9\xc5\x02\x28\xd5\xe2\x55\x96\xb6\x4a\xda\x16\x0b\x48\xf7\x7a\x93\xaa\xce\xd2\x26\x17\xbf\xe0\x05\xe0\x0f\xe2\x0a\x53\x2a\x0a\xdc\xb8\x18\xc8\x78\xdc\x5d\x66\x49\x27\x77\x77\xca\x1a\x81\x4e\x21\xd0\xb5\x33\x08\xaf\x40\x78\xbe\x45\x54\x71\x5e\x4c\xe4\x82\x8b\x01\x2f\x25\xff\xa1\x3a\x6c\xeb\x30\xd2\x0a\x75\xde\xba\x8a\x34\x4e\x41\xd6\x27\xfa\x63\x8f\xef\xf3\x8a\x30\x63\xa0\x18\x75\x19\xb3\x9b\x05\x3f\x71\x34\xd9\xcd\x83\xe6\x09\x1a\xcc\xf5\xd2\xe3\xa0\x5e\xdf\xa1\xdf\xbe\x18\x1a\x87\xad\x86\xba\x24\xfe\x6b\x97\xfe\x00\x04\xb5\x30\x82\x04\xb1\x30\x82\x03\x99\xa0\x03\x02\x01\x02\x02\x10\x04\xe1\xe7\xa4\xdc\x5c\xf2\xf3\x6d\xc0\x2b\x42\xb8\x5d\x15\x9f\x30\x0d\x06\x09\x2a\x86\x48\x86\xf7\x0d\x01\x01\x0b\x05\x00\x30\x6c\x31\x0b\x30\x09\x06\x03\x55\x04\x06\x13\x02\x55\x53\x31\x15\x30\x13\x06\x03\x55\x04\x0a\x13\x0c\x44\x69\x67\x69\x43\x65\x72\x74\x20\x49\x6e\x63\x31\x19\x30\x17\x06\x03\x55\x04\x0b\x13\x10\x77\x77\x77\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x31\x2b\x30\x29\x06\x03\x55\x04\x03\x13\x22\x44\x69\x67\x69\x43\x65\x72\x74\x20\x48\x69\x67\x68\x20\x41\x73\x73\x75\x72\x61\x6e\x63\x65\x20\x45\x56\x20\x52\x6f\x6f\x74\x20\x43\x41\x30\x1e\x17\x0d\x31\x33\x31\x30\x32\x32\x31\x32\x30\x30\x30\x30\x5a\x17\x0d\x32\x38\x31\x30\x32\x32\x31\x32\x30\x30\x30\x30\x5a\x30\x70\x31\x0b\x30\x09\x06\x03\x55\x04\x06\x13\x02\x55\x53\x31\x15\x30\x13\x06\x03\x55\x04\x0a\x13\x0c\x44\x69\x67\x69\x43\x65\x72\x74\x20\x49\x6e\x63\x31\x19\x30\x17\x06\x03\x55\x04\x0b\x13\x10\x77\x77\x77\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x31\x2f\x30\x2d\x06\x03\x55\x04\x03\x13\x26\x44\x69\x67\x69\x43\x65\x72\x74\x20\x53\x48\x41\x32\x20\x48\x69\x67\x68\x20\x41\x73\x73\x75\x72\x61\x6e\x63\x65\x20\x53\x65\x72\x76\x65\x72\x20\x43\x41\x30\x82\x01\x22\x30\x0d\x06\x09\x2a\x86\x48\x86\xf7\x0d\x01\x01\x01\x05\x00\x03\x82\x01\x0f\x00\x30\x82\x01\x0a\x02\x82\x01\x01\x00\xb6\xe0\x2f\xc2\x24\x06\xc8\x6d\x04\x5f\xd7\xef\x0a\x64\x06\xb2\x7d\x22\x26\x65\x16\xae\x42\x40\x9b\xce\xdc\x9f\x9f\x76\x07\x3e\xc3\x30\x55\x87\x19\xb9\x4f\x94\x0e\x5a\x94\x1f\x55\x56\xb4\xc2\x02\x2a\xaf\xd0\x98\xee\x0b\x40\xd7\xc4\xd0\x3b\x72\xc8\x14\x9e\xef\x90\xb1\x11\xa9\xae\xd2\xc8\xb8\x43\x3a\xd9\x0b\x0b\xd5\xd5\x95\xf5\x40\xaf\xc8\x1d\xed\x4d\x9c\x5f\x57\xb7\x86\x50\x68\x99\xf5\x8a\xda\xd2\xc7\x05\x1f\xa8\x97\xc9\xdc\xa4\xb1\x82\x84\x2d\xc6\xad\xa5\x9c\xc7\x19\x82\xa6\x85\x0f\x5e\x44\x58\x2a\x37\x8f\xfd\x35\xf1\x0b\x08\x27\x32\x5a\xf5\xbb\x8b\x9e\xa4\xbd\x51\xd0\x27\xe2\xdd\x3b\x42\x33\xa3\x05\x28\xc4\xbb\x28\xcc\x9a\xac\x2b\x23\x0d\x78\xc6\x7b\xe6\x5e\x71\xb7\x4a\x3e\x08\xfb\x81\xb7\x16\x16\xa1\x9d\x23\x12\x4d\xe5\xd7\x92\x08\xac\x75\xa4\x9c\xba\xcd\x17\xb2\x1e\x44\x35\x65\x7f\x53\x25\x39\xd1\x1c\x0a\x9a\x63\x1b\x19\x92\x74\x68\x0a\x37\xc2\xc2\x52\x48\xcb\x39\x5a\xa2\xb6\xe1\x5d\xc1\xdd\xa0\x20\xb8\x21\xa2\x93\x26\x6f\x14\x4a\x21\x41\xc7\xed\x6d\x9b\xf2\x48\x2f\xf3\x03\xf5\xa2\x68\x92\x53\x2f\x5e\xe3\x02\x03\x01\x00\x01\xa3\x82\x01\x49\x30\x82\x01\x45\x30\x12\x06\x03\x55\x1d\x13\x01\x01\xff\x04\x08\x30\x06\x01\x01\xff\x02\x01\x00\x30\x0e\x06\x03\x55\x1d\x0f\x01\x01\xff\x04\x04\x03\x02\x01\x86\x30\x1d\x06\x03\x55\x1d\x25\x04\x16\x30\x14\x06\x08\x2b\x06\x01\x05\x05\x07\x03\x01\x06\x08\x2b\x06\x01\x05\x05\x07\x03\x02\x30\x34\x06\x08\x2b\x06\x01\x05\x05\x07\x01\x01\x04\x28\x30\x26\x30\x24\x06\x08\x2b\x06\x01\x05\x05\x07\x30\x01\x86\x18\x68\x74\x74\x70\x3a\x2f\x2f\x6f\x63\x73\x70\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x30\x4b\x06\x03\x55\x1d\x1f\x04\x44\x30\x42\x30\x40\xa0\x3e\xa0\x3c\x86\x3a\x68\x74\x74\x70\x3a\x2f\x2f\x63\x72\x6c\x34\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x2f\x44\x69\x67\x69\x43\x65\x72\x74\x48\x69\x67\x68\x41\x73\x73\x75\x72\x61\x6e\x63\x65\x45\x56\x52\x6f\x6f\x74\x43\x41\x2e\x63\x72\x6c\x30\x3d\x06\x03\x55\x1d\x20\x04\x36\x30\x34\x30\x32\x06\x04\x55\x1d\x20\x00\x30\x2a\x30\x28\x06\x08\x2b\x06\x01\x05\x05\x07\x02\x01\x16\x1c\x68\x74\x74\x70\x73\x3a\x2f\x2f\x77\x77\x77\x2e\x64\x69\x67\x69\x63\x65\x72\x74\x2e\x63\x6f\x6d\x2f\x43\x50\x53\x30\x1d\x06\x03\x55\x1d\x0e\x04\x16\x04\x14\x51\x68\xff\x90\xaf\x02\x07\x75\x3c\xcc\xd9\x65\x64\x62\xa2\x12\xb8\x59\x72\x3b\x30\x1f\x06\x03\x55\x1d\x23\x04\x18\x30\x16\x80\x14\xb1\x3e\xc3\x69\x03\xf8\xbf\x47\x01\xd4\x98\x26\x1a\x08\x02\xef\x63\x64\x2b\xc3\x30\x0d\x06\x09\x2a\x86\x48\x86\xf7\x0d\x01\x01\x0b\x05\x00\x03\x82\x01\x01\x00\x18\x8a\x95\x89\x03\xe6\x6d\xdf\x5c\xfc\x1d\x68\xea\x4a\x8f\x83\xd6\x51\x2f\x8d\x6b\x44\x16\x9e\xac\x63\xf5\xd2\x6e\x6c\x84\x99\x8b\xaa\x81\x71\x84\x5b\xed\x34\x4e\xb0\xb7\x79\x92\x29\xcc\x2d\x80\x6a\xf0\x8e\x20\xe1\x79\xa4\xfe\x03\x47\x13\xea\xf5\x86\xca\x59\x71\x7d\xf4\x04\x96\x6b\xd3\x59\x58\x3d\xfe\xd3\x31\x25\x5c\x18\x38\x84\xa3\xe6\x9f\x82\xfd\x8c\x5b\x98\x31\x4e\xcd\x78\x9e\x1a\xfd\x85\xcb\x49\xaa\xf2\x27\x8b\x99\x72\xfc\x3e\xaa\xd5\x41\x0b\xda\xd5\x36\xa1\xbf\x1c\x6e\x47\x49\x7f\x5e\xd9\x48\x7c\x03\xd9\xfd\x8b\x49\xa0\x98\x26\x42\x40\xeb\xd6\x92\x11\xa4\x64\x0a\x57\x54\xc4\xf5\x1d\xd6\x02\x5e\x6b\xac\xee\xc4\x80\x9a\x12\x72\xfa\x56\x93\xd7\xff\xbf\x30\x85\x06\x30\xbf\x0b\x7f\x4e\xff\x57\x05\x9d\x24\xed\x85\xc3\x2b\xfb\xa6\x75\xa8\xac\x2d\x16\xef\x7d\x79\x27\xb2\xeb\xc2\x9d\x0b\x07\xea\xaa\x85\xd3\x01\xa3\x20\x28\x41\x59\x43\x28\xd2\x81\xe3\xaa\xf6\xec\x7b\x3b\x77\xb6\x40\x62\x80\x05\x41\x45\x01\xef\x17\x06\x3e\xde\xc0\x33\x9b\x67\xd3\x61\x2e\x72\x87\xe4\x69\xfc\x12\x00\x57\x40\x1e\x70\xf5\x1e\xc9\xb4\x16\x03\x03\x00\x04\x0e\x00\x00\x00')

# Client Key Exchange, Change Cipher Spec, and Encrypted Handshake Message
action do_c_key_exchange:
  client io.puts('\x16\x03\x03\x02\x06\x10\x00\x02\x02\x02\x00\x18\x95\xd1\x32\x1e\x90\x0d\xa6\x72\xf2\xb4\xe7\x0d\x55\x1b\x6e\x4a\xf3\x8d\xf2\x5a\xbb\x25\x19\x18\x69\x95\x97\x09\x74\x5f\x72\xbd\x41\x01\x49\x9f\xae\x7e\xda\xec\x33\xaa\x91\x59\x4d\x44\x8f\x86\x46\xdc\x53\xda\x83\xf9\x97\xa1\xde\x91\x01\xbc\x10\x8f\x86\x1a\x2a\x58\x78\x0a\xb8\x6f\x30\x27\x30\xc5\xbc\x1a\xed\xe5\x4b\xfb\xbc\x04\x31\xbb\x0c\x57\xb5\xce\x7d\x04\xeb\xf7\x69\x53\xfd\x55\xfa\x4d\x9f\x3d\x17\x07\xd4\x22\xba\x64\xed\x5a\x73\x8d\xdf\x62\xd8\xf5\xe0\x34\x06\x89\xc6\x7b\x39\x96\x4e\x6c\xb8\x1c\x08\x65\xa8\xbb\xfe\x54\x86\xbf\x55\x99\x66\x46\xfa\x1d\x99\xd1\xf3\xaa\x3f\x44\x82\xcc\x9a\x6d\x6f\x6d\xcd\x6f\xa7\xde\xdb\x48\xbd\x2e\xdb\xa1\xe4\x03\x9a\x94\xe2\xcf\x20\x2c\xba\xa0\x1b\x31\xfe\x53\x17\x46\xf6\xca\xcd\xa1\x08\x5e\x72\x13\x07\x5d\x69\xb4\xa6\x41\x36\xb1\xbf\xa9\xf6\xf9\xc1\x8d\xa7\x6f\x72\x94\x12\x92\x57\x9b\x5b\x1e\x7d\x4c\x8a\xfa\x2c\x2e\x83\xb6\xdd\xf5\xcb\x4c\xfd\x78\x6b\xe7\x17\xfd\x3c\x77\xc6\x80\x16\x43\xce\xef\x95\x09\xca\x00\x21\x3c\xc4\x5d\xa7\x76\xee\x91\x5b\xea\xfa\xee\xd1\xab\x4d\x97\x99\xeb\xc4\x5d\x1b\x51\x98\xcd\x3a\x8e\xf6\x68\xf5\x5a\x5b\x46\xc3\x3c\x53\x4a\x97\xa3\x64\x4a\x5a\xd1\x5d\x33\x37\xec\x4e\x56\xad\x83\x8b\x69\x8e\xb7\xb3\xbe\x81\x0f\x1b\xad\xd9\x3b\x82\x35\xf0\x3e\x02\x28\x14\xb9\xdc\xab\xa0\x76\x12\x75\x1e\x84\xf0\x92\xc0\xbb\xc6\x08\xe3\x8e\x9d\x3f\xd9\x8d\x70\x53\x7c\xab\xe8\xd3\x5c\xd3\xf6\xb9\x01\x66\x62\x11\x0d\xa6\xa2\xbc\x17\x6b\x07\xa5\x4c\x49\x3c\x9b\x1d\xb1\x48\xd6\xb7\xaf\xef\x5b\xec\xb3\xf8\xba\x9d\xa9\xb6\xaa\x1b\x16\xa6\x82\xc8\xda\x83\x63\x34\x7c\x2e\x21\xf0\xbf\x9e\xb4\x28\xdc\x7c\xa4\x76\x69\x3d\x1e\x0e\x05\x1a\xe5\x04\xf0\xe5\xd9\xde\x14\xe0\xd0\xe6\xae\xee\x60\x6e\x52\xe5\x3a\x5f\x7c\xd5\xca\x2e\xb8\x01\x4f\x58\xa1\xee\xb7\x22\x68\xa6\x92\x71\x8c\xe0\x11\xdb\xee\x3b\x0c\xce\xc6\x84\x5a\x3d\xf3\xce\x63\x2a\x98\x8a\x81\xbe\xeb\xf7\x74\xa4\x68\xd3\xed\x31\x82\xa7\xef\x75\x3b\x37\xba\x45\x15\x12\x8c\xfe\xfb\xc4\x9e\x45\xbc\x3a\x43\x40\x9c\xfe\xa4\x27\xd0\xd0\xab\xb3\x8a\x4d\x83\x45\x9c\xf1\xec\x1c\xd9\xc3\xf3\x3e\x33\xc6\x2f\x4f\x81\x06\xe5\xdc\xe0\xd5\x14\x03\x03\x00\x01\x01\x16\x03\x03\x00\x40\x40\x47\x40\x39\x48\x8b\xe0\x51\xf0\x7a\xda\x3f\x1a\xb9\xc8\x1b\xa2\xb2\x3d\x44\x9a\xc1\x8e\x49\xb5\x47\x09\x5e\x87\x3c\xc6\x67\xe8\xe0\x82\xa9\xdd\x30\x21\x65\x9b\x03\xd9\x72\xfb\x9b\x4c\x10\xa9\xdc\xea\x0b\xb0\xde\x7b\x49\x78\xc7\xf5\x38\x5b\x1b\x4d\xc8')

# Server Change Cipher Spec, and Encrypted Handshake Message
action do_s_change_cipher_spec:
  server io.puts('\x14\x03\x03\x00\x01\x01\x16\x03\x03\x00\x40\xf6\x53\x99\x89\xdf\x44\xa1\x28\x5a\xc5\x85\xd3\x60\x09\xc6\x99\xe2\x9a\x3a\xea\x9f\xcb\xb2\x5d\x89\x40\x21\xcc\xf0\x9f\xcb\x1f\x01\x93\x66\x88\xb9\xd4\x56\xf0\xf0\x0f\x14\xcf\x19\x34\x13\xc7\xf5\x0a\xb2\x10\x42\x9c\x19\x2a\x56\x7b\x9c\xd3\x85\x71\x7d\x23')

# Client Application Data - \x01\xbe is the length of the "TLS encrypted" data in hex
action do_c_app_data:
  client fte.send("^\x17\x03\x03\x01\xbe.*$", 426)

# Server Application Data - \x01\xbe is the length of the "TLS encrypted" data in hex
action do_s_app_data:
  server fte.send("^\x17\x03\x03\x01\xbe.*$", 426)
```

This protocol simulates a secure web connection with encryption handshake and certificate exchange.  To simulate the secure handshake, a set of transitions leads the activity that statically describe the exchange are first executed.  These contain data strings which encode certificates, handshaking protocols and other activities.  This information has been scraped from actual https sessions, and therefore, can be used to great effect.  

The following sets of transitions from the `cstart` state simulate sets of secure packets being transferred back and forth.  Each has a different length, thereby allowing simulation of connections where different numbers of a packets are possible.  By using sequences of fixed numbers of packets, the probabilities of transition from the `cstart` state determine the frequency of these packet sets.  This allows the protocol to take on the traffic patters of actually observed web browsing.

#### Summary

* Sophisticated protocol behaviour, such as key exchange, can be simulated through the use of fixed strings based on data scraped from actual packet capture and the use of the `io.puts` command.
* Other traffic patterns, such as the number of packets per connection, can be simulated through probabilistic transitions, using probabilites derived from actual packet capture.
* Large .mar files should not be feared.  They are the best way to model real-world behavior. 

### SubModel (web\_sess443) Protocol

#### web\_sess443 Protocol

This is our most sophisticated protocol and is an excellent example of traffic manipulation to appear as if a person is actually using an application.  It simulates secure web browsing of amazon.com with actual certificates, appropriate time delays between accesses to simulate human interaction and sub-models for each web browser connection.

```
connection(tcp, 110):
  start do33 NULL 0.35
  do33 do33x2 sess33 1.0
  do33x2 end sleep33 1.0
  start do34 NULL 0.15
  do34 do34x2 sess34 1.0
  do34x2 end sleep34 1.0
  start do35 NULL 0.15
  do35 do35x2 sess35 1.0
  do35x2 end sleep35 1.0
  start do37 NULL 0.15
  do37 do37x2 sess37 1.0
  do37x2 end sleep37 1.0
  start do39 NULL 0.20
  do39 do39x2 sess39 1.0
  do39x2 end sleep39 1.0

action sess33:
  client model.spawn("web_conn443", 33)
  server model.spawn("web_conn443", 33)

action sess34:
  client model.spawn("web_conn443", 34)
  server model.spawn("web_conn443", 34)

action sess35:
  client model.spawn("web_conn443", 35)
  server model.spawn("web_conn443", 35)

action sess37:
  client model.spawn("web_conn443", 37)
  server model.spawn("web_conn443", 37)

action sess39:
  client model.spawn("web_conn443", 39)
  server model.spawn("web_conn443", 39)

action sleep33:
  server model.sleep("{'5.0' : 1.0}")

action sleep34: 
  server model.sleep("{'4.0' : 1.0}")

action sleep35: 
  server model.sleep("{'3.0' : 1.0}")

action sleep37: 
  server model.sleep("{'2.0' : 1.0}")

action sleep39: 
  server model.sleep("{'1.0' : 1.0}")
``` 

This model is demonstrates two behaviors that make it appear to be a standard web browsing session.  The statistics are captured from actual recorded web behavior.

#### Model.spawn

The command `model.spawn` instantiates a protocol model as a submodel of the current protocol.  This command has two arguments.  First, the model of the protocol that is to be used, which is `web_connn443` in this case.  The second argument is the number of times that the model will perform its `start` to `end` loop before returning.  This allows the protocols to be specified hierarchically.

Note, that the model needs to be identically spawned for both the client and server sides.  This is not done automatically as in the case of `fte.send`.  

#### Sleep

The command `sleep` instantiates a period of time when the protocol is inactive.  This command is extremely useful for shaping the appearance of web traffic.  Human interaction, if it is being simulated, is considerably slower than automated systems.  Shaping of such web traffic requires delays to be injected into the protocol.  The sleep command takes an array of numerical pairs as an argument.  Each numerical pair represents the sleep time in seconds and the probability that the time will be utilized.

#### Summary

Although in the `web_conn443` protocol we saw that the size of the file was not truly relevant, by using hierarchical models, we can make protocols far more manageable.

Sleep is an important command for use with systems that have human interaction.  Use it.

