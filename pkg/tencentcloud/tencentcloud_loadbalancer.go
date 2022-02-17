package tencentcloud

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	clb "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/clb/v20180317"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	cloudErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

const (
	// public network based clb or private network based clb
	ServiceAnnotationLoadBalancerType = "service.beta.kubernetes.io/tencentcloud-loadbalancer-type"
	LoadBalancerTypePublic            = "public"
	LoadBalancerTypePrivate           = "private"

	// subnet id for private network based clb
	ServiceAnnotationLoadBalancerTypeInternalSubnetId    = "service.beta.kubernetes.io/tencentcloud-loadbalancer-type-internal-subnet-id"
	ServiceAnnotationLoadBalancerNodeLabelKey            = "service.beta.kubernetes.io/tencentcloud-loadbalancer-node-label-key"
	ServiceAnnotationLoadBalancerNodeLabelValue          = "service.beta.kubernetes.io/tencentcloud-loadbalancer-node-label-value"
	ServiceAnnotationLoadBalancerHealthCheckSwitch       = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-switch"
	ServiceAnnotationLoadBalancerHealthCheckTimeout      = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-timeout"
	ServiceAnnotationLoadBalancerHealthCheckIntervalTime = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-interval-time"
	ServiceAnnotationLoadBalancerHealthCheckHealthNum    = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-health-num"
	ServiceAnnotationLoadBalancerHealthCheckUnHealthNum  = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-un-health-num"

	//ServiceAnnotationLoadBalancerListenerPort            = "service.beta.kubernetes.io/tencentcloud-loadbalancer-listener-port"
	//ServiceAnnotationLoadBalancerHealthCheckPort         = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-port"
	//ServiceAnnotationLoadBalancerListenerProtocol        = "service.beta.kubernetes.io/tencentcloud-loadbalancer-listener-protocol"
	//ServiceAnnotationLoadBalancerListenerHttpRules       = "service.beta.kubernetes.io/tencentcloud-loadbalancer-listener-http-rules"
	//ServiceAnnotationLoadBalancerHealthCheckHttpCode     = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-http-code"
	//ServiceAnnotationLoadBalancerHealthCheckHttpPath     = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-http-path"
	//ServiceAnnotationLoadBalancerHealthCheckHttpDomain   = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-http-domain"
	//ServiceAnnotationLoadBalancerHealthCheckHttpMethod   = "service.beta.kubernetes.io/tencentcloud-loadbalancer-health-check-http-method"

	nodeLabelKeyOfLoadBalancerDefault   = "kubernetes.io/role"
	nodeLabelValueOfLoadBalancerDefault = "node"
)

var (
	ClbLoadBalancerTypePublic  = "OPEN"
	ClbLoadBalancerTypePrivate = "INTERNAL"
	ClbTagServiceKey           = "k8s-service-id"
	//cacheNamePreCLBListener: cache key name pre for clb listener id
	cacheNamePreCLBListener = "clb_listener_id_"
	//cacheNamePreCLB: cache key name pre for clb id
	cacheNamePreCLB = "clb_id_"
	//loadBalancerPassToTarget: Target是否放通来自CLB的流量。开启放通（true）：只验证CLB上的安全组；不开启放通（false）：需同时验证CLB和后端实例上的安全组。
	loadBalancerPassToTarget = true
)

// getLoadBalancerName return LoadBalancer Name for service
func (cloud *Cloud) getLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	klog.V(3).Infof("tencentcloud.getLoadBalancerName(\"%s, %T\"): entered\n", clusterName, *service)
	klog.V(3).Infof("tencentcloud.getLoadBalancerName: CLBNamePrefix=%s service.Namespace=%s service.Name=%s\n", cloud.txConfig.CLBNamePrefix, service.Namespace, service.Name)

	name := cloud.txConfig.CLBNamePrefix + "_" + service.Namespace + "_" + service.Name
	//腾讯云CLB名称最长为60，如果计算出来的默认名大于60
	//则取默认名前50位加_uid前8位
	if len(name) > 60 {
		uid := string(service.UID)
		name = name[:50] + "_" + uid[:8]
	}

	klog.V(3).Infof("tencentcloud.getLoadBalancerName: return: %s\n", name)
	return name
}

// getLoadBalancerListeners return Tencent Cloud LoadBalancer Listeners Name for LoadBalancer
func (cloud *Cloud) getLoadBalancerListeners(LoadBalancerId string) ([]*clb.Listener, error) {
	klog.V(3).Infof("tencentcloud.getLoadBalancerListeners(\"%s\"): entered\n", LoadBalancerId)

	cacheKey := cacheNamePreCLBListener + LoadBalancerId
	cacheValue, exist := cloud.cache.Get(cacheKey)
	if exist {
		klog.V(3).Infof("tencentcloud.getLoadBalancerListeners: cache return(CLB_ID:%s):  %T, nil\n", LoadBalancerId, cacheValue.([]*clb.Listener))
		return cacheValue.([]*clb.Listener), nil
	}

	request := clb.NewDescribeListenersRequest()
	request.LoadBalancerId = common.StringPtr(LoadBalancerId)
	response, err := cloud.clb.DescribeListeners(request)
	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.getLoadBalancerListeners: Get TencentCloud error: %s\n", err)
		klog.V(3).Infof("tencentcloud.getLoadBalancerListeners: return: nil,%v\n", err)
		return nil, err
	}
	if err != nil {
		klog.Warningf("tencentcloud.getLoadBalancerListeners: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.getLoadBalancerListeners: return: nil, %v\n", err)
		return nil, err
	}

	cloud.cache.Set(cacheKey, response.Response.Listeners)
	klog.V(3).Infof("tencentcloud.getLoadBalancerListeners: return(lbID:%s): %+v, nil\n", LoadBalancerId, response.Response.Listeners)
	return response.Response.Listeners, nil
}

// getLoadBalancer return Tencent Cloud LoadBalancer for LoadBalancer name
func (cloud *Cloud) getLoadBalancer(name string, service *v1.Service) (*clb.LoadBalancer, error) {
	klog.V(3).Infof("tencentcloud.getLoadBalancerByName(\"%s\"): entered\n", name)

	cacheKey := cacheNamePreCLB + name
	cacheValue, exist := cloud.cache.Get(cacheKey)
	if exist {
		klog.V(3).Infof("tencentcloud.getLoadBalancerByName: cache return(name:%s): %T, nil\n", name, cacheValue.(*clb.LoadBalancer))
		return cacheValue.(*clb.LoadBalancer), nil
	}

	// we don't need to check loadbalancer kind here because ensureLoadBalancerInstance will ensure the kind is right
	request := clb.NewDescribeLoadBalancersRequest()
	request.LoadBalancerName = common.StringPtr(name)
	request.Filters = cloud.getLoadBalancerFilter(service)

	response, err := cloud.clb.DescribeLoadBalancers(request)
	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.getLoadBalancerByName: Get TencentCloud error: %s\n", err)
		klog.V(3).Infof("tencentcloud.getLoadBalancerByName: return: nil, %v\n", err)
		return nil, err
	}
	if err != nil {
		klog.Warningf("tencentcloud.getLoadBalancerByName: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.getLoadBalancerByName: return: nil, %v\n", err)
		return nil, err
	}
	count := len(response.Response.LoadBalancerSet)
	switch {
	case count == 1:
		cloud.cache.Set(cacheKey, response.Response.LoadBalancerSet[0])
		klog.V(3).Infof("tencentcloud.getLoadBalancerByName: return(name: %s, CLB ID: %s): %T nil\n", *response.Response.LoadBalancerSet[0].LoadBalancerName, *response.Response.LoadBalancerSet[0].LoadBalancerId, response.Response.LoadBalancerSet[0])
		return response.Response.LoadBalancerSet[0], nil
	case count < 1:
		klog.Warningf("tencentcloud.getLoadBalancerByName: return(name: %s): nil, %v\n", name, ErrCloudLoadBalancerNotFound)
		return nil, ErrCloudLoadBalancerNotFound
	default:
		klog.Warningf("tencentcloud.getLoadBalancerByName: find CLB count > 1,count: %d. return(name: %s): nil, %v\n", count, name, ErrCloudLoadBalancerNotFound)
		return nil, ErrCloudLoadBalancerNotFound
	}
}

// getLoadBalancer return Tencent Cloud LoadBalancer for LoadBalancer name
func (cloud *Cloud) getLoadBalancerFilter(service *v1.Service) []*clb.Filter {
	klog.V(3).Infof("tencentcloud.getLoadBalancerFilter(\"service: %s\"): entered\n", service.Name)
	tagName := "tag:" + ClbTagServiceKey
	tagValue := string(service.UID)
	filter := clb.Filter{
		Name:   &tagName,
		Values: []*string{&tagValue},
	}

	klog.V(3).Infof("tencentcloud.getLoadBalancerFilter: return: tagName: %s, tagValue: %s\n", tagName, tagValue)
	return []*clb.Filter{&filter}
}

// ensureLoadBalancerInstance ensure the Tencent Cloud Load Balancer Instance is the same as the service
func (cloud *Cloud) ensureLoadBalancerInstance(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance(\"%s %T\"): entered\n", clusterName, service)
	loadBalancerName := cloud.getLoadBalancerName(ctx, clusterName, service)
	loadBalancer, err := cloud.getLoadBalancer(loadBalancerName, service)

	if err != nil {
		if err != ErrCloudLoadBalancerNotFound {
			klog.Warningf("tencentcloud.ensureLoadBalancerInstance: Get error: %s\n", err)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance: return: %v\n", err)
			return err
		}
		err = cloud.createLoadBalancer(ctx, clusterName, service)
		if err != nil {
			klog.Warningf("tencentcloud.ensureLoadBalancerInstance: Get error: %s\n", err)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance: return: %v\n", err)
			return err
		}
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance: return: nil\n")
		return nil
	}

	loadBalancerDesiredType, ok := service.Annotations[ServiceAnnotationLoadBalancerType]
	if !ok || (loadBalancerDesiredType != LoadBalancerTypePrivate && loadBalancerDesiredType != LoadBalancerTypePublic) {
		loadBalancerDesiredType = LoadBalancerTypePrivate
	}

	needRecreate := false
	switch loadBalancerDesiredType {
	case LoadBalancerTypePublic:
		if !(*loadBalancer.LoadBalancerType == ClbLoadBalancerTypePublic && *loadBalancer.VpcId == cloud.txConfig.VpcId) {
			klog.Infof("tencentcloud.ensureLoadBalancerInstance: CLB need delete,pls check, ID: %s,Type: %s, VpcId: %s \n", *loadBalancer.LoadBalancerId, *loadBalancer.LoadBalancerType, *loadBalancer.VpcId)
			needRecreate = true
		}
		break
	case LoadBalancerTypePrivate:
		loadBalancerTypeInternalSubnetId, ok := service.Annotations[ServiceAnnotationLoadBalancerTypeInternalSubnetId]
		if !ok {
			klog.Warningf("tencentcloud.ensureLoadBalancerInstance: Get error: service (nameSpace:%s,name:%s) subnet must be specified for private loadBalancer\n", service.Namespace, service.Name)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance: return: error: subnet must be specified for private loadBalancer\n")
			return errors.New("subnet must be specified for private loadBalancer")
		}

		if !(*loadBalancer.LoadBalancerType == ClbLoadBalancerTypePrivate && *loadBalancer.VpcId == cloud.txConfig.VpcId && *loadBalancer.SubnetId == loadBalancerTypeInternalSubnetId) {
			klog.Infof("tencentcloud.ensureLoadBalancerInstance: CLB need delete,pls check, ID: %s, Type: %s, ClbLoadBalancerTypePrivate: %s, VpcId: %s, txConfig.VpcId: %s, subnetId: %s, loadBalancerTypeInternalSubnetId: %s\n", *loadBalancer.LoadBalancerId, *loadBalancer.LoadBalancerType, ClbLoadBalancerTypePrivate, *loadBalancer.VpcId, cloud.txConfig.VpcId, *loadBalancer.SubnetId, loadBalancerTypeInternalSubnetId)
			needRecreate = true
		}
		break
	default:
		klog.Warningf("tencentcloud.ensureLoadBalancerInstance: Get error: service annotation " + ServiceAnnotationLoadBalancerType + "must be specified " + LoadBalancerTypePublic + " or " + LoadBalancerTypePrivate)
		return errors.New("service annotation " + ServiceAnnotationLoadBalancerType + "must be specified " + LoadBalancerTypePublic + " or " + LoadBalancerTypePrivate)
	}

	if needRecreate {
		if err := cloud.deleteLoadBalancer(ctx, clusterName, service); err != nil {
			klog.Warningf("tencentcloud.ensureLoadBalancerInstance: Get error: %s\n", err)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance: return: %v\n", err)
			return err
		}
		if err := cloud.createLoadBalancer(ctx, clusterName, service); err != nil {
			klog.Warningf("tencentcloud.ensureLoadBalancerInstance: Get error: %s\n", err)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance: return: %v\n", err)
			return err
		}
	}

	klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance: return: nil\n")
	return nil
}

// ensureLoadBalancerListeners ensure the Tencent Cloud Load Balancer Listeners is the same as the service ports
func (cloud *Cloud) ensureLoadBalancerListeners(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners(\"%s, %T\"): entered\n", clusterName, service)

	loadBalancerName := cloud.getLoadBalancerName(ctx, clusterName, service)
	loadBalancer, err := cloud.getLoadBalancer(loadBalancerName, service)
	if err != nil {
		klog.Warningf("tencentcloud.ensureLoadBalancerListeners: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: return: %v\n", err)
		return err
	}

	loadBalancerListeners, err := cloud.getLoadBalancerListeners(*loadBalancer.LoadBalancerId)
	if err != nil {
		klog.Warningf("tencentcloud.ensureLoadBalancerListeners: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: return: %v\n", err)
		return err
	}

	usedListenerIds := make([]string, 0)
	createdServicePortNames := make([]string, 0)
	findOneListenerValid := func(port v1.ServicePort) (listenerId string) {
		listenerId = ""
		for _, listener := range loadBalancerListeners {
			if *listener.Port == int64(port.Port) && *listener.Protocol == string(port.Protocol) {
				klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: return: %s\n", *listener.ListenerId)
				return *listener.ListenerId
			}
		}
		return
	}

	for _, port := range service.Spec.Ports {
		listenerId := findOneListenerValid(port)
		if listenerId != "" {
			// TODO check if port name is unique
			createdServicePortNames = append(createdServicePortNames, port.Name)
			usedListenerIds = append(usedListenerIds, listenerId)
		}
	}

	sort.Strings(usedListenerIds)
	sort.Strings(createdServicePortNames)
	listenersToCreate := make([]*clb.CreateListenerRequest, 0)
	for _, port := range service.Spec.Ports {
		ensured := false
		if isExist(port.Name, createdServicePortNames) {
			ensured = true
		}
		if !ensured {
			createListenerRequest := clb.NewCreateListenerRequest()
			createListenerRequest.Ports = common.Int64Ptrs([]int64{int64(port.Port)})
			createListenerRequest.ListenerNames = common.StringPtrs([]string{port.Name})
			createListenerRequest.Protocol = common.StringPtr(string(port.Protocol))
			createListenerRequest.LoadBalancerId = common.StringPtr(*loadBalancer.LoadBalancerId)
			createListenerRequest.HealthCheck = cloud.buildHealthCheck(service)
			listenersToCreate = append(listenersToCreate, createListenerRequest)
		}
	}

	listenersToDelete := make([]*clb.DeleteListenerRequest, 0)
	for _, listener := range loadBalancerListeners {
		used := false
		if isExist(*listener.ListenerId, usedListenerIds) {
			used = true
		}
		if !used {
			deleteListenerRequest := clb.NewDeleteListenerRequest()
			deleteListenerRequest.LoadBalancerId = common.StringPtr(*loadBalancer.LoadBalancerId)
			deleteListenerRequest.ListenerId = common.StringPtr(*listener.ListenerId)
			listenersToDelete = append(listenersToDelete, deleteListenerRequest)
		}
	}

	for _, usedListener := range listenersToCreate {
		response, err := cloud.clb.CreateListener(usedListener)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: create listener: CLB_ID:%s, Port:%+v, name:%+v\n", *usedListener.LoadBalancerId, *usedListener.Ports[0], *usedListener.ListenerNames[0])

		if err != nil {
			klog.Warningf("tencentcloud.ensureLoadBalancerListeners: create listener (CLB_ID:%s) error: %s\n", *usedListener.LoadBalancerId, err)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: return: %v\n", err)
			return err
		}
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: create listener: CLB_ID:%s, Port:%+v, name:%+v, RequestID:%s\n", *usedListener.LoadBalancerId, *usedListener.Ports[0], *usedListener.ListenerNames[0], *response.Response.RequestId)
		if err := cloud.waitApiTaskDone(response.Response.RequestId); err != nil {
			klog.Warningf("tencentcloud.ensureLoadBalancerListeners: return: %v\n", err)
			return err
		}
	}

	for _, unusedListener := range listenersToDelete {
		response, err := cloud.clb.DeleteListener(unusedListener)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: delete listener: CLB_ID:%s, ListenerId:%s\n", *unusedListener.LoadBalancerId, *unusedListener.ListenerId)

		if err != nil {
			klog.Warningf("tencentcloud.ensureLoadBalancerListeners: delete listener %s error: %s\n", *unusedListener.ListenerId, err)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: return: %v\n", err)
			return err
		}
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: delete listener: CLB_ID:%s, ListenerId:%s, RequestID:%s\n", *unusedListener.LoadBalancerId, *unusedListener.ListenerId, *response.Response.RequestId)
		if err := cloud.waitApiTaskDone(response.Response.RequestId); err != nil {
			klog.Warningf("tencentcloud.ensureLoadBalancerListeners: return: %v\n", err)
			return err
		}
	}

	klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: return: %s\n", "nil")
	return nil
}

// waitApiTaskDone wait Tencent Cloud async api task done
// tasks *[]string requestId list
func (cloud *Cloud) waitApiTaskDone(task *string) error {
	klog.V(3).Infof("tencentcloud.waitApiTaskDone(\"%s\"): entered\n", *task)
	for i := 0; i < 30; i++ {
		status, err := cloud.getTencentCloudApiTaskStatus(task)
		if err != nil {
			return err
		}
		switch status {
		case 0:
			klog.V(2).Infof("tencentcloud.waitApiTaskDone: Task %s executed successfully.\n", *task)
			return nil
		case 1:
			klog.Warningf("tencentcloud.waitApiTaskDone: Task %s executed failed!\n", *task)
			return nil
		case 2:
			klog.V(2).Infof("tencentcloud.waitApiTaskDone: Task %s executing, Current number: %d, Try again next time.\n", *task, i)
			break
		default:
			klog.Warningf("tencentcloud.waitApiTaskDone: Task %s executed return a not expected value: %d\n", *task, status)
			return errors.New("task" + *task + " executed return a not expected value:" + strconv.FormatInt(status, 10))
		}
		time.Sleep(time.Duration(1) * time.Second)
	}
	klog.Warningf("tencentcloud.waitApiTasksDone: task %s execute timeout!\n", *task)
	return errors.New("task" + *task + " execute timeout:")
}

// getTencentCloudApiTaskStatus return Tencent Cloud async api task status
// return: 任务的当前状态。 0：成功，1：失败，2：进行中, 3: error。
func (cloud *Cloud) getTencentCloudApiTaskStatus(taskId *string) (int64, error) {
	klog.V(3).Infof("tencentcloud.getTencentCloudApiTaskStatus(\"%s\"): entered\n", *taskId)

	request := clb.NewDescribeTaskStatusRequest()
	request.TaskId = common.StringPtr(*taskId)
	response, err := cloud.clb.DescribeTaskStatus(request)
	if err != nil {
		klog.Warningf("tencentcloud.getTencentCloudApiTaskStatus: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.getTencentCloudApiTaskStatus: return(taskId:%s): 3\n", *taskId)
		return 3, err
	}

	klog.V(3).Infof("tencentcloud.getTencentCloudApiTaskStatus: return(taskId:%s): %d\n", *taskId, *response.Response.Status)
	// *response.Response.Status: 任务的当前状态。 0：成功，1：失败，2：进行中。
	return *response.Response.Status, nil
}

// getNodeLabelKey return node annotations key for service annotations(ServiceAnnotationLoadBalancerNodeLabelKey)
func (cloud *Cloud) getNodeLabelKey(service *v1.Service) string {
	klog.V(3).Infof("tencentcloud.getNodeLabelKey(\"%T\"): entered\n", service)
	if _, ok := service.Annotations[ServiceAnnotationLoadBalancerNodeLabelKey]; ok {
		klog.V(3).Infof("tencentcloud.getNodeLabelKey: return(service name:%s): %s\n", service.Name, service.Annotations[ServiceAnnotationLoadBalancerNodeLabelKey])
		return service.Annotations[ServiceAnnotationLoadBalancerNodeLabelKey]
	}
	klog.V(3).Infof("tencentcloud.getNodeLabelKey: return(service name:%s): %s\n", service.Name, nodeLabelKeyOfLoadBalancerDefault)
	return nodeLabelKeyOfLoadBalancerDefault
}

// getNodeLabelValue return node annotations value for service annotations(ServiceAnnotationLoadBalancerNodeLabelValue)
func (cloud *Cloud) getNodeLabelValue(service *v1.Service) string {
	klog.V(3).Infof("tencentcloud.getNodeLabelValue(\"%T\"): entered\n", service)
	if _, ok := service.Annotations[ServiceAnnotationLoadBalancerNodeLabelValue]; ok {
		klog.V(3).Infof("tencentcloud.getNodeLabelValue: return(service name:%s): %s\n", service.Name, service.Annotations[ServiceAnnotationLoadBalancerNodeLabelValue])
		return service.Annotations[ServiceAnnotationLoadBalancerNodeLabelValue]
	}
	klog.V(3).Infof("tencentcloud.getNodeLabelValue: return(service name:%s): %s\n", service.Name, nodeLabelValueOfLoadBalancerDefault)
	return nodeLabelValueOfLoadBalancerDefault
}

// ensureLoadBalancerListeners ensure the backends in Load Balancer Listeners is the same as the service nodes
func (cloud *Cloud) ensureLoadBalancerBackends(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends(\"%s, %T, %T\"): entered\n", clusterName, service, nodes)

	loadBalancerName := cloud.getLoadBalancerName(ctx, clusterName, service)
	loadBalancer, err := cloud.getLoadBalancer(loadBalancerName, service)
	if err != nil {
		klog.Warningf("tencentcloud.ensureLoadBalancerBackends: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "nil")
		return err
	}

	nodeLanIps := make([]string, 0)
	for _, node := range nodes {
		if _, ok := node.Labels[cloud.getNodeLabelKey(service)]; ok {
			if node.Labels[cloud.getNodeLabelKey(service)] == cloud.getNodeLabelValue(service) {
				nodeLanIps = append(nodeLanIps, node.Name)
			}
		}
	}

	if len(nodeLanIps) == 0 {
		klog.Warningf("tencentcloud.ensureLoadBalancerBackends: return error: can't found nodes base on label: " + cloud.getNodeLabelKey(service) + "=" + cloud.getNodeLabelValue(service) + "\n")
		return errors.New("can't found nodes base on label: " + cloud.getNodeLabelKey(service) + "=" + cloud.getNodeLabelValue(service))
	}

	//instancesInMultiVpc, err := cloud.getInstancesByMultiLanIp(ctx, nodeLanIps)
	instancesInMultiVpc, err := cloud.getInstanceByInstancePrivateIps(ctx, nodeLanIps)

	if err != nil {
		klog.Warningf("tencentcloud.ensureLoadBalancerBackends: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "nil")
		return err
	}

	instances := make([]*cvm.Instance, 0)
	for _, instance := range instancesInMultiVpc {
		if *instance.VirtualPrivateCloud.VpcId == cloud.txConfig.VpcId {
			instances = append(instances, instance)
		}
	}

	request := clb.NewDescribeTargetsRequest()
	request.LoadBalancerId = common.StringPtr(*loadBalancer.LoadBalancerId)
	response, err := cloud.clb.DescribeTargets(request)

	if err != nil {
		klog.Warningf("tencentcloud.ensureLoadBalancerBackends: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "nil")
		return err
	}

	forwardListeners := response.Response.Listeners

	// remove unused backends first
	for _, port := range service.Spec.Ports {
		// find listener match this service port
		forwardListener := new(clb.ListenerBackend)
		for _, listener := range forwardListeners {
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: Port: %d, port.Port: %d, Protocol: %s, port.Protocol: %s\n", *listener.Port, int64(port.Port), *listener.Protocol, string(port.Protocol))
			if *listener.Port == int64(port.Port) && *listener.Protocol == string(port.Protocol) {
				klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: forwardListener = listener, listener id: %s", *listener.ListenerId)
				forwardListener = listener
				break
			}
		}

		if forwardListener.ListenerId == nil {
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "err:can not find loadBalancer listener for this service port")
			return errors.New("can not find loadBalancer listener for this service port")
		}

		backendsToDelete := make([]*clb.Target, 0)
		for _, backend := range forwardListener.Targets {

			found := false
			for _, instance := range instances {
				//klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: backend.InstanceId: %s, instance.InstanceId: %s, backend.Port: %d, port.NodePort: %d\n", *backend.InstanceId, *instance.InstanceId, *backend.Port, int64(port.NodePort))
				if *backend.InstanceId == *instance.InstanceId && *backend.Port == int64(port.NodePort) {
					//klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: found = true, *instance.InstanceId: %s", *instance.InstanceId)
					found = true
					break
				}
			}

			if !found {
				backendsToDelete = append(backendsToDelete, &clb.Target{
					InstanceId: backend.InstanceId,
					Port:       backend.Port,
				})
				klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: Add to backendsToDelete, instance.InstanceId: %s", backend.InstanceId)
			}
		}

		if count := len(backendsToDelete); count > 0 {
			backend := make([]*clb.Target, 0)
			for i := 0; i < count; i++ {
				backend = append(backend, backendsToDelete[i])
				if (i > 0 && (i+1)%20 == 0) || i == count-1 {
					err := cloud.deleteLoadBalancerBackends(*loadBalancer.LoadBalancerId, *forwardListener.ListenerId, backend)
					if err != nil {
						klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s %v\n", "", err)
						return err
					}
					klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: deleteLoadBalancerBackends CLB_ID: %s, ListenerId: %s\n", *loadBalancer.LoadBalancerId, *forwardListener.ListenerId)
					backend = nil
				}
			}
		}
	}

	// then add backends needed
	for _, port := range service.Spec.Ports {
		// find listener match this service port
		forwardListener := new(clb.ListenerBackend)
		for _, listener := range forwardListeners {
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: Port: %d, port.Port: %d, Protocol: %s, port.Protocol: %s\n", *listener.Port, int64(port.Port), *listener.Protocol, string(port.Protocol))
			if *listener.Port == int64(port.Port) && *listener.Protocol == string(port.Protocol) {
				klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: forwardListener = listener, listener id: %s", *listener.ListenerId)
				forwardListener = listener
				break
			}
		}

		if forwardListener.ListenerId == nil {
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "err:can not find loadBalancer listener for this service port")
			return errors.New("can not find loadBalancer listener for this service port")
		}

		backendsToAdd := make([]*clb.Target, 0)
		for _, instance := range instances {
			found := false
			for _, backend := range forwardListener.Targets {
				//klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: backend.InstanceId: %s, instance.InstanceId: %s, backend.Port: %d, port.NodePort: %d\n", *backend.InstanceId, *instance.InstanceId, *backend.Port, int64(port.NodePort))
				if *backend.InstanceId == *instance.InstanceId && *backend.Port == int64(port.NodePort) {
					//klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: found = true, *instance.InstanceId: %s", *instance.InstanceId)
					found = true
					break
				}
			}

			if !found {
				nodePort := int64(port.NodePort)
				backendsToAdd = append(backendsToAdd, &clb.Target{
					InstanceId: instance.InstanceId,
					Port:       &nodePort,
				})
				klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: Add to backendsToAdd, instance.InstanceId: %s", *instance.InstanceId)
			}
		}

		if count := len(backendsToAdd); count > 0 {
			backend := make([]*clb.Target, 0)
			for i := 0; i < count; i++ {
				backend = append(backend, backendsToAdd[i])
				if (i > 0 && (i+1)%20 == 0) || i == count-1 {
					err := cloud.addLoadBalancerBackends(*loadBalancer.LoadBalancerId, *forwardListener.ListenerId, backend)
					if err != nil {
						klog.Warningf("tencentcloud.ensureLoadBalancerBackends: Get error: %s, backend count=%d\n", err, len(backend))
						klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %v\n", err)
						return err
					}
					klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: addLoadBalancerBackends CLB_ID:%s,ListenerId:%s \n", *loadBalancer.LoadBalancerId, *forwardListener.ListenerId)
					backend = nil
				}
			}
		}
	}

	klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "nil")
	return nil
}

// addLoadBalancerBackends add Tencent Cloud Load Balancer Backends, return Tencent Cloud RequestId
func (cloud *Cloud) addLoadBalancerBackends(loadBalancerId string, listenerId string, backends []*clb.Target) error {
	klog.V(3).Infof("tencentcloud.addLoadBalancerBackends(\"%s %s %T\"): entered\n", loadBalancerId, listenerId, backends)
	for _, backend := range backends {
		klog.V(3).Infof("tencentcloud.addLoadBalancerBackends: add backend instanceId: %s\n", *backend.InstanceId)
	}

	request := clb.NewRegisterTargetsRequest()
	request.LoadBalancerId = common.StringPtr(loadBalancerId)
	request.ListenerId = common.StringPtr(listenerId)
	request.Targets = backends

	response, err := cloud.clb.RegisterTargets(request)
	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.addLoadBalancerBackends: tencentcloud API error: %s\n", err)
		klog.V(3).Infof("tencentcloud.addLoadBalancerBackends: return: %s %v\n", "", err)
		return err
	}
	if err != nil {
		klog.Warningf("tencentcloud.addLoadBalancerBackends: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.addLoadBalancerBackends: return: %s %v\n", "", err)
		return err
	}

	if err := cloud.waitApiTaskDone(response.Response.RequestId); err != nil {
		klog.Warningf("tencentcloud.addLoadBalancerBackends: return: %v\n", err)
		return err
	}

	klog.V(3).Infof("tencentcloud.addLoadBalancerBackends: return: %s, backends count: %d\n", "nil", len(backends))
	return nil
}

// deleteLoadBalancerBackends delete Tencent Cloud Load Balancer Backends, return Tencent Cloud RequestId
func (cloud *Cloud) deleteLoadBalancerBackends(loadBalancerId string, listenerId string, backends []*clb.Target) error {
	klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends(\"%s %s %T\"): entered\n", loadBalancerId, listenerId, backends)
	for _, backend := range backends {
		klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends: delete backend instanceId: %s\n", *backend.InstanceId)
	}

	request := clb.NewDeregisterTargetsRequest()
	request.LoadBalancerId = common.StringPtr(loadBalancerId)
	request.ListenerId = common.StringPtr(listenerId)
	request.Targets = backends
	response, err := cloud.clb.DeregisterTargets(request)

	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.deleteLoadBalancerBackends: tencentcloud API error: %s\n", err)
		klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends: return:  %v\n", err)
		return err
	}
	if err != nil {
		klog.Warningf("tencentcloud.deleteLoadBalancerBackends: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends: return: %v\n", err)
		return err
	}

	if err := cloud.waitApiTaskDone(response.Response.RequestId); err != nil {
		klog.Warningf("tencentcloud.addLoadBalancerBackends: return: %v\n", err)
		return err
	}

	klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends: return: %s, backends count: %d\n", "nil", len(backends))
	return nil
}

// createLoadBalancer Create Tencent Cloud Load Balancer
func (cloud *Cloud) createLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(3).Infof("tencentcloud.createLoadBalancer(\"%s %T\"): entered\n", clusterName, *service)

	loadBalancerName := cloud.getLoadBalancerName(ctx, clusterName, service)
	request := clb.NewCreateLoadBalancerRequest()
	loadBalancerDesiredType, ok := service.Annotations[ServiceAnnotationLoadBalancerType]
	if !ok || (loadBalancerDesiredType != LoadBalancerTypePrivate && loadBalancerDesiredType != LoadBalancerTypePublic) {
		loadBalancerDesiredType = LoadBalancerTypePrivate
	}
	switch loadBalancerDesiredType {
	case LoadBalancerTypePrivate:
		request.LoadBalancerType = &ClbLoadBalancerTypePrivate
		loadBalancerDesiredSubnetId, ok := service.Annotations[ServiceAnnotationLoadBalancerTypeInternalSubnetId]
		if !ok {
			klog.Warningf("tencentcloud.createLoadBalancer: Get error: subnet must be specified for private loadBalancer\n")
			return errors.New("subnet must be specified for private loadBalancer")
		}
		klog.V(3).Infof("tencentcloud.createLoadBalancer: loadBalancerName: %s, SubnetId: %s\n", loadBalancerName, loadBalancerDesiredSubnetId)
		request.SubnetId = &loadBalancerDesiredSubnetId
	case LoadBalancerTypePublic:
		request.LoadBalancerType = &ClbLoadBalancerTypePublic
	}
	request.LoadBalancerName = common.StringPtr(loadBalancerName)
	request.VpcId = common.StringPtr(cloud.txConfig.VpcId)
	request.Tags = cloud.getLBTags(ctx, service)
	request.LoadBalancerPassToTarget = &loadBalancerPassToTarget

	response, err := cloud.clb.CreateLoadBalancer(request)
	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.createLoadBalancer: loadBalancerName: %s, tencentcloud API error: %s\n", loadBalancerName, err)
		klog.V(3).Infof("tencentcloud.createLoadBalancer: loadBalancerName: %s, return: %v\n", loadBalancerName, err)
		return err
	}
	if err != nil {
		klog.Warningf("tencentcloud.createLoadBalancer: loadBalancerName: %s, Get error: %s\n", loadBalancerName, err)
		klog.V(3).Infof("tencentcloud.createLoadBalancer: loadBalancerName: %s, return: %s %v\n", "", loadBalancerName, err)
		return err
	}
	klog.V(3).Infof("tencentcloud.createLoadBalancer: loadBalancerName: %s, requestId: %s, VpcID: %s\n", loadBalancerName, *response.Response.RequestId)

	if err := cloud.waitApiTaskDone(response.Response.RequestId); err != nil {
		klog.Warningf("tencentcloud.createLoadBalancer: loadBalancerName: %s, return:  %v\n", loadBalancerName, err)
		return err
	}

	klog.V(3).Infof("tencentcloud.createLoadBalancer: exit\n")
	return nil
}

// getTags get new load balancer tags
func (cloud *Cloud) getLBTags(ctx context.Context, service *v1.Service) []*clb.TagInfo {
	var tags []*clb.TagInfo

	t := clb.TagInfo{
		TagKey:   &cloud.txConfig.TagKey,
		TagValue: &cloud.txConfig.CLBNamePrefix,
	}
	tags = append(tags, &t)

	tServiceValue := string(service.UID)
	tService := clb.TagInfo{
		TagKey:   &ClbTagServiceKey,
		TagValue: &tServiceValue,
	}
	tags = append(tags, &tService)
	return tags
}

// buildHealthCheck build load balancer HealthCheck
func (cloud *Cloud) buildHealthCheck(service *v1.Service) *clb.HealthCheck {
	var healthSwitch int64 = 1
	var sourceType int64 = 1
	var timeout int64 = 2
	var intervalTime int64 = 5
	var healthNum int64 = 3
	var unHealthNum int64 = 3

	sHealthSwitch, ok := service.Annotations[ServiceAnnotationLoadBalancerHealthCheckSwitch]
	if ok {
		healthSwitch, _ = strconv.ParseInt(sHealthSwitch, 10, 64)
	}
	sTimeout, ok := service.Annotations[ServiceAnnotationLoadBalancerHealthCheckTimeout]
	if ok {
		timeout, _ = strconv.ParseInt(sTimeout, 10, 64)
	}
	sIntervalTime, ok := service.Annotations[ServiceAnnotationLoadBalancerHealthCheckIntervalTime]
	if ok {
		intervalTime, _ = strconv.ParseInt(sIntervalTime, 10, 64)
	}
	sHealthNum, ok := service.Annotations[ServiceAnnotationLoadBalancerHealthCheckHealthNum]
	if ok {
		healthNum, _ = strconv.ParseInt(sHealthNum, 10, 64)
	}
	sUnHealthNum, ok := service.Annotations[ServiceAnnotationLoadBalancerHealthCheckUnHealthNum]
	if ok {
		unHealthNum, _ = strconv.ParseInt(sUnHealthNum, 10, 64)
	}

	healthCheck := &clb.HealthCheck{
		HealthSwitch: &healthSwitch,
		SourceIpType: &sourceType,
		TimeOut:      &timeout,
		IntervalTime: &intervalTime,
		HealthNum:    &healthNum,
		UnHealthNum:  &unHealthNum,
	}

	return healthCheck
}

// createLoadBalancer Delete Tencent Cloud Load Balancer
func (cloud *Cloud) deleteLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(3).Infof("tencentcloud.deleteLoadBalancer(\"%s, %T\"): entered\n", clusterName, service)

	loadBalancerName := cloud.getLoadBalancerName(ctx, clusterName, service)
	loadBalancer, err := cloud.getLoadBalancer(loadBalancerName, service)
	if err != nil {
		if err == ErrCloudLoadBalancerNotFound {
			klog.Warningf("tencentcloud.deleteLoadBalancer: Get error: %s\n", err)
			klog.V(3).Infof("tencentcloud.deleteLoadBalancer: return: nil\n")
			return nil
		}
		klog.Warningf("tencentcloud.deleteLoadBalancer: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.deleteLoadBalancer: return: %v\n", err)
		return err
	}
	request := clb.NewDeleteLoadBalancerRequest()
	request.LoadBalancerIds = common.StringPtrs([]string{*loadBalancer.LoadBalancerId})
	klog.V(3).Infof("tencentcloud.deleteLoadBalancer: LoadBalancerId: %s\n", *loadBalancer.LoadBalancerId)
	response, err := cloud.clb.DeleteLoadBalancer(request)
	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.deleteLoadBalancer: tencentcloud API error: %s\n", err)
		klog.V(3).Infof("tencentcloud.deleteLoadBalancer: return: %v\n", err)
		return err
	}
	if err != nil {
		klog.Warningf("tencentcloud.deleteLoadBalancer: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.deleteLoadBalancer: return: %s %v\n", "", err)
		return err
	}
	klog.V(3).Infof("tencentcloud.deleteLoadBalancer: requestId: %s\n", *response.Response.RequestId)

	if err := cloud.waitApiTaskDone(response.Response.RequestId); err != nil {
		if cacheKey := cacheNamePreCLB + loadBalancerName; cloud.cache.Delete(cacheKey) {
			klog.Infof("tencentcloud.deleteLoadBalancer: delete cache done. key: %s\n", cacheKey)
		} else {
			klog.Warningf("tencentcloud.deleteLoadBalancer: delete cache fail. key: %s\n", cacheKey)
		}
		klog.Warningf("tencentcloud.deleteLoadBalancer: return: %v\n", err)
		return err
	}

	klog.V(3).Infof("tencentcloud.deleteLoadBalancer: exit\n")
	return nil
}
