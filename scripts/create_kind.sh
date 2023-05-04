#!.bin/bash

kind delete cluster --name kvnet-test
kind create cluster --config kind.yaml --name kvnet-test
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
