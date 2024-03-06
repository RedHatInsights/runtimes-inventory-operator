# runtimes-inventory-operator
This project aims to provide a reusable component for Red Hat operators managing Java workloads.
This component allows these operators to more easily integrate their workloads into the Red Hat Insights
Runtimes Inventory.

## Description
Containers running in OpenShift that support either the [Insights Java Client](https://github.com/RedHatInsights/insights-java-client)
or its corresponding [Java Agent](https://github.com/RedHatInsights/insights-java-agent), will attempt to send reports to Red Hat Insights.
Doing so requires authentication to associate the report with a particular Red Hat customer. On OpenShift, these containers will likely not have
the means to obtain this authentication information when sending their reports.

This component allows operators for Red Hat products to create and manage an [APICast](https://github.com/3scale/APIcast) HTTP proxy,
configured with the necessary authentication information.
If the workload containers are configured to send their Insights reports to the proxy, they do not need to authenticate themselves.
The proxy is created using authentication information obtained from the OpenShift cluster that uniquely identifies it as belonging to a particular
customer.

## Getting Started

### Environment Variables
The following are required environment variables your operator must set:
- `RELATED_IMAGE_INSIGHTS_PROXY`: the container image to be used for the APICast proxy (e.g. `registry.redhat.io/3scale-amp2/apicast-gateway-rhel8:3scale2.14`)
- `INSIGHTS_BACKEND_DOMAIN`: the Red Hat Insights server host where reports will be forwarded (e.g. `console.redhat.com`)
- `INSIGHTS_ENABLED`: must be set to `true` in order for this component to run, this provides an opt-out mechanism for customers at the operator level

Optionally set the following environment variable:
- `INSIGHTS_PROXY_DOMAIN`: only needed when testing against a staging Insights backend that requires a proxy to access

### RBAC
Your operator will need to be run with the following permissions:
- Create, Get, List, Watch, Delete on Deployments, Services, Config Maps, Secrets in its own namespace
- Get, List, Watch on the OpenShift global pull secret: `pull-secret` in the `openshift-config` namespace
- Get, List, Watch on the cluster-scoped ClusterVersion resource, named `version`

### UHC Auth Proxy
In order for Red Hat Insights to accept traffic from the proxy, the proxy must specify a User-Agent header
with an approved prefix. Ensure that your operator's name is added to the list of
[approved prefixes](https://github.com/RedHatInsights/uhc-auth-proxy/blob/02be85bd43fb083c2dbed8f24356d9c040b0d6b1/server/server.go#L46-L53).

### Adding to your Manager
Inside your operator's main function, add this component by creating a new `InsightsIntegration` and
call its `Setup` method. You will need the following arguments:
- Your controller-runtime Manager
- The operator's name and namespace, which can be obtained from the Kubernetes downward API
- The UHC Auth Proxy approved User-Agent prefix, of the form `operator-name/x.y.z`, where x.y.z is your operator's version
- Your Manager's logger

```go
    insightsURL, err := insights.NewInsightsIntegration(mgr,
        operatorName, operatorNamespace, userAgentPrefix, &setupLog).Setup()
    if err != nil {
        setupLog.Error(err, "failed to set up Insights integration")
    }
    setupLog.Info("Insights proxy set up", "url", insightsURL.String())
```

This will add a new Insights Controller to your manager, which will be responsible for managing the proxy container.


TODO merge with above

TODO Required config
Environment variables:
- `RELATED_IMAGE_INSIGHTS_PROXY`
- `INSIGHTS_BACKEND_DOMAIN`
- `INSIGHTS_ENABLED`
- `INSIGHTS_PROXY_DOMAIN` optional

When running as a container image:
- `OPERATOR_NAME`
- `OPERATOR_NAMESPACE`
- `USER_AGENT_PREFIX`

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/runtimes-inventory-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/runtimes-inventory-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

