
## Sample configuration for nginx

Please run `make` in the current directory to generate the "large" file response.

**NOTE**: Unlike the other samples in this repository, this implementation/configuration is not standalone. In order to receive HTTP POSTs, a separate process is required. The configuration supplied uses the swift or go implementation's of the `/slurp` endpoint.

```
networkQuality -C https://networkquality.example.com:8443/config
==== SUMMARY ====
Upload capacity: 32.735 Mbps
Download capacity: 2.182 Gbps
Upload flows: 12
Download flows: 12
Responsiveness: Medium (440 RPM)
```