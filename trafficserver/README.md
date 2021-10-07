# Sample configuration for Apache Traffic Server

This requires a the latest version of Traffic Server (currently 9.1.0) . The sample configuration uses the [generator](https://docs.trafficserver.apache.org/en/9.1.x/admin-guide/plugins/generator.en.html) plugin and the statichit plugin to generate the download bytes, receive POST requests and serve the configuration file. You can find source downloads [here](https://trafficserver.apache.org/downloads).

* Copy the contents of `conf` to `TS_ROOT/etc/trafficserver`

* Modify `remap.config`, adding:
```
.include networkquality.config
```

* Modify `ssl_multicert.config` and include the certificate for your test endpoint. eg
```
ssl_cert_name=networkquality.example.com.ecdsa
```

* Reload the Traffic Server configs (`traffic_ctl config reload` or start the `trafficserver` service).
