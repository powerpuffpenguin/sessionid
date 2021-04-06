#!/usr/bin/env bash

dir=$(cd $(dirname $BASH_SOURCE) && pwd)
set -ex

protoc -I pb/ --go_out=plugins=grpc:"$dir"  "agent.proto"
rm "$dir"/protocol -rf
mv "$dir"/github.com/powerpuffpenguin/sessionid/protocol ./
rm "$dir"/github.com -rf