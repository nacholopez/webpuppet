FROM golang:1.13.1
COPY src/ /go/src/
WORKDIR  /go
RUN go install webpuppet

FROM ubuntu:bionic
COPY --from=0 /go/bin/webpuppet /
CMD ["/webpuppet"]
