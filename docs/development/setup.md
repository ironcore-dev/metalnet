# Development Setup
This section describes necessary steps to setup a development environment for testing and contributing to this project.


## Requirements
* `go` >= 1.20
* `git`, `make` and `kubectl`
* Access to a Kubernetes cluster ([Minikube](https://minikube.sigs.k8s.io/docs/), [kind](https://kind.sigs.k8s.io/) or a
  real cluster). We also provide detailed [guidance](./kind_install.md) on setting up a kind cluster for local development.
* [metalbond](https://github.com/ironcore-dev/metalbond) We need to setup a metalbond server that is responsible of managing overlay routes in a network. Metalnet establishes a connection with it during the initialization.
* [dp-service](https://github.com/ironcore-dev/net-dpservice) We need to start a dp-service process that reacts to the gRPC calls generated from metalbond.


## Metalbond and dp-service
Assuming the executable binaries of metalbond and dp-service has been successfully compiled by following their corresponding instructions, we first need to start these two components. For more detailed information on these two components, please refer to their documentation.

To start metalbond server, we use the following command. Switch to metalbond's repo and run
```sh
./metalbond server --listen [::]:4711 --http [::1]:4712 --keepalive 3
```

Additionally, the default router address can be announced by running the following command. In this case, no extra parameter needs to be provided when starting the controller manager.

```
 ./metalbond client      --server "[::]:4711"      --keepalive 3      --subscribe 100      --announce 100#0.0.0.0/0#[default-router-address, e.g., abcd:efgh:1234::5]
```

To start dp-service, we use the following command. Switch to dp-service's repo and run
```sh
sudo ./build/src/dpservice-bin -l 0,1  -a [pci-address-of-1st-pf],representor=[vf-in-use, e.g., 0-4]  -a [pci-address-of-2nd-pf] --proc-type=primary --log-level user1:8  -- --pf0=[interface-name-of-1st-pf] --pf1=[interface-name-of-2nd-pf] --vf-pattern=[interface-name-patternof-vfs, e.g., ens1f0npf0vf] --ipv6=[ipv6-addr-of-host] --no-stats --no-offload

```

## Running on the cluster
Before running metalnet operators, it is necessary to ensure that proper access to a kubernetes cluster is obtained. Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Install Instances of Custom Resources:
Your Kubernetes API server needs to know about the APIs which come with the metalnet project. To install the APIs your cluster, run
```sh
make install
```
### Start the Controller Manager
The controller manager can be started via the following command
```sh
make run-base
```

If you want to manually specify the default router address, start the controller manager by running:
```
go run ./main.go --router-address=[default-router-address]
```

### Apply Sample Manifests
The `config/samples` folder contains samples for all APIs supported by this project. You can apply any of the samples by
running

```sh
kubectl apply -f config/samples/SOME_RESOURCE.yaml
```
**Note**: It is possible that you need to adapt these manifest samples to your local environment. For example, the `nodeName` field needs to be changed to the actual worker node's name in your kubernetes cluster.

### Rebuilding API Type and Manifests

Everytime a change has been done to any of the types definitions, the corresponding manifests and generated code pieces
have to be rebuilt. Make sure your APIs are up-to-date by running `make install` after your code / manifests
have been regenerated.

```sh
make generate
make manifests
```
**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

