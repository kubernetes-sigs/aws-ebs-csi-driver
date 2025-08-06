#!/usr/bin/env bash

while true; do 
	sleep 30s; echo; echo "###### HEARTBEAT $(date)";
	ps -aux
	echo "###### END"; echo;
done &

go get -t -v ./cmd/... ./pkg/... ./tests/sanity/...
go test -v -race ./pkg/... | cat

sleep 100s
