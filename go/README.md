
## A Go implementation of the Network Quality Server

### Building (requires Go 1.17+)

#### Using the command line

1. `go build -o networkqualityd .`
1. Use the produced `networkqualityd` binary from the command line

### Running

#### Usage:

```
Usage of ./networkqualityd:
  -announce
        announce this server using DNS-SD
  -cert-file string
        cert to use
  -config-name string
        domain to generate config for (default "networkquality.example.com")
  -context-path string
        context-path if behind a reverse-proxy
  -debug
        enable debug mode
  -enable-cors
        enable CORS headers
  -enable-h2c
        enable h2c (non-TLS http/2 prior knowledge) mode
  -enable-http2
        enable HTTP/2 (default true)
  -enable-http3
        enable HTTP/3
  -insecure-public-port int
        The port to listen on for HTTP measurement accesses
  -key-file string
        key to use
  -listen-addr string
        address to bind to (default "localhost")
  -public-name string
        host to generate config for (same as -config-name if not specified)
  -public-port int
        The port to listen on for HTTPS/H2C/HTTP3 measurement accesses (default 4043)
  -socket-send-buffer-size uint
        The size of the socket send buffer via TCP_NOTSENT_LOWAT. Zero/unset means to leave unset
  -template string
        template json config (default "config.json.in")
  -tos string
        set TOS for listening socket (default "0")
```

#### Example run:

```
./networkqualityd --cert-file networkquality.example.com.pem --key-file networkquality.example.com-key.pem --public-name networkquality.example.com
2021/09/07 14:39:05 Network Quality URL: https://networkquality.example.com:4043/config
```

##### Running client against server:
```
networkQuality -C https://networkquality.example.com:4043/config
==== SUMMARY ====
Upload capacity: 73.213 Mbps
Download capacity: 4.269 Gbps
Upload flows: 12
Download flows: 12
Responsiveness: Medium (829 RPM)
```

### Docker

The server can be run in a docker container. The `Dockerfile` in this repository
will generate a container that can run the server. To build the container,
simply execute

```
docker build -t rpmserver .
```

The command will generate a container image named `rpmserver`.

In order to run the resulting container, you will have to either accept some
default values or provide configuration. The server requires access
to a public/private key for its SSL connections, and the `Dockerfile` does not
specify that they be copied in to the image. In other words, you will have to configure a
shared volume between the executing container and the host, where that volume
contains the key files.

The container executing the RPM server will also need to have a port map
established. You will have to publish the port on which the server in the
container is listening to the host.

Assuming that you use the default values specified in the `Dockerfile`, you can
run the container using

```
docker run --env-file docker_config.env  -v $(pwd)/live:/live -p 4043:4043 -p rpmserver
```

where there exists a directory `$(pwd)/live` that contains two files named
`fullchain.pem` and `privkey.pem` that hold the public and private keys for
the SSL connections, respectively.

You can use environment variables to configure any of the `networkqualityd` command-line options.

| Command-line option name | Environment variable name |
| -- | -- |
| `-cert-file` | `cert_file` |
| `-key-file` | `key_file` |
| `-public-port` | `public_port` |
| `-config-name` | `config_name` |
| `-listen-addr` | `listen_addr` |
| `-public-name` | `public_name` |
| `-template` | `template` |
| `-debug` | *see below* |

If you want to configure whether the server runs in debug mode, simply set the `debug` environment variable to `-debug`. If you enable debugging, you will also need to create a map between a port on the host and port 9090 on the container (e.g., `-p 9090:9090`).

There is `docker_config.env` in this directory that you can
use to make passing those configuration options to the container
easier. To use this file, add the `--env-file docker_config.env` arguments to the `docker run` command.
