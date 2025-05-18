# infini-operator
// TODO(user): Add simple overview of use/purpose

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started

### Prerequisites
- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/infini-operator:tag
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
make deploy IMG=<some-registry>/infini-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

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
make build-installer IMG=<some-registry>/infini-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/infini-operator/<tag or branch>/dist/install.yaml
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

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025 infinilabs.com.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Usage

这个`operator`设计用于在k8s集群上创建和管理`infinilabs`的资源，包括`inifinilabs`已有的产品 `gateway` 和 `console`，我们现在来实际操作部署这两个产品。

- ### gateway

得益于`operator`自动化的特性，我们只需要在k8s集群上创建一个`gateway`的`custom resource`即可，`operator`会自动创建`gateway`相关的`statefulset`、`service`、`configmap`、`pvc`等，我们只需要按照提供的模板声明式地编写我们需要的资源值即可，下面是`gateway`的`custom resource`的yaml文件：

```yaml
apiVersion: app.infini.cloud/v1
kind: ApplicationDefinition
metadata:
  name: my-infini-gateway # 应用实例名称
  namespace: default # 应用部署所在命名空间
spec:
  components:
    - name: infini-gw # 组件实例名称 (会影响资源名称)
      properties:
        # --- 核心配置 ---
        replicas: 1 # 副本数 (例如: 1 或 3)
        image:
          repository: docker.1ms.run/infinilabs/gateway # 目标镜像
          tag: 1.29.3-2018 # 目标镜像版本
        ports:
          - name: http-2900 # 应用端口
            containerPort: 2900
            protocol: TCP
          - name: http-8000 # 应用端口
            containerPort: 8000
            protocol: TCP
          # - name: metrics # 如果有其他端口
          #   containerPort: 9090
        storage: # 存储配置
          enabled: true
          size: 2Gi # 存储容量大小
          mountPath: /app # gateway 数据/配置挂载所在容器中的目录路径
          volumeClaimTemplateName: data # VCT 名称 (通常与 mountPath 相关)
          accessModes: # 访问模式
            - ReadWriteOnce
          storageClassName: local # 集群中有效的 StorageClass
#        serviceAccount:  # ServiceAccount 名称
#            create: false # 是否创建 ServiceAccount, 默认值为 false
#            name: controller-manager # ServiceAccount 名称
        # --- 可选配置 ---
        service: # service服务暴露配置
          type: NodePort # service类型，NodePort 或 LoadBalancer
          ports:
            - name: http-2900 # Service 端口名称
              containerPort: 2900 # Service 监听的端口
               # targetPort: 8080 # 转发的目标 Pod 端口 (默认等于 containerPort)
               # nodePort: 30080 # 可选：指定 NodePort 端口，或者自动分配
            - name: http-8000 # Service 端口名称
              containerPort: 8000 # Service 监听的端口
        resources:
          requests: # 资源请求配置
            cpu: "0.5" # 500m
            memory: "512Mi"
          limits: # 资源限制配置
            cpu: "1" # 1 Core
            memory: "1Gi"
        probes: # 健康检查配置
          liveness: # 存活探针配置
            httpGet:
              path: /_info # 存活检查路径
              port: 2900   # 存活检查端口
            initialDelaySeconds: 15 # 存活检查延迟时间
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 5
          readiness: # 就是探针配置
            httpGet:
              path: /_info 
              port: 2900       
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 5
        env:
          - name: GATEWAY_HEAP_SIZE # 环境变量，可按需添加
            value: "512m"
          - name: LOG_LEVEL
            value: "info"
        configMounts: # configmap配置
          - name: infini-gw-config # configmap名称，生成规则是 {{.name}}-config
            mountPath: /gateway.yml # 挂载配置文件的路径
            subPath: gateway.yml # 挂载具体的某个文件
            readOnly: true
        configFiles: # configmap内容
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
            path.configs: /app/config # directory of additional gateway configurations
            configs.auto_reload: true # set true to auto reload gateway configurations
            
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
                      schema: "http" #https or http
                      max_idle_conn_duration: "900s"
                      suffix: $[[parsed_index.service]].$[[env.K8S_NAMESPACE]]
                      domain: "svc.$[[env.K8S_CLUSTER]]:2900"
                  - wildcard_domain:
                      when:
                        contains:
                          parsed_index.service: "gateway"
                      schema: "http" #https or http
                      max_idle_conn_duration: "900s"
                      suffix: $[[parsed_index.service]].$[[env.K8S_NAMESPACE]]
                      domain: "svc.$[[env.K8S_CLUSTER]]:2900"
                  - wildcard_domain:
                      when:
                        contains:
                          parsed_index.service: "easysearch"
                      schema: "https" #https or http
                      max_idle_conn_duration: "900s"
                      suffix: $[[parsed_index.service]].$[[env.K8S_NAMESPACE]]
                      domain: "svc.$[[env.K8S_CLUSTER]]:9200"
                  - wildcard_domain:
                      when:
                        contains:
                          parsed_index.service: "superset"
                      schema: "http" #https or http
                      max_idle_conn_duration: "900s"
                      suffix: $[[parsed_index.service]].$[[env.K8S_NAMESPACE]]
                      domain: "svc.$[[env.K8S_CLUSTER]]:8088"

```
如上yaml所示，各个配置项目有详细的注释说明和示例，我们创建了一个名为`infini-gw`的gateway实例，gateway的pod被创建在`default`命名空间下，statefulset名称为`infini-gw`, pod的名称为`infini-gw-0`，pod的service名称为`infini-gw`，pod的configmap名称为`infini-gw-config`，pod的pvc名称为`data-infini-gw-0`，pod的serviceaccount名称为默认的`default`。  
整个配置已经可以直接使用，根据不同用户的需求，用户只需要关注两处配置就可以了，
  1. gateway.yml: 这里是gateway的配置文件，用户可以根据自己的需求修改配置文件的内容。
  2. tag: 这是gateway的docker镜像版本号，用户可以根据自己的需求修改版本号。


- ### console

同样的，`console`的`custom resource`的yaml文件如下：

```yaml
apiVersion: app.infini.cloud/v1
kind: ApplicationDefinition
metadata:
  name: my-infini-console # 应用实例名称
  namespace: default # 示例所在命名空间
spec:
  components:
    - name: infini-console # 组件实例名称 (会影响资源名称)
      properties:
        # --- 核心配置 ---
        replicas: 1 # 副本数 (例如: 1 或 3)
        image:
          repository: docker.1ms.run/infinilabs/console # 目标镜像
          tag: 1.29.4-2108 # 镜像版本
        ports:
          - name: http-9000 # 应用端口
            containerPort: 9000
            protocol: TCP
          # - name: metrics # 如果有其他端口
          #   containerPort: 9090
        storage: # 存储配置
          enabled: true
          size: 2Gi # 存储大小
          mountPath: /data # console 数据/配置挂载路径
          volumeClaimTemplateName: data # VCT 名称 (通常与 mountPath 相关)
          accessModes:
            - ReadWriteOnce
          storageClassName: local # 集群中有效的 StorageClass
#        serviceAccount:  # ServiceAccount 名称
#            create: false # 是否创建 ServiceAccount, 默认值为 false
#            name: controller-manager # ServiceAccount 名称
        # --- 可选配置 ---
        service: # 服务暴露配置
          type: NodePort # 使用 NodePort 便于测试，或 LoadBalancer
          ports:
            - name: http-9000 # Service 端口名称
              containerPort: 9000 # Service 监听的端口
        resources:
          requests:
            cpu: "0.5" # 500m
            memory: "512Mi"
          limits:
            cpu: "1" # 1 Core
            memory: "1Gi"
        probes: # 健康检查配置
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
            mountPath: /config # 挂载配置文件的目录
            readOnly: true
        configFiles: # 配置文件内容
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

如上yaml所示，`console`的配置文件和`gateway`的配置文件类似，用户只需要关注两处配置就可以了，
  1. security.yml: 这里是console的配置文件，用户可以根据自己的需求修改配置文件的内容。
  2. tag: 这是console的docker镜像版本号，用户可以根据自己的需求修改版本号。