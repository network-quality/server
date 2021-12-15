
## A Go implementation of the Network Quality Server

### Building (requires Go 1.13+)

#### Using the command line

1. `go build networkqualityd.go`
1. Use the produced `networkQualityd` binary from the command line

### Running

#### Usage:

```
Usage of ./networkqualityd:
  -base-port int
    	The base port to listen on (default 4043)
  -cert-file string
    	cert to use
  -debug
    	enable debug mode
  -domain string
    	domain to generate config for (default "networkquality.example.com")
  -key-file string
    	key to use
  -listen-addr string
    	address to bind to (default "localhost")
  -public-name string
    	host to generate config for
  -template string
    	template json config (default "config.json.in")
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
