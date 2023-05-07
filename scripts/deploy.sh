#!/bin/bash

make -C .. undeploy IMG="tjjh89017/kvnet:v0.0.1"
make -C .. docker-build IMG="tjjh89017/kvnet:v0.0.1" || exit 1
kind load docker-image "tjjh89017/kvnet:v0.0.1" -n kvnet-test 
make -C .. deploy IMG="tjjh89017/kvnet:v0.0.1"

echo "wait for kvnet controller manager"
kubectl wait deployment -n kvnet-system kvnet-controller-manager --for condition=Available=True --timeout=90s
