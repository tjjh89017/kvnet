#!/bin/bash

kubectl apply -f ../config/samples/kvnet_v1alpha1_bridgeconfig.yaml
kubectl apply -f ../config/samples/kvnet_v1alpha1_uplinkconfig.yaml 
kubectl apply -f ../config/samples/k8s.cni.cncf.io_v1_networkattachmentdefinition.yaml 
kubectl apply -f ../config/samples/kvnet_v1alpha1_subnet.yaml
