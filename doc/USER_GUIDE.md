User Guide
==========

This guide provides instructions on how to install and run the `marionette`
command line interface. For more detailed information about Marionette, please
refer to the [Design Document](DESIGN.md) and [Developer Manual](DEVELOPER_MANUAL.md).


## Installation

To install and build marionette, you will need to download and install the Go
toolchain: https://golang.org/dl/

Once your Go installation is set up, fetch the `marionette` source into your
`GOPATH`, install and build your dependencies, and then build and install
`marionette`:

Three dependencies are required. Two of them
are in the `third_party` directory and the third one can be downloaded from
the web.

### If You Are Installing on CentOS

Ensure you have a C/C++ compiler installed & have the proper version of `shasum`
installed:

```sh
$ yum group install -y "Development Tools"
$ yum install -y perl-Digest-SHA
```

### Installing OpenFST

You must use the included `third_party/openfst` implementation. Also note that
static builds must be enabled via the `--enable-static` flag.

```sh
$ cd third_party/openfst
$ ./configure --enable-static=yes
$ make
$ sudo make install
```

### Installing re2

You must use the included `third_party/re2` implementation:

```sh
$ cd third_party/re2
$ make
$ sudo make install
```

### GMP

Download the latest version of [GMP][], unpack the
archive and run:

```sh
$ wget https://gmplib.org/download/gmp/gmp-6.1.2.tar.bz2
$ tar -xvjf gmp-6.1.2.tar.bz2
$ cd gmp-6.1.2

$ ./configure --enable-cxx
$ make
$ sudo make install
$ make check
```

### Building the Marionette Binary

First, make sure you have installed Go from [https://golang.org/][go]. Next,
install `dep` using [these instructions][dep].

Finally, retrieve the source, update project dependencies, and install the
`marionette` binary:

```sh
$ go get github.com/redjack/marionette
$ cd $GOPATH/src/github.com/redjack/marionette
$ dep ensure
$ go install ./cmd/marionette
```

The `marionette` binary is now installed in your `$GOPATH/bin` folder.

[marionette]: https://github.com/marionette-tg/marionette
[GMP]: https://gmplib.org
[go]: https://golang.org/
[dep]: https://github.com/golang/dep#installation

## Installing new build-in formats

When adding new formats, you'll need to first install `go-bindata`:

```sh
$ go get -u github.com/jteeuwen/go-bindata/...
```

Then you can use `go generate` to convert the asset files to Go files:

```sh
$ go generate ./...
```

## View available subcommands

The `marionette` binary is composed of multiple subcommands to provide different
functionality in a single, easily distributable binary. You can see a list of
subcommands by executing `marionette` without any arguments:

```sh
$ marionette
Marionette is a programmable client-server proxy that enables the user to
control network traffic features with a lightweight domain-specific language.

Usage:

	marionette command [arguments]

The commands are:

	client    runs the client proxy
	formats   show a list of available formats
	pt-client runs the client proxy as a PT
	pt-server runs the server proxy as a PT
	server    runs the server proxy

```


## Listing available formats

To view a list of prebuilt, included MAR formats, run the `formats` subcommand:

```sh
$ marionette formats
active_probing/ftp_pureftpd_10:20150701
active_probing/http_apache_247:20150701
active_probing/ssh_openssh_661:20150701
dns_request:20150701
dummy:20150701
ftp_simple_blocking:20150701
http_active_probing2:20150701
http_active_probing:20150701
http_probabilistic_blocking:20150701
http_simple_blocking:20150701
http_simple_blocking:20150702
http_simple_blocking_with_msg_lens:20150701
http_simple_nonblocking:20150701
http_squid_blocking:20150701
https_simple_blocking:20150701
nmap/kpdyer.com:20150701
smb_simple_nonblocking:20150701
ssh_simple_nonblocking:20150701
ta/amzn_sess:20150701
udp_test_format:20150701
web_sess443:20150701
web_sess:20150701
```


## Running the server

The server component should be started first when setting up `marionette`. You 
can view the available options by passing the `-h` option.

```sh
$ marionette server -h
Usage of marionette-server:
  -bind string
    	Bind address
  -debug string
    	debug http bind address
  -format string
    	Format name and version
  -proxy string
    	Proxy IP and port
  -sleep-factor float
    	model.sleep() multipler (default 1)
  -socks5
    	Enable socks5 proxying
  -trace-path string
    	stream trace directory path
  -v	Debug logging enabled
```


The `-format` parameter specifes the format to execute against. This can be
with or without the version number (e.g. `http_simple_blocking` or
`http_simple_blocking:20150701`). The client _must_ use the same format when
connecting to the server.

The `-bind` parameter allows you to specify the IP address to open the listener
on. By default, marionette will listen on all available IP addresses on the
local system.

The `-proxy` and `-socks5` arguments are mutually exclusive. If you specify a
proxy hostport, then all requests will make a single connection to that proxied
server. For example, specify `-proxy google.com:80` will send all requests to
`google.com` on port `80`. If you specify `-socks5` then the server command will
act as a SOCKS5 proxy.

The `-debug`, `-v`, `-trace-path`, and `-sleep-factor` are all used for
debugging and end users will not require these unless instructed to do so by a
developer for troubleshooting.

#### Examples

```sh
# Single hostport proxy
$ marionette server -format http_simple_blocking -proxy google.com:80
```

```sh
# SOCKS5 proxy
$ marionette server -format http_simple_blocking -socks5
```

```sh
# Bind to a specific IP address.
$ marionette server -format http_simple_blocking -bind 127.0.0.1 -proxy localhost:8000
```


## Running the client

The client component should be started after the server is running. You can view
the available options by passing the `-h` option.

```sh
$ marionette client -h
Usage of marionette-client:
  -bind string
    	Bind address (default "127.0.0.1:8079")
  -debug string
    	debug http bind address
  -format string
    	Format name and version
  -server string
    	Server IP address (default "127.0.0.1")
  -sleep-factor float
    	model.sleep() multipler (default 1)
  -trace-path string
    	stream trace directory path
  -v	Debug logging enabled
```

The `-format` parameter specifes the format to execute against. This can be
with or without the version number (e.g. `http_simple_blocking` or
`http_simple_blocking:20150701`). You _must_ use the same format as what is
specified by the server.


The `-bind` parameter specifies the hostport where the client will listen for
incoming connections. By default it uses port `8079` on the `127.0.0.1` IP
address. This is the hostport you should pass to your end user application such
as `curl` or your browser's SOCKS5 settings.

The `-server` parameter specifies the hostname or IP address of the server. The
port number _should not_ be specified as this is derived from the `connection()`
string in the MAR format.

The `-debug`, `-v`, `-trace-path`, and `-sleep-factor` are all used for
debugging and end users will not require these unless instructed to do so by a
developer for troubleshooting.

#### Examples

```sh
# Listen on all IP addresses on port 2000.
$ marionette client -format http_simple_blocking -bind :2000
```

```sh
# Connect to marionette server running at mydomain.com.
$ marionette client -format http_simple_blocking -server mydomain.com
```


## Demo

### HTTP-over-FTP

In this example, we'll mask our HTTP traffic as FTP packets.

First, follow the installation instructions above on your client & server machines.

Start the server proxy on your server machine and forward traffic to a server
such as `google.com`.

```sh
$ marionette server -format ftp_simple_blocking -proxy google.com:80
listening on [::]:2121, proxying to google.com:80
```

Start the client proxy on your client machine and connect to your server proxy.
Replace `$SERVER_IP` with the IP address of your server.

```sh
$ marionette client -format ftp_simple_blocking -server $SERVER_IP
listening on 127.0.0.1:8079, connected to <SERVER_IP>
```

Finally, send a `curl` to `127.0.0.1:8079` and you should see a response from
`google.com`:

```sh
$ curl 127.0.0.1:8079
```


### Testing the Binary

First start the server proxy on your server machine and forward traffic to a server
such as `google.com`.

```sh
$ marionette server -format http_simple_blocking -proxy google.com:80
listening on [::]:8081, proxying to google.com:80
```
This has launched the server process.  The server is now waiting for a client connection.

Leave the server process running and start the client proxy on your client machine and connect to your server proxy.
Replace `$SERVER_IP` with the IP address of your server.

```sh
$ marionette client -format http_simple_blocking -server $SERVER_IP
listening on 127.0.0.1:8079, connected to <SERVER_IP>
```
Now the client process has started and is waiting for traffic at 127.0.0.1:8079 to forward to the server.

Finally, send a `curl` to `127.0.0.1:8079` and you should see a response from
`google.com`:

```sh
$ curl 127.0.0.1:8079
```

#### Browser Setup

Testing Marionette is best done through the Firefox browser.  If you do not have a copy of Firefox, download it [here](https://www.mozilla.org/en-US/firefox/new/).

#### Activate the Proxy

Go to:

``Firefox > Preferences > General > Network Proxy``

- Set the proxy button to Manual Proxy Configuration.
- Set the SOCKS host to the machine to the incoming port on the Marionette client (Probably localhost and port 8079)
- Make sure that the SOCKS v5 Radio button is depressed.
- Check the box marked "Proxy DNS when using SOCKS v5"

#### Secure the DNS

Although the code can work through the proxy with the above data, Firefox does not yet have its DNS fully going through the proxy.  To fix this:

- Type about:config into the search bar.  This will open the advanced settings for the browser.
- Go to the term media.peerconnection.enabled 
- Set it to false by double clicking on it.

#### Testing the Browser

First start the server proxy on your server machine and forward traffic to a server
such as `google.com`.

```sh
$ marionette server -format http_simple_blocking -socks5
listening on [::]:8081, proxying via socks5
```
Note that, unlike before, we do not use a -proxy command line option, but instead use -socks5 option.  This starts a general socks5 proxy server that internet connections can pass through.

This has launched the server process.  The server is now waiting for a client connection.

Leave the server process running and start the client proxy on your client machine and connect to your server proxy.
Replace `$SERVER_IP` with the IP address of your server.

```sh
$ marionette client -format http_simple_blocking -server $SERVER_IP
listening on 127.0.0.1:8079, connected to <SERVER_IP>
```
Now the client process has started and is waiting for traffic at 127.0.0.1:8079 to forward to the server.

Now start the Firefox browser as earlier configured.  You will now be able to surf the web.



### Browser Demo

Testing Marionette is best done through the Firefox browser.  If you do not have a copy of Firefox, download it [here](https://www.mozilla.org/en-US/firefox/new/).

#### Activate the Proxy

Go to:

``Firefox > Preferences > General > Network Proxy``

- Set the proxy button to Manual Proxy Configuration.
- Set the SOCKS host to the machine to the incoming port on the Marionette client (Probably localhost and port 8079)
- Make sure that the SOCKS v5 Radio button is depressed.
- Check the box marked "Proxy DNS when using SOCKS v5"

#### Secure the DNS

Although the code can work through the proxy with the above data, Firefox does not yet have its DNS fully going through the proxy.  To fix this:

- Type about:config into the search bar.  This will open the advanced settings for the browser.
- Go to the term media.peerconnection.enabled 
- Set it to false by double clicking on it.

#### Installation (Docker)

For this demo, please we will use the v0.1 Docker image. You'll need to have Docker
installed. You can find instructions for specific operating system here:
https://docs.docker.com/install

Once docker is installed, then download the appropriate docker file from the v0.1 release of Marionette.  The file can be found here:

https://github.com/redjack/marionette/releases/tag/v0.1

To install the docker file in docker:

```
$ gunzip redjack-marionette-0.1.gz
```
```
$ docker load -i redjack-marionette-0.1
```

#### Running using the Docker image

Next, run the Docker image and use the appropriate port mappings for the
Marionette format you're using. `http_simple_blocking` uses
port `8081`:

```sh
$ docker run -p 8081:8081 redjack/marionette server -format http_simple_blocking
```

```sh
$ docker run -p 8079:8079 redjack/marionette client -bind 0.0.0.0:8079 -format http_simple_blocking
```

If you're running _Docker for Mac_ then you'll also need to add a `-server` argument:

```sh
$ docker run -p 8079:8079 redjack/marionette client -bind 0.0.0.0:8079 -server docker.for.mac.host.internal -format http_simple_blocking
```

Start wireshark on the loopback network and watch the packets.

(Note, if wireshark is not displaying the packets as HTTP, go to:

``WireShark > Preferences > Protocols > HTTP``
 
 and add port 8081 to the port list.


#### Surf

Look at your favorite webpage(s).  The system is fairly reliable now, but in the event that the connection drops, then:

- Stop the server and the client
- Restart the server and the client (in order)
- Refresh the page
- Report the error
