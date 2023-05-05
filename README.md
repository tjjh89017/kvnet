# kvnet
Kubernetes virtual network


## use KIND to test

- kind create cluster --config a.yaml (4 node)
- install cert-manager
    - kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml
- make docker-build deploy IMG="tjjh89017/kvnet:v0.0.1"
- kubectl apply -f config/samples/kvnet_v1alpha1_bridgeconfig.yaml
- kubectl apply -f config/samples/kvnet_v1alpha1_uplinkconfig.yaml
- kubectl apply -f config/samples/k8s.cni.cncf.io_v1_networkattachmentdefinition.yaml
- kubectl apply -f config/samples/deployment_alpine.yaml

change node labels and BridgeConfig/UplinkConfig/Deployment selectors to test

## TODO

- fill status for all resource
- create a new MyDeployment CRD to add affinity test on it
    - add webhook to insert affinity
    - test webhook will add network label expression to all affinity
    - add MyDeployment to deployment (use MyDeployment as 3rd party resource, becuase add k8s core reousrce webhook is nightmare for controller-runtime version before v0.15.0)

## trouble shoot

### Too many open files

https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
