
## A Swift implementation of the Network Quality Server

### Building

#### Using the command line

1. `swift build [-c release]`
1. Use the produced `networkQualityd` binary from the command line, either by running `swift run [-c release] [networkQualityd] [arguments]` or by invoking the binary directly at `.build/[release|debug]/networkQualityd`

### Running

#### Usage:

```
USAGE: server [--bind-host <bind-host>] [--http-port <http-port>] [--https-port <https-port>] [--cert-file <cert-file>] [--key-file <key-file>] [--public-name <public-name>] [--allow-half-closure] [--no-allow-half-closure]

OPTIONS:
  --bind-host <bind-host> IP to bind to (default: 127.0.0.1)
  --http-port <http-port> http port (default: 4040)
  --https-port <https-port>
                          https port (default: 4043)
  --cert-file <cert-file> cert file
  --key-file <key-file>   key file
  --public-name <public-name>
                          public name
  -a, --allow-half-closure/--no-allow-half-closure
                          allow half closure (default: true)
  -h, --help              Show help information.
```

#### Example run:

```
./.build/release/networkQualityd --cert-file networkquality.example.com.pem --key-file networkquality.example.com-key.pem --public-name networkquality.example.com
2021-09-07T13:46:55-0700 info com.example.networkqualityd.main : h2 Listening at [IPv4]127.0.0.1/127.0.0.1:4043
2021-09-07T13:46:55-0700 info com.example.networkqualityd.main : http Listening at [IPv4]127.0.0.1/127.0.0.1:4040
2021-09-07T13:46:55-0700 info com.example.networkqualityd.main : Network Quality URL: https://networkquality.example.com:4043/config
```

##### Running client against server
```
networkQuality -C https://networkquality.example.com:4043/config
==== SUMMARY ====                                                                                         
Upload capacity: 73.213 Mbps
Download capacity: 4.269 Gbps
Upload flows: 12
Download flows: 12
Responsiveness: Medium (829 RPM)
```
