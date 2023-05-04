# kvnet
Kubernetes virtual network


# use KIND to test

- kind create cluster --config a.yaml (4 node)
- install cert-manager
    - kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
- make docker-build deploy IMG="tjjh89017/kvnet:v0.0.1"
- kubectl apply -f config/samples/kvnet_v1alpha1_bridgeconfig.yaml

# trouble shoot

## Too many open files

https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
