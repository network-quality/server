## General Description:
This is a service that will be used to measure various aspects of a users's connection to the Internet. The client will open a number of simultaneous HTTP connections (a combination of GETs and POSTs) attempting to saturate the users's network connection and collect data-points during these connections. The test is expected to take about 15 seconds and will close any connections that are still transferring at the end of the test period.

Service Protocol Requirements: All requests will be with TLSv1.3 over HTTP/2

## There are 4 URLs/responses required to be implemented for this service.

###  A "config" URL/response.
Description: This is the configuration file/format used by the client. It's a simple JSON file format that points the client at the various URLs mentioned below.

Method: `GET`

Sample URL: `https://networkquality.example.com/api/v1/config`

Sample Contents:
```
{
  "version": 1,
  "urls": {
    "small_download_url": "https://networkquality.example.com/api/v1/small",
    "large_download_url": "https://networkquality.example.com/api/v1/large",
    "upload_url": "https://networkquality.example.com/api/v1/upload"
  }
}
```

If the request cannot be serviced, the server should return a 429 or an appropriate 50x. If possible, set the response header `Retry-After` with an integer to indicate how many seconds later the request should be tried again or an HTTP Date string. See [here](https://datatracker.ietf.org/doc/html/rfc7231#section-7.1.3) for more info.

### A "small" URL/response.
Description: This needs to serve a status code of 200 and 1 byte in the body. The actual body content is irrelevant.

Method: `GET`

Config file field name: `small_download_url`

Sample URL: `https://networkquality.example.com/api/v1/small`

If the request cannot be serviced, the server should return a 429 or an appropriate 50x. The `Retry-After` header is not required for this response.

### A "large" URL/response.
Description: This needs to serve a status code of 200 and a body size of at least 8GB. The body can be bigger, and will need to grow as network speeds increases over time. The actual body content is irrelevant. The client will probably never completely download the object.

Method: `GET`

Config file field name: `large_download_url`

Sample URL: `https://networkquality.example.com/api/v1/large`

If the request cannot be serviced, the server should return a 429 or an appropriate 50x. The `Retry-After` header is not required for this response.

###  An "upload" URL/response.
Description: This needs to handle a POST request with an arbitrary body size. Nothing needs to be done with the payload, it should be discarded.

Method: POST

Config file field name: `upload_url`

Sample URL: `https://networkquality.example.com/api/v1/upload`

If the request cannot be serviced, the server should return a 429 or an appropriate 50x. The `Retry-After` header is not required for this response.

### Other items to note:
* There should not be any content-encoding applied to the response.
* Redirects will be ignored and will trigger failure by the client.
