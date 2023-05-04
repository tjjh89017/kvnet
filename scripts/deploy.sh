#!/bin/bash

make -C .. undeploy IMG="tjjh89017/kvnet:v0.0.1"
make -C .. docker-build IMG="tjjh89017/kvnet:v0.0.1"
kind load docker-image "tjjh89017/kvnet:v0.0.1" -n kvnet-test 
make -C .. undeploy IMG="tjjh89017/kvnet:v0.0.1"
