# INFINI Runtime Operator

INFINI Runtime Operator is a Kubernetes operator designed to automate the deployment, provisioning, management, and orchestration of projects built on the INFINI Framework such as INFINI Gateway and INFINI Console. 
It streamlines the lifecycle management of INFINI-based components in cloud-native environments, ensuring consistent, scalable, and resilient operations across Kubernetes clusters.


## Getting Started

### Prerequisites
- Go version v1.23.0+
- Docker version 17.03+.
- Kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy the operator on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=infinilabs/runtime-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=infinilabs/runtime-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

### Create instances of your solution

Below is a practical guide for deploying the InfiniLabs' products gateway and console.

#### gateweay

Thanks to the automation capabilities of the operator, deploying gateway only requires creating a custom resource in the Kubernetes cluster. The operator will automatically create related resources such as `StatefulSet`, `Service`, `ConfigMap`, and `PVC`. Users simply need to declaratively define desired configurations using the provided template. Below is the YAML file for the gateway custom resource:

```yaml
apiVersion: app.infini.cloud/v1
kind: ApplicationDefinition
metadata:
  name: my-infini-gateway # Application instance name
  namespace: default # target namespace
spec:
  components:
    - name: infini-gw # Component instance name (affects resource names)
      properties:
        # --- Core Configurations ---
        replicas: 1 # Replica count (e.g. 1 or 3)
        image:
          repository: docker.1ms.run/infinilabs/gateway # Target image
          tag: 1.29.3-2018 # Image version
        ports:
          - name: http-2900 # Application port
            containerPort: 2900
            protocol: TCP
          - name: http-8000 # Application port
            containerPort: 8000
            protocol: TCP
        storage: # Storage configuration
          enabled: true
          size: 2Gi # Storage size
          mountPath: /app # Directory path for gateway data/config in container
          volumeClaimTemplateName: data # VCT name (typically related to mountPath)
          accessModes:
            - ReadWriteOnce
          storageClassName: local # Valid StorageClass in the cluster
        service: # Service exposure configuration
          type: NodePort # Service type: NodePort or LoadBalancer
          ports:
            - name: http-2900
              containerPort: 2900
            - name: http-8000
              containerPort: 8000
        resources:
          requests:
            cpu: "0.5" # 500m
            memory: "512Mi"
          limits:
            cpu: "1" # 1 Core
            memory: "1Gi"
        probes: # Health checks
          liveness:
            httpGet:
              path: /_info 
              port: 2900
            initialDelaySeconds: 15
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 5
          readiness:
            httpGet:
              path: /_info 
              port: 2900
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 5
        env:
          - name: GATEWAY_HEAP_SIZE
            value: "512m"
          - name: LOG_LEVEL
            value: "info"
        configMounts: # ConfigMap mounts
          - name: infini-gw-config # ConfigMap name (generated as {{.name}}-config)
            mountPath: /gateway.yml 
            subPath: gateway.yml
            readOnly: true
        configFiles: # ConfigMap content
          "gateway.yml": |
            env:
              NODE_ID:
              NODE_ENDPOINT:
              TENANT_ID:
              GROUP_ID:
              CONFIG_SERVER:
              K8S_CLUSTER:
              K8S_NAMESPACE:
              K8S_CLUSTER_ID:
            
            path.data: /app/data
            path.logs: /app/log
            path.configs: /app/config 
            configs.auto_reload: true 
            
            node:
              id: $[[env.NODE_ID]]
              labels:
                endpoint: $[[env.NODE_ENDPOINT]]
                tenant_id: $[[env.TENANT_ID]]
                group_id: $[[env.GROUP_ID]]
                k8s_cluster_id: $[[env.K8S_CLUSTER_ID]]
            
            entry:
              - name: gateway_proxy
                enabled: true
                router: my_router
                network:
                  binding: 0.0.0.0:8000
                tls:
                  enabled: false
            
            api:
              enabled: true
              network:
                binding: 0.0.0.0:2900
              security:
                enabled: false
                username: admin
                password: $[[keystore.API_PASS]]
            
            router:
              - name: my_router
                default_flow: default_flow
            
            flow:
              - name: default_flow
                filter:
                  - context_parse:
                      context: _ctx.request.host
                      pattern: ^(?P<service>((easysearch|runtime|gateway)-[a-z0-9-]*|superset)).*?
                      group: "parsed_index"
                  - wildcard_domain:
                      when:
                        contains:
                          parsed_index.service: "runtime"
                      schema: "http"
                      max_idle_conn_duration: "900s"
                      suffix: $[[parsed_index.service]].$[[env.K8S_NAMESPACE]]
                      domain: "svc.$[[env.K8S_CLUSTER]]:2900"
                  - wildcard_domain:
                      when:
                        contains:
                          parsed_index.service: "gateway"
                      schema: "http"
                      max_idle_conn_duration: "900s"
                      suffix: $[[parsed_index.service]].$[[env.K8S_NAMESPACE]]
                      domain: "svc.$[[env.K8S_CLUSTER]]:2900"
                  - wildcard_domain:
                      when:
                        contains:
                          parsed_index.service: "easysearch"
                      schema: "https"
                      max_idle_conn_duration: "900s"
                      suffix: $[[parsed_index.service]].$[[env.K8S_NAMESPACE]]
                      domain: "svc.$[[env.K8S_CLUSTER]]:9200"
                  - wildcard_domain:
                      when:
                        contains:
                          parsed_index.service: "superset"
                      schema: "http"
                      max_idle_conn_duration: "900s"
                      suffix: $[[parsed_index.service]].$[[env.K8S_NAMESPACE]]
                      domain: "svc.$[[env.K8S_CLUSTER]]:8088"

```
In this YAML:  
- A gateway instance named infini-gw is created.
- Resources are deployed in the default namespace:
  - StatefulSet: infini-gw
  - Pod: infini-gw-0
  - Service: infini-gw
  - ConfigMap: infini-gw-config
  - PVC: data-infini-gw-0
  - ServiceAccount: default (default)

Key configurations for users:  
- namespace: Specify the deployment namespace.
- gateway.yml: Modify the configuration file content as needed.
- tag: Adjust the Docker image version.

#### console

Similarly like gateway, the YAML for the console custom resource is as follows:

```yaml
apiVersion: app.infini.cloud/v1
kind: ApplicationDefinition
metadata:
  name: my-infini-console # Application instance name
  namespace: default # Namespace for deployment
spec:
  components:
    - name: infini-console # Component instance name
      properties:
        # --- Core Configurations ---
        replicas: 1 # Replica count
        image:
          repository: docker.1ms.run/infinilabs/console # Target image
          tag: 1.29.4-2108 # Image version
        command: # Container startup command
          - sh
          - -c
          - |
            if [ ! -e /config_bak/certs ]; then
              cp -rf /config/* /config_bak
            fi
            exec /console
        ports:
          - name: http-9000 # Application port
            containerPort: 9000
            protocol: TCP
        storage: # Storage configuration
          enabled: true
          size: 2Gi # Storage size
          mountPath: /data # Console data/config mount path
          volumeClaimTemplateName: data # VCT name
          accessModes:
            - ReadWriteOnce
          storageClassName: local # StorageClass
        service: # Service exposure
          type: NodePort 
          ports:
            - name: http-9000
              containerPort: 9000
        resources:
          requests:
            cpu: "0.5" # 500m
            memory: "512Mi"
          limits:
            cpu: "1" # 1 Core
            memory: "1Gi"
        probes: # Health checks
          liveness:
            httpGet:
              path: /_info 
              port: 9000       
            initialDelaySeconds: 15
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 5
          readiness:
            httpGet:
              path: /_info 
              port: 9000       
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 5
        env:
          - name: INFINI_CONSOLE_ENDPOINT
            value: http://console:9000/
        configMounts: 
          - name: infini-console-config 
            mountPath: /config # Config directory
            readOnly: true
        configFiles: # Config content
          "security.yml": |
            security:
              enabled: true
              oauth:
                authorize_url: https://github.com/login/oauth/authorize
                client_id: your_github_acess_client_id
                client_secret: your_github_acess_client_secret
                default_roles:
                - ReadonlyUI
                - DATA
                enabled: true
                redirect_url: http://your_console_endpoint/sso/callback/
                role_mapping:
                  luohoufu:
                  - Administrator
                  medcl:
                  - Administrator
                scopes: []
                token_url: https://github.com/login/oauth/access_token
```
Key configurations for users:

- namespace: Specify the deployment namespace.
- security.yml: Modify the configuration file content as needed.
- tag: Adjust the Docker image version.

After preparing the YAML configuration files according to the instructions above, you can proceed with deployment by running the following command:
```sh
kubectl apply -k config/your-custeom-resouce-yamls
```

Then check resource creation progress via:
```shell
kubectl get applicationdefinition,statefulset,pod,service,configmap,pvc -n <namespace>
```

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=infinilabs/runtime-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/runtime-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Community

Fell free to join the Discord server to discuss anything around this project:

[https://discord.gg/4tKTMkkvVX](https://discord.gg/4tKTMkkvVX)

## License

INFINI Runtime Operator is a truly open-source project, licensed under the [GNU Affero General Public License v3.0](https://opensource.org/licenses/AGPL-3.0).
We also offer a commercially supported, enterprise-ready version of the software.
For more details, please refer to our [license information](./LICENSE).