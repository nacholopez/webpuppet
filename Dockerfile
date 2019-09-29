FROM golang:1.13.1
COPY src/ /go/src/
WORKDIR  /go
RUN go get webpuppet
RUN go install webpuppet

FROM ubuntu:bionic
RUN adduser --system --quiet --group webpuppet
COPY --from=0 /go/bin/webpuppet /
USER webpuppet
CMD ["/webpuppet"]
