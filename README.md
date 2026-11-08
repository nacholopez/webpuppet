## webpuppet

### Introduction and motivation

webpuppet is a simple http daemon which behavior can be controlled by http requests.

webpuppet was mainly developed to help me debugging [kubernetes termination lifecycle](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-terminating-with-grace) but it can also be used to:
* Inspect HTTP Headers and Body received by webpuppet.
* Test forward and reverse proxy behavior.

> webpuppet is a debugging tool. `/mirror` echoes every request header back (including `Authorization` and cookies) and `/print/*` writes request bodies to the container logs, so don't put it in the path of real user traffic.

### Available endpoints

| Endpoint | Methods | Description |
|---|---|---|
| [`/health`](#health) | `GET` | Health endpoint, mainly used by kubernetes probes. |
| [`/sleep/{seconds}`](#sleepseconds) | `GET`, `POST` | Waits {seconds} (0-3600) before responding. |
| [`/sleepms/...`](#sleepmsmaxmaxmsminminmsmessagelengthmaxmaxmlminminml) | `GET`, `POST` | Waits a random number of milliseconds and replies with a random-length payload. |
| [`/httpResponseCode/{code}`](#httpresponsecodecode) | `GET`, `POST` | Replies with the HTTP status code {code} (100-599). |
| [`/mirror`](#mirror) | `GET`, `POST` | Returns the headers and body of the request. |
| [`/print/stdout`](#printstdout-and-printstderr) | `POST` | Writes the request body to stdout. |
| [`/print/stderr`](#printstdout-and-printstderr) | `POST` | Writes the request body to stderr. |

Invalid path values return `400 Bad Request`, and a method not listed for an endpoint returns `405 Method Not Allowed`. Request bodies are limited to 1 MiB.

The examples below assume webpuppet is listening on `localhost:8080`.

#### `/health`

```console
$ curl -i localhost:8080/health
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8

OK
```

#### `/sleep/{seconds}`

The request takes {seconds} (0-3600) to get a response. Handy for checking that in-flight requests complete while a pod is terminating.

```console
$ curl -i localhost:8080/sleep/2
HTTP/1.1 200 OK
Content-Type: application/json

{ "msg": "Slept for 2 seconds" }
```

If the request has a `TraceID` header, its value is included in the debug logs (`LOG_LEVEL=DEBUG`) so you can follow a single request:

```console
$ curl -H 'TraceID: req-42' localhost:8080/sleep/0
{ "msg": "Slept for 0 seconds" }
```

```text
DEBUG  webpuppet.go:124 [req-42] Sleeping for 0
DEBUG  webpuppet.go:128 [req-42] Slept for 0
```

Out-of-range or non-numeric values are rejected:

```console
$ curl -i localhost:8080/sleep/abc
HTTP/1.1 400 Bad Request

Invalid seconds value
```

#### `/sleepms/max/{maxms}/min/{minms}/messagelength/max/{maxML}/min/{minML}`

The response takes a random number of milliseconds between {minms} and {maxms} (inclusive, 0-3600000). The response contains a random string with a length between {minML} and {maxML} (inclusive, 0-1048576). Useful for simulating a backend with variable latency and payload size.

```console
$ curl localhost:8080/sleepms/max/200/min/50/messagelength/max/20/min/10
{ "msg": "dVXfvfNsrFMCOoiP" }
```

When the chosen message length is 0, the response reports the time slept instead:

```console
$ curl localhost:8080/sleepms/max/500/min/100/messagelength/max/0/min/0
{ "msg": "Slept for 353 milliseconds" }
```

Set min and max to the same value for a fixed delay or length. A min greater than its max returns `400 Bad Request`.

#### `/httpResponseCode/{code}`

Replies with the HTTP status code {code} (100-599). Useful for testing how proxies, load balancers or clients handle errors.

```console
$ curl -i localhost:8080/httpResponseCode/503
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{ "code": "503" }
```

```console
$ curl -i localhost:8080/httpResponseCode/600
HTTP/1.1 400 Bad Request
Content-Type: application/json

{ "error": "invalid code" }
```

#### `/mirror`

Returns the headers and the body exactly as webpuppet received them. Put it behind a proxy to see which headers the proxy adds, removes or rewrites. The response is always `text/plain`.

```console
$ curl -H 'X-Request-Id: abc123' -H 'Content-Type: application/json' \
       -d '{"hello":"world"}' localhost:8080/mirror
HEADERS
=======
User-Agent: curl/8.21.0
Accept: */*
X-Request-Id: abc123
Content-Type: application/json
Content-Length: 17

BODY
====
{"hello":"world"}
```

The `BODY` section is omitted when the request has no body. Header order may vary.

#### `/print/stdout` and `/print/stderr`

Write the request body, followed by a newline, to the container's stdout or stderr. The response is an empty `200 OK`. Useful for testing log collection.

```console
$ curl -d 'hello from stdout' localhost:8080/print/stdout
$ curl -d 'hello from stderr' localhost:8080/print/stderr
```

```console
$ docker logs webpuppet
hello from stdout
hello from stderr
```

### Configuring

webpuppet can be configured by using the following environment variables

* `PORT` Listening port, defaults to 8080.
* `BOOT_WAIT_SECS`. Number of seconds to wait before the application starts accepting remote connections and becomes healthy.
* `LOG_LEVEL`. Logging level, defaults to Info. Set it to `DEBUG` to log every sleep.

```console
$ docker run --rm -p 9090:9090 -e PORT=9090 -e BOOT_WAIT_SECS=5 -e LOG_LEVEL=DEBUG k8stools/webpuppet
```

On `SIGTERM` or `SIGINT`, webpuppet stops accepting new connections and waits up to 60 seconds for in-flight requests to finish.

### Building

1. `git clone https://github.com/nacholopez/webpuppet.git`
2. `cd webpuppet`
3. `go build`
4. `./webpuppet`

### Testing

```console
$ go test ./...
```

### Docker image

A webpuppet Docker image can be run by `docker run --rm -ti -p 8080:8080 k8stools/webpuppet`
