GOBIN := $(shell pwd)/bin

default:
	go build -o $(GOBIN)/gnet-proxy main.go