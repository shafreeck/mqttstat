#!/bin/bash

version=`awk '/Version/{gsub("\"", "", $4); print $4}' version.go`

pkg=./mqttstat-v$version
mkdir -p $pkg

echo build $pkg/mqttstat
env GOOS=linux go build -o $pkg/mqttstat

echo build $pkg/mqttstat.darwin
env GOOS=darwin go build -o $pkg/mqttstat.darwin

echo build $pkg/mqttstat.exe
env GOOS=windows go build -o $pkg/mqttstat.exe

tar -zcf $pkg.tar.gz $pkg
