# epsagon-operator
Epsagon Kubernetes Operator - For integrating K8s clusters with Epsagon

## Installation
### From code:
Clone this repository and 
```
make install
make deploy
```
### From github release:
```
https://raw.githubusercontent.com/epsagon/epsagon-operator/v0.1/build/crd.yaml | kubectl apply -f -
https://raw.githubusercontent.com/epsagon/epsagon-operator/v0.1/build/epsagon-operator.yaml | kubectl apply -f -
```

## Usage
Create an Epsagon resource in `epsagon-monitoring` namespace:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: epsagon-monitoring
---
apiVersion: integration.epsagon.com/v1alpha1
kind: Epsagon
metadata:
  name: epsagon-integration
  namespace: epsagon-monitoring
spec:
  epsagonToken: <YOUR EPSAGON TOKEN>
  clusterEndpoint: <EXTERNAL ENDPOINT TO ACCESSS THIS CLUSTER FROM THE INTERNET>
```

### Remove integration
`kubectl delete epsagon -n epsagon-monitoring epsagon-integration`
