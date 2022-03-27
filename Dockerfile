FROM golang:1.18.0-alpine3.14 AS build
WORKDIR /a
COPY go.mod go.sum webpuppet.go ./
RUN go build

FROM alpine:3.14
RUN adduser -D webpuppet
COPY --from=build /a/webpuppet /webpuppet
USER webpuppet
CMD ["/webpuppet"]
