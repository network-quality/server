
# Network Quality Server

## Welcome!
The _Network Quality Server_ project was created as a place to share example servers that can used by the `networkQuality` command line tool available in macOS 12.

You can find a textual description of what a server needs to implement [here](SERVER_SPEC.md)

There’s also 2 complete reference server implementations, one written in [Swift](swift/README.md) and the other in [Go](go/README.md).  Also provided are configurations for HTTP server/proxy servers: [Apache Traffic Server](trafficserver/README.md), [Apache HTTPD](httpd/README.md) and [nginx](nginx/README.md).

All the samples require an SSL certificate to run.

## Using networkQuality against your server
The `networkQuality` CLI takes a `-C` switch allowing you to point it at your server.

```
user@myhost ~ % networkQuality -C https://networkquality.example.com:8443/config
==== SUMMARY ====
Upload capacity: 29.662 Mbps
Download capacity: 541.622 Mbps
Upload flows: 12
Download flows: 12
Responsiveness: High (1825 RPM)
```

**NOTE**: The `networkQuality` CLI tool will only connect to a server presenting a valid SSL certificiate. If you are using a custom CA, ensure the CA is trusted by the system.

There are more options available to affect behavior of this utility. See the manpage of `networkQuality` for more info.

## Contributing
Please review [_how to contribute_](CONTRIBUTING.md) if you would like to submit a pull request.

## Asking Questions and Discussing Ideas
If you have any questions you’d like to ask publicly, or ideas you’d like to discuss, please [_raise a GitHub issue_](https://github.com/network-quality/server/issues).
##
## Project Maintenance
Project maintenance involves, but not limited to, adding clarity to incoming [_issues_](https://github.com/network-quality/server/issues) and reviewing pull requests. Project maintainers can approve and merge pull requests. Reviewing a pull request involves judging that a proposed contribution follows the project’s guidelines, as described by the [_guide to contributing_](CONTRIBUTING.md).

Project maintainers are expected to always follow the project’s [_Code of Conduct_](CODE_OF_CONDUCT.md), and help to model it for others.

## Project Governance
Although we expect this to happen very infrequently, we reserve the right to make changes, including changes to the configuration format and scope, to the project at any time.

