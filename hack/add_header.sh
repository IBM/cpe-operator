#!/bin/bash

for i in ../main.go ../*/*.go ../*/*/*.go # or whatever other pattern...
do
  if ! grep -q Copyright $i
  then
    cat boilerplate.go.txt $i >$i.new && mv $i.new $i
  fi
done