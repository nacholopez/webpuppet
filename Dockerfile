FROM golang:1.18.0-alpine AS build
WORKDIR /a
COPY go.mod go.sum webpuppet.go ./
RUN CGO_ENABLED=0 go build -trimpath

FROM alpine:3.14
RUN adduser -D webpuppet
COPY --from=build /a/webpuppet /webpuppet
USER webpuppet
CMD ["/webpuppet"]
