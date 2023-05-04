#!.bin/bash

kind create cluster --config kind.yaml
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
make undeploy IMG="tjjh89017/kvnet:v0.0.1" || true
make docker-build IMG="tjjh89017/kvnet:v0.0.1"
kind load docker-image "tjjh89017/kvnet:v0.0.1"
make deploy IMG="tjjh89017/kvnet:v0.0.1"
kubectl apply -f ../config/samples/kvnet_v1alpha1_bridgeconfig.yaml
