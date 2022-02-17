目录
=================
  * [一、前言](#一前言)
  * [二、功能](#二功能)
  * [三、兼容性](#三兼容性)
  * [四、前置要求](#四前置要求)
  * [五、部署](#五部署)
  * [六、CLB功能](#六CLB功能)
  * [七、贡献指南](#七贡献指南)

# 一、前言
tencent cloud controller manager是基于腾讯云tencentcloud cloud controller manager的一个改进项目。 重新开发的主要原因是tencentcloud cloud controller manager很久不更新，为了更好的针对bug的修复和对新k8s的支持，所以把 tencentcloud cloud controller manager重写并命名为tencent-cloud-controller-manager

# 二、功能
当前 tencentcloud-cloud-controller-manager 实现了:

    nodecontroller - 更新 kubernetes node 相关的 addresses 信息。
    nodeLifecycleController - 根据node的生命周期kubernetes node的状态，如当node关机时，更新kubernetes node的状态为not ready
    routecontroller - 负责创建 vpc 内 pod 网段内的路由。
    servicecontroller - 当集群中创建了类型为 LoadBalancer 的 service 的时候，创建相应的LoadBalancers。

# 三、兼容性
tencent CCM版本 | kubernetes版本
---|---
v1.x | 推荐v1.18


# 四、前置要求

在腾讯云从云服务器手工搭建 kubernetes 且集群网络使用路由方案时，需要先创建kubernetes用来建立pod网络的子网，使得集群中的pod通信能够和腾讯云vpc打通，参考：
https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/blob/master/route-ctl/README_zhCN.md

在当前 kubernetes 中运行 cloud controller manager 需要一些设置的改动。下面是一些相关的建议。
设置 --cloud-provider=external

集群内所有的 kubelet 需要 要设置启动参数 --cloud-provider=external。 kube-apiserver 和 kube-controller-manager 不应该 设置 --cloud-provider 参数。

注意: 设置 --cloud-provider=external 会给集群内所有的节点加上 node.cloudprovider.kubernetes.io/uninitialized taint, 从而使得 pod 不会调度到有此标记的节点。cloud controller manager 需要在这些节点初始化完成之后，去掉这个标记，这意味着在 cloud controller manager 完成节点初始化相关的工作之前，pod 不会被调度到这个节点上。

在后续的发展中, --cloud-provider=external 将会成为默认参数. 
Kubernetes 节点的名字需要和节点的内网 ip 相同

默认情况下，kubelet 会使用节点的 hostname 作为节点的名称。可以使用 --hostname-override 参数使用节点的内网 ip 覆盖掉节点本身的 hostname，从而使得节点的名称和节点的内网 ip 保持一致。这一点非常重要，否则 cloud controller manager 会无法找到对应 kubernetes 节点的云服务器。

# 五、部署

（1）创建secret
修改examples/secret.yaml:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tencent-cloud-controller-manager-config
  namespace: kube-system
data:
  # 需要注意的是,secret 的 value 需要进行 base64 编码
  #   echo -n "<REGION>" | base64
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION: "<REGION>"    #腾讯云区域
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID: "<SECRET_ID>"  #腾讯云帐号secret id
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY: "<SECRET_KEY>" #腾讯云帐号secret key
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ROUTE_TABLE: "<CLUSTER_NETWORK_ROUTE_TABLE_NAME>" #腾讯云创建的路由表名
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID: "<VPC_ID>" #腾讯云创建的路由表的VPC ID
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_NAME_PREFIX: "<TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_NAME_PREFIX>"  #在腾讯云创建CLB时的前缀
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_TAG_KEY: "<TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_TAG_KEY>" #在腾讯云创建CLB等资源时打tag的key，tag value为TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_NAME_PREFIX
```
将上面的value修改为你需要的配置，记得需要是base64编码.


创建：
```bash
kubectl apply -f ./examples/secret.yaml
```

（2）创建RBAC
```bash
kubectl apply -f ./examples/rbac.yaml
```

（3）创建Deployment

修改examples/deployment.yaml的启动参数：
```
            - --cloud-provider=tencentcloud # 指定 cloud provider 为 tencentcloud
            - --cluster-cidr=10.248.0.0/17 # 集群 pod 所在网络，需要提前创建
            - --cluster-name=kubernetes # 集群名称
```
主要是cluster-cidr，它应该与TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ROUTE_TABLE相对应。

```bash
kubectl apply -f ./examples/deployment.yaml
```

# 六、CLB功能
当用户创建类型是**LoadBalancer**的Service，默认情况下，tencent cloud controller manager会联动的创建CLB。而当用户删除此Service时，tencent cloud controller manager也会联动的删除CLB。  

下面是一个LoadBalancer类型的service例子
```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/tencentcloud-loadbalancer-type: private
    service.beta.kubernetes.io/tencentcloud-loadbalancer-type-internal-subnet-id: subnet-bh6bxta3
  labels:
    app: nginx
    service: nginx
  name: nginx
  namespace: default
spec:
  ports:
  - name: http
    nodePort: 27726
    port: 80
    protocol: TCP
    targetPort: 80
  selector:
    app: nginx
  sessionAffinity: None
  type: LoadBalancer
```

其中annotations和type是关键，type需要为LoadBalancer,tencent cloud controller manager支持的annotations有：

annotations | 必选 | 说明
---|---|---
service.beta.kubernetes.io/tencentcloud-loadbalancer-type | 否 | 可以为public和private(默认)，public为公网型CLB，private为私有子网型，当为private时，tencentcloud-loadbalancer-type-internal-subnet-id也必需要配置。
service.beta.kubernetes.io/tencentcloud-loadbalancer-type-internal-subnet-id | 否 | 私有网络型CLB的私有子网ID，私有子网型CLB必需配置
service.beta.kubernetes.io/tencentcloud-loadbalancer-node-label-key | 否 |  node的标签的key，默认值为kubernetes.io/role
service.beta.kubernetes.io/tencentcloud-loadbalancer-node-label-value | 否 |  node的标签key的值，默认值为node，也就是说默认只有标签kubernetes.io/role=node的节点才会加入到CLB的后端节点内。
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-switch | 否 |  是否开启健康检查：1（开启）、0（关闭），默认为0
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-timeout | 否 | 健康检查的响应超时时间（仅适用于四层监听器），可选值：2~60，默认值：2，单位：秒。响应超时时间要小于检查间隔时间。
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-interval-time | 否 | 健康检查探测间隔时间，默认值：5，可选值：5~300，单位：秒。
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-health-num | 否 | 健康阈值，默认值：3，表示当连续探测三次健康则表示该转发正常，可选值：2~10，单位：次。
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-un-health-num | 否 | 不健康阈值，默认值：3，表示当连续探测三次不健康则表示该转发异常，可选值：2~10，单位：次。


以下的annotations暂时未想好怎么实现，腾讯云有提供相应的功能，但是通过k8s的service来创建7层的CLB怎么关联还需要做一些适配。

annotations | 必选 | 说明
---|---|---
service.beta.kubernetes.io/tencentcloud-loadbalancer-listener-port | 否 |  要将监听器创建到哪个端口，仅允许一个端口。
service.beta.kubernetes.io/tencentcloud-loadbalancer-listener-protocol | 否 | 监听器协议： TCP , UDP , HTTP , HTTPS , TCP_SSL（TCP_SSL 正在内测中，如需使用请通过工单申请，目前仅支持tcp,udp,http这三种）。
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-port | 否 | 自定义探测相关参数。健康检查端口，默认为后端服务的端口，除非您希望指定特定端口，否则建议留空。（仅适用于TCP/UDP监听器）。
service.beta.kubernetes.io/tencentcloud-loadbalancer-listener-http-rules | 否 | 当service.beta.kubernetes.io/tencentcloud-loadbalancer-listener-protocol为http时必填，格式为host1;host2;hostn,当前不支持自义path，path统一使用/
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-http-code | 否 | 健康检查状态码（仅适用于HTTP/HTTPS转发规则、TCP监听器的HTTP健康检查方式）。可选值：1~31，默认 31。1 表示探测后返回值 1xx 代表健康，2 表示返回 2xx 代表健康，4 表示返回 3xx 代表健康，8 表示返回 4xx 代表健康，16 表示返回 5xx 代表健康。若希望多种返回码都可代表健康，则将相应的值相加。注意：TCP监听器的HTTP健康检查方式，只支持指定一种健康检查状态码。
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-http-path | 否 | 当service.beta.kubernetes.io/tencentcloud-loadbalancer-listener-protocol为http时必填，健康检查路径（仅适用于HTTP/HTTPS转发规则、TCP监听器的HTTP健康检查方式）。
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-http-domain | 否 | 健康检查域名（仅适用于HTTP/HTTPS转发规则、TCP监听器的HTTP健康检查方式）。
service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-http-method | 否 | 健康检查方法（仅适用于HTTP/HTTPS转发规则、TCP监听器的HTTP健康检查方式），默认值：HEAD，可选值HEAD或GET。


# 七、贡献指南

请参阅：

[贡献指南](https://github.com/weimob-tech/cloud-provider-tencent/blob/master/CONTRIBUTING.md)