FROM golang:1.22-bullseye

RUN apt-get -y -q update && \
     apt-get -y -q install software-properties-common && \
     apt-get install -qqy \
         dos2unix \
         default-mysql-client \
         psmisc \
         vim

ADD . /proxy
RUN export GOPROXY="https://proxy.golang.org,direct" && cd /proxy && go mod download -x && make

EXPOSE 6000
EXPOSE 3080

ENTRYPOINT ["/proxy/bin/gnet-proxy", "--mode 2", "--backends 127.0.0.1:4000"]