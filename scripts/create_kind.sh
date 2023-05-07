#!/bin/bash

kind delete cluster --name kvnet-test
kind create cluster --config kind.yaml --name kvnet-test
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v4.0.1/deployments/multus-daemonset.yml
kubectl apply -f install_cni.yaml

nodes=( kvnet-test-control-plane kvnet-test-worker kvnet-test-worker2 kvnet-test-worker3 )

for node in "${nodes[@]}"
do
	docker exec -t "$node" ip link add nic0 type veth peer name outnic0
	docker exec -t "$node" ip link set nic0 up
	docker exec -t "$node" ip link set outnic0 up
	docker exec -t "$node" ip link add nic1 type veth peer name outnic1
	docker exec -t "$node" ip link set nic1 up
	docker exec -t "$node" ip link set outnic1 up
	docker exec -t "$node" sysctl -w net.bridge.bridge-nf-call-iptables=0
	docker exec -t "$node" sysctl -w net.bridge.bridge-nf-call-ip6tables=0
done

echo "wait for cert-manager"
kubectl wait deployment -n cert-manager cert-manager --for condition=Available=True --timeout=90s
kubectl wait deployment -n cert-manager cert-manager-cainjector --for condition=Available=True --timeout=90s
kubectl wait deployment -n cert-manager cert-manager-webhook --for condition=Available=True --timeout=90s
