#!/usr/bin/env bash

dir=$(cd $(dirname $BASH_SOURCE) && pwd)
set -ex

protoc -I pb/ --go_out=plugins=grpc:"$dir"  "service.proto"
rm "$dir"/protocol -rf
mv "$dir"/server/protocol ./
rmdir "$dir"/server