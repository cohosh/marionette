Developer Manual
================

This is guide for developers interested in writing their own Marionette
specifications using the MAR language. End users can refer to the user
manual instead to simply use the client and server tools with existing
MAR documents.


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
- `PROBABILITY` is the probability of execution, between `0.0` and `1.0`. This format location can also take on the value `error`.

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


#### Actions

Actions represent a call to the built-in plugins in `marionette`. These are 
specified as follows:

```
PARTY MODULE.NAME([ARG, [ARG, ...]]) [if regex_match_incoming("REGEX")]
```

- `PARTY` must be either `client` or `server`.
- `MODULE` specifies the plugin module where the function exists.
- `NAME` specifies the plugin name.
- `ARG...` represents zero or more comma-delimited arguments passed to the plugin.
- `REGEX` is a regular expression that incoming data must match to execute.  The regular expression is also the form that outgoing data will be shaped into.

Actions are only performed by one party (either `client` or `server`), however,
some actions have built-in counter actions. For example, when a client sends
data the server will implicitly have an action to read the data.

Arguments must be a quoted string (single or double quotes), an integer, or a
floating point number.


## Plugins

There are several modules of built-in plugins. Each plugin is the basis for an
action in Marionette and each one has specific arguments as documented below.
All plugin code can be found in the `plugins` package.


### Module: channel

The `channel` module provides plugins related to creating additional incoming
TCP connections.

#### `channel.bind()`

Arguments:

1. `name:string`

The `bind()` plugin opens a listener on a random port and saves the port number
to the `name` variable in the FSM. This is used in conjunction with
`model.spawn()` to use this random port for a child FSM.


### Module: fte

The `fte` module provides plugins to send and receive data using FTE
(Format-Transforming Encryption). This encrypts plaintext data and then conforms
it to a given regular expression for transport.

#### `fte.send()` & `fte.send_async()`

Arguments:

1. `regex:string`
2. `msg_len:integer`

The `send()` plugin encrypts and sends queued incoming data to the counter
party, up to `msg_len` bytes. If there is no available data then an empty cell
is sent. If `send_async()` is used and no data is available then the send is
ignored.

`recv()` and `recv_async()` are counter actions to `send()` and `send_async()`.


#### `fte.recv()` & `fte.recv_async()`

Arguments:

1. `regex:string`
2. `msg_len:integer`

The `recv()` & `recv_async()` plugins decrypt data sent by `fte.send()` &
`fte.send_async()`. These actions should not be specified by the protocol author
but instead are added automatically as counter actions by the FSM evaluator.


### Module: io

The `io` module provides plaintext input/output plugins.

#### `io.puts()`

Arguments:

1. `data:string` is the data to send to the counter party.

The `puts()` plugin simply writes `data` to the outgoing connection.

`gets()` is the counter action to `puts()`.


#### `io.gets()`

Arguments:

1. `expects:string` is the data expected to be read from the 

The `gets()` plugin reads `expects` data from the incoming connection. Fails
if the expected data is not read. On failure, any read data is returned to the
incoming connection's buffer.

The `gets()` plugin should not be specified by the protocol author but instead
it is added automatically as a counter action by the FSM evaluator.


### Module: model

The `model` module provides plugins for creating child FSMs and for sleeping.

#### `model.sleep()`

Arguments:

1. `distribution:string` is a mapping of sleep times (in seconds) to probabilities.

The `sleep()` plugin sleeps for an amount of time randomly chosen from
`distribution`. The `distribution` string must be formatted as a curly-brace
contained mapping of `float` sleep times to `float` probabilities.

For example, this is a distribution containing `0.1` seconds and `0.01` seconds.
The first timing has a 25% probability and the second timing has a 75% 
probability.

```
model.sleep("{'0.1': 0.25, '0.01': 0.75}")
```

#### `model.spawn()`

Arguments:

1. `format:string` is the MAR document format name.
2. `n:integer` is the number of instances to spawn.

The `spawn()` plugin executes a child FSM with the name specified by `format`.
This child FSM is executed `n` times. The child FSM copies all variables from
the parent FSM.


### Module: tg

The `tg` module provides a _template grammar_ for executing more specific
protocol formats such as specific HTTP variants, FTP, POP3, DNS, etc. Many
settings for these models are hardcoded and are not extensible within the MAR
file.

The following formats are available:

- `http_request_keep_alive`
- `http_response_keep_alive`
- `http_request_close`
- `http_response_close`
- `pop3_message_response`
- `pop3_password`
- `http_request_keep_alive_with_msg_lens`
- `http_response_keep_alive_with_msg_lens`
- `http_amazon_request`
- `http_amazon_response`
- `ftp_entering_passive`
- `dns_request`
- `dns_response`


#### `tg.send()`

Arguments:

1. `grammar:string` is the name of the grammar to execute.

The `send()` plugin executes the grammar specified by the `grammar` argument.

`recv()` is the counter action to `send()`.


#### `tg.recv()`

Arguments:

1. `grammar:string` is the name of the grammar to execute.

The `recv()` plugin executes the grammar specified by the `grammar` argument.

`send()` is the counter action to `recv()`.


## Plugin Development

All plugins exist as Go functions within subpackages of the `plugins` package.
Adding a new plugin requires two steps:

1. Write the plugin function.
2. Register it with [`RegisterPlugin()`](https://godoc.org/github.com/redjack/marionette#RegisterPlugin).

### Writing a plugin

All plugin functions implement the [`PluginFunc`](https://godoc.org/github.com/redjack/marionette#PluginFunc)
interface:

```go
type PluginFunc func(ctx context.Context, fsm FSM, args ...interface{}) error
```

Let's say we want to build a `test.echo()` plugin that simply prints our
arguments to STDOUT. First we'll create a `plugins/test` package and we can
write our function like this:

```go
func Echo(ctx context.Context, fsm marionette.FSM, args ...interface{}) error {
	if len(args) < 1 {
		return errors.New("not enough arguments")
	}

	fmt.Println(args...)
	return nil
}
```

### Registering a plugin

Next we let `marionette` know about the plugin by registering it. We'll do this
by adding an `init()` function with the call to `RegisterPlugin()` so it is 
included any time we import our `plugins/test` package:

```
package test

func init() {
	marionette.RegisterPlugin("test", "echo", Echo)
}
```

We'll need to also import our package in the `plugins/plugins.go` file which is
imported by the `cmd/marionette` binary:

```
import _ "github.com/redjack/marionette/plugins/test"
```


### Invoking our plugin

Now we can write an _action_ line using our new plugin:

```
connection(tcp, 8000):
  start echo NULL 1.0
  echo  end  echo 1.0

action echo:
  client test.echo("hello", "world")
```


## Installing new build-in formats

When adding new formats, you'll need to first install `go-bindata`:

```sh
$ go get -u github.com/jteeuwen/go-bindata/...
```

Then you can use `go generate` to convert the asset files to Go files:

```sh
$ go generate ./...
```

To install the original [marionette][] library for comparing tests, download
the latest version, unpack the archive and run:


## Testing

Use the built-in go testing command to run the unit tests:

```sh
$ go test ./...
```

If you have the original Python marionette installed then you can run tests
of the ports using the `python` tag:

```sh
$ go test -tags python ./regex2dfa
$ go test -tags python ./fte
```


## References

Below are some references to see how these plugins are used in practice within
the built-in formats. All formats can be found in the `mar/formats` directory.

- `http_simple_blocking`: Uses `fte.send()` calls to send & receive HTTP
  formatted messages. Messages are sent and received continuously—even when
  there is no available data to be sent.

- `http_simple_nonblocking`: Uses `fte.send_async()` to send & receive HTTP
  formatted messages. Messages are only sent when there is data available.

- `ftp_simple_blocking`: Uses the `tg` plugins to send FTP formatted messages
  and embed data to connect to random ports created by the `channel.bind()`
  plugin and managed by the `model.spawn()` plugin.

- `http_active_probing2`: Uses the `regex_match_incoming()` directive to
  conditionally send responses based on what data is sent to the server.

- `https_simple_blocking`: Uses the `io` plugins to hardcode the TLS handshake
  and then uses the `fte` plugins to exchange data.

