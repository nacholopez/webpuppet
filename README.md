## webpuppet

### Introduction and motivation

webpuppet is a simple http daemon which behavior can be controlled by http requests.

webpuppet was mainly developed to help me debugging [kubernetes termination lifecycle](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-terminating-with-grace) but it can also be used to:
* Inspect HTTP Headers and Body received by webpuppet.
* Test forward and reverse proxy behavior.

### Available endpoints

* `/sleep/{seconds}` Request takes {seconds} to get a response. Supported methods: `GET` and `POST`.
* `/mirror` Returns the Headers and the Body received from the HTTP request. Supported methods: `GET` and `POST`.
* `/httpResponseCode/{code}` Returns a HTTP response with the HTTP code set by {code}. `GET` and `POST`.
* `/sleepms/max/{maxms}/min/{minms}/messagelength/max/{maxML}/min/{minML}` Response takes a random number of milliseconds between {maxms} and {minms}. The response also contains a json payload with a random length between {maxML} and {minML}. Supported methods: `GET` and `POST`.
* `/print/stderr` Dumps the body payload into stderr. Supported method: `POST`.
* `/print/stdout` Dumps the body payload into stdout. Supported method: `POST`.
* `/health` Health endpoint, mainly used by kubernetes probes. 
Supported method: `GET`.

### Configuring

webpuppet can be configured by using the following environment variables

* `PORT` Listening port, defaults to 8080.
* `BOOT_WAIT_SECS`. Number of seconds to wait before the application starts accepting remote connections and becomes healthy.
* `LOG_LEVEL`. Logging level, defaults to Info.

### Building

1. `git clone https://github.com/nacholopez/webpuppet.git`
2. `cd webpuppet`
3. `go build`
4. `./webpuppet`

### Docker image

A webpuppet Docker image can be run by `docker run --rm -ti k8stools/webpuppet`