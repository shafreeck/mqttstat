#!/bin/bash

mkdir -p ./mqttstat-release
echo build ./mqttstat-release/mqttstat.linux
env GOOS=linux go build -o ./mqttstat-release/mqttstat.linux

echo build ./mqttstat-release/mqttstat.darwin
env GOOS=darwin go build -o ./mqttstat-release/mqttstat.darwin

echo build ./mqttstat-release/mqttstat.exe
env GOOS=windows go build -o ./mqttstat-release/mqttstat.exe
