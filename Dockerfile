FROM alpine:edge as builder

RUN apk add --no-cache --progress git make go
ADD . /proxy
RUN export GOPROXY="https://proxy.golang.org,direct" && cd /proxy && go mod download -x && make

FROM alpine:latest

EXPOSE 6000

ENTRYPOINT ["/proxy/bin/gnet-proxy", "--mode", "2", "--backends", "127.0.0.1:4000"]