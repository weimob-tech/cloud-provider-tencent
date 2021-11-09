package tencentcloud

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	cloudErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"

	clb "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/clb/v20180317"
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
	ServiceAnnotationLoadBalancerTypeInternalSubnetId = "service.beta.kubernetes.io/tencentcloud-loadbalancer-type-internal-subnet-id"
	ServiceAnnotationLoadBalancerNodeLabelKey         = "service.beta.kubernetes.io/tencentcloud-loadbalancer-node-label-key"
	ServiceAnnotationLoadBalancerNodeLabelValue       = "service.beta.kubernetes.io/tencentcloud-loadbalancer-node-label-value"
	nodeLabelKeyOfLoadBalancerDefault                 = "kubernetes.io/role"
	nodeLabelValueOfLoadBalancerDefault               = "node"
)

var (
	ClbLoadBalancerTypePublic         = "OPEN"
	ClbLoadBalancerTypePrivate        = "INTERNAL"
	cacheNamePreCLBListener    string = "clb_listener_id_" //cache key name pre for clb listener id
	cacheNamePreCLB            string = "clb_id_"          //cache key name pre for clb id
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

// getLoadBalancerByName return Tencent Cloud LoadBalancer for LoadBalancer name
func (cloud *Cloud) getLoadBalancerByName(name string) (*clb.LoadBalancer, error) {
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

	if len(response.Response.LoadBalancerSet) < 1 {
		klog.V(3).Infof("tencentcloud.getLoadBalancerByName: return: nil, %v\n", name, ErrCloudLoadBalancerNotFound)
		return nil, ErrCloudLoadBalancerNotFound
	}

	cloud.cache.Set(cacheKey, response.Response.LoadBalancerSet[0])
	klog.V(3).Infof("tencentcloud.getLoadBalancerByName: return(name:%s): %T, nil\n", name, response.Response.LoadBalancerSet[0])
	return response.Response.LoadBalancerSet[0], nil
}

// ensureLoadBalancerInstance ensure the Tencent Cloud Load Balancer Instance is the same as the service
func (cloud *Cloud) ensureLoadBalancerInstance(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance(\"%s %T\"): entered\n", clusterName, service)
	loadBalancerName := cloud.getLoadBalancerName(ctx, clusterName, service)
	loadBalancer, err := cloud.getLoadBalancerByName(loadBalancerName)

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
	switch {
	case loadBalancerDesiredType == LoadBalancerTypePublic:
		if !(*loadBalancer.LoadBalancerType == ClbLoadBalancerTypePublic && *loadBalancer.VpcId == cloud.txConfig.VpcId) {
			needRecreate = true
		}
	case loadBalancerDesiredType == LoadBalancerTypePrivate:
		loadBalancerTypeInternalSubnetId, ok := service.Annotations[ServiceAnnotationLoadBalancerTypeInternalSubnetId]
		if !ok {
			klog.Warningf("tencentcloud.ensureLoadBalancerInstance: Get error: service (nameSpace:%s,name:%s) subnet must be specified for private loadBalancer\n", service.Namespace, service.Name)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerInstance: return: error: subnet must be specified for private loadBalancer\n")
			return errors.New("subnet must be specified for private loadBalancer")
		}

		if !(*loadBalancer.LoadBalancerType == ClbLoadBalancerTypePrivate && *loadBalancer.VpcId == cloud.txConfig.VpcId && *loadBalancer.SubnetId == loadBalancerTypeInternalSubnetId) {
			needRecreate = true
		}
	default:
		needRecreate = true
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
	loadBalancer, err := cloud.getLoadBalancerByName(loadBalancerName)
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

	listenersToCreate := make([]*clb.CreateListenerRequest, 0)
	for _, port := range service.Spec.Ports {
		ensured := false

		for _, portName := range createdServicePortNames {
			if port.Name == portName {
				ensured = true
				break
			}
		}

		if !ensured {
			createListenerRequest := clb.NewCreateListenerRequest()
			createListenerRequest.Ports = common.Int64Ptrs([]int64{int64(port.Port)})
			createListenerRequest.ListenerNames = common.StringPtrs([]string{port.Name})
			createListenerRequest.Protocol = common.StringPtr(string(port.Protocol))
			createListenerRequest.LoadBalancerId = common.StringPtr(*loadBalancer.LoadBalancerId)
			listenersToCreate = append(listenersToCreate, createListenerRequest)
		}
	}

	listenersToDelete := make([]*clb.DeleteListenerRequest, 0)
	for _, listener := range loadBalancerListeners {
		used := false

		for _, usedListenerId := range usedListenerIds {
			if *listener.ListenerId == usedListenerId {
				used = true
			}
		}

		if !used {
			deleteListenerRequest := clb.NewDeleteListenerRequest()
			deleteListenerRequest.LoadBalancerId = common.StringPtr(*loadBalancer.LoadBalancerId)
			deleteListenerRequest.ListenerId = common.StringPtr(*listener.ListenerId)
			listenersToDelete = append(listenersToDelete, deleteListenerRequest)
		}
	}

	apiTasks := make([]string, 0)
	defer cloud.waitApiTasksDone(&apiTasks)

	for _, usedListener := range listenersToCreate {
		response, err := cloud.clb.CreateListener(usedListener)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: create listener: CLB_ID:%s, Port:%+v, name:%+v\n", *usedListener.LoadBalancerId, usedListener.Ports, usedListener.ListenerNames)

		if err != nil {
			klog.Warningf("tencentcloud.ensureLoadBalancerListeners: create listener (CLB_ID:%s) error: %s\n", *usedListener.LoadBalancerId, err)
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: return: %v\n", err)
			return err
		}
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: create listener: CLB_ID:%s, Port:%+v, name:%+v, RequestID:%s\n", *usedListener.LoadBalancerId, usedListener.Ports, usedListener.ListenerNames, *response.Response.RequestId)
		apiTasks = append(apiTasks, *response.Response.RequestId)
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
		apiTasks = append(apiTasks, *response.Response.RequestId)
	}

	klog.V(3).Infof("tencentcloud.ensureLoadBalancerListeners: return: %s\n", "nil")
	return nil
}

// waitApiTasksDone wait Tencent Cloud async api task done
// tasks *[]string requestId list
func (cloud *Cloud) waitApiTasksDone(tasks *[]string) {
	klog.V(3).Infof("tencentcloud.waitApiTasksDone(\"%v\"): entered\n", tasks)
	wg := &sync.WaitGroup{}
	wg.Add(len(*tasks))
	for _, task := range *tasks {
		go func(task string, wg *sync.WaitGroup) {
			for {
				status := cloud.getTencentCloudApiTaskStatus(&task)
				if status == 0 {
					klog.V(2).Infof("tencentcloud.waitApiTasksDone: Task %s executed successfully.\n", task)
					wg.Done()
					break
				} else if status == 1 {
					klog.Warningf("tencentcloud.waitApiTasksDone: Task %s executed failed!\n", task)
				}
				time.Sleep(time.Duration(1) * time.Second)
			}
		}(task, wg)
	}
	wg.Wait()
	klog.V(3).Infof("tencentcloud.waitApiTasksDone: exit\n")
}

// getTencentCloudApiTaskStatus return Tencent Cloud async api task status
func (cloud *Cloud) getTencentCloudApiTaskStatus(taskId *string) int64 {
	klog.V(3).Infof("tencentcloud.getTencentCloudApiTaskStatus(\"%s\"): entered\n", *taskId)

	request := clb.NewDescribeTaskStatusRequest()
	request.TaskId = common.StringPtr(*taskId)
	response, err := cloud.clb.DescribeTaskStatus(request)
	if err != nil {
		klog.Warningf("tencentcloud.getTencentCloudApiTaskStatus: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.getTencentCloudApiTaskStatus: return(taskId:%s): 3\n", *taskId)
		return 3
	}

	klog.V(3).Infof("tencentcloud.getTencentCloudApiTaskStatus: return(taskId:%s): %d\n", *taskId, *response.Response.Status)
	return *response.Response.Status
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
	loadBalancer, err := cloud.getLoadBalancerByName(loadBalancerName)
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

	instancesInMultiVpc, err := cloud.getInstancesByMultiLanIp(ctx, nodeLanIps)
	if err != nil {
		klog.Warningf("tencentcloud.ensureLoadBalancerBackends: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "nil")
		return err
	}

	instances := make([]cvm.Instance, 0)
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

	apiTasks := make([]string, 0)
	defer cloud.waitApiTasksDone(&apiTasks)

	// remove unused backends first
	for _, port := range service.Spec.Ports {
		// find listener match this service port
		forwardListener := new(clb.ListenerBackend)
		for _, listener := range forwardListeners {
			if *listener.Port == int64(port.Port) && *listener.Protocol == string(port.Protocol) {
				forwardListener = listener
				break
			}
		}

		if forwardListener == nil {
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "err:can not find loadBalancer listener for this service port")
			return errors.New("can not find loadBalancer listener for this service port")
		}

		backendsToDelete := make([]*clb.Target, 0)
		for _, backend := range forwardListener.Targets {

			found := false
			for _, instance := range instances {
				if *backend.InstanceId == *instance.InstanceId && *backend.Port == int64(port.NodePort) {
					found = true
				}
			}

			if !found {
				backendsToDelete = append(backendsToDelete, &clb.Target{
					InstanceId: backend.InstanceId,
					Port:       backend.Port,
				})
			}
		}

		if len(backendsToDelete) > 0 {
			requestId, err := cloud.deleteLoadBalancerBackends(*loadBalancer.LoadBalancerId, *forwardListener.ListenerId, backendsToDelete)
			if err != nil {
				klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s %v\n", "", err)
				return err
			}
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: deleteLoadBalancerBackends CLB_ID:%s,ListenerId:%s,requestId:%s \n", *loadBalancer.LoadBalancerId, *forwardListener.ListenerId, requestId)
			apiTasks = append(apiTasks, requestId)
		}
	}

	// then add backends needed
	for _, port := range service.Spec.Ports {
		// find listener match this service port
		forwardListener := new(clb.ListenerBackend)
		for _, listener := range forwardListeners {
			if *listener.Port == int64(port.Port) && *listener.Protocol == string(port.Protocol) {
				forwardListener = listener
				break
			}
		}

		if forwardListener == nil {
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "err:can not find loadBalancer listener for this service port")
			return errors.New("can not find loadBalancer listener for this service port")
		}

		backendsToAdd := make([]*clb.Target, 0)
		for _, instance := range instances {
			found := false

			for _, backend := range forwardListener.Targets {
				if *backend.InstanceId == *instance.InstanceId && *backend.Port == int64(port.NodePort) {
					found = true
				}
			}

			if !found {
				nodePort := int64(port.NodePort)
				backendsToAdd = append(backendsToAdd, &clb.Target{
					InstanceId: instance.InstanceId,
					Port:       &nodePort,
				})
			}
		}

		if len(backendsToAdd) > 0 {
			requestId, err := cloud.addLoadBalancerBackends(*loadBalancer.LoadBalancerId, *forwardListener.ListenerId, backendsToAdd)
			if err != nil {
				klog.Warningf("tencentcloud.ensureLoadBalancerBackends: Get error: %s\n", err)
				klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %v\n", err)
				return err
			}
			klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: addLoadBalancerBackends CLB_ID:%s,ListenerId:%s,requestId:%s \n", *loadBalancer.LoadBalancerId, *forwardListener.ListenerId, requestId)
			apiTasks = append(apiTasks, requestId)
		}
	}

	klog.V(3).Infof("tencentcloud.ensureLoadBalancerBackends: return: %s\n", "nil")
	return nil
}

// addLoadBalancerBackends add Tencent Cloud Load Balancer Backends, return Tencent Cloud RequestId
func (cloud *Cloud) addLoadBalancerBackends(loadBalancerId string, listenerId string, backends []*clb.Target) (string, error) {
	klog.V(3).Infof("tencentcloud.addLoadBalancerBackends(\"%s %s %T\"): entered\n", loadBalancerId, listenerId, backends)

	request := clb.NewRegisterTargetsRequest()
	request.LoadBalancerId = common.StringPtr(loadBalancerId)
	request.ListenerId = common.StringPtr(listenerId)
	request.Targets = backends

	response, err := cloud.clb.RegisterTargets(request)
	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.addLoadBalancerBackends: tencentcloud API error: %s\n", err)
		klog.V(3).Infof("tencentcloud.addLoadBalancerBackends: return: %s %v\n", "", err)
		return "", err
	}
	if err != nil {
		klog.Warningf("tencentcloud.addLoadBalancerBackends: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.addLoadBalancerBackends: return: %s %v\n", "", err)
		return "", err
	}

	klog.V(3).Infof("tencentcloud.addLoadBalancerBackends: return: %s %v\n", *response.Response.RequestId, nil)
	return *response.Response.RequestId, nil
}

// deleteLoadBalancerBackends delete Tencent Cloud Load Balancer Backends, return Tencent Cloud RequestId
func (cloud *Cloud) deleteLoadBalancerBackends(loadBalancerId string, listenerId string, backends []*clb.Target) (string, error) {
	klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends(\"%s %s %T\"): entered\n", loadBalancerId, listenerId, backends)

	request := clb.NewDeregisterTargetsRequest()
	request.LoadBalancerId = common.StringPtr(loadBalancerId)
	request.ListenerId = common.StringPtr(listenerId)
	request.Targets = backends
	response, err := cloud.clb.DeregisterTargets(request)

	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.deleteLoadBalancerBackends: tencentcloud API error: %s\n", err)
		klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends: return: %s %v\n", "", err)
		return "", err
	}
	if err != nil {
		klog.Warningf("tencentcloud.deleteLoadBalancerBackends: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends: return: %s %v\n", "", err)
		return "", err
	}

	klog.V(3).Infof("tencentcloud.deleteLoadBalancerBackends: return: %s %v\n", *response.Response.RequestId, nil)
	return *response.Response.RequestId, nil
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
	if loadBalancerDesiredType == LoadBalancerTypePrivate {
		loadBalancerDesiredSubnetId, ok := service.Annotations[ServiceAnnotationLoadBalancerTypeInternalSubnetId]
		if !ok {
			klog.Warningf("tencentcloud.createLoadBalancer: Get error: subnet must be specified for private loadBalancer\n")
			klog.V(3).Infof("tencentcloud.createLoadBalancer: return: error: subnet must be specified for private loadBalancer\n")
			return errors.New("subnet must be specified for private loadBalancer")
		}
		request.SubnetId = &loadBalancerDesiredSubnetId
	}
	//} else {
	//	request.ZoneId = common.StringPtr(cloud.txConfig.Region)
	//}
	switch loadBalancerDesiredType {
	case LoadBalancerTypePrivate:
		request.LoadBalancerType = &ClbLoadBalancerTypePrivate
	case LoadBalancerTypePublic:
		request.LoadBalancerType = &ClbLoadBalancerTypePublic
	default:
		request.LoadBalancerType = &ClbLoadBalancerTypePrivate
	}

	request.LoadBalancerName = common.StringPtr(loadBalancerName)
	request.VpcId = common.StringPtr(cloud.txConfig.VpcId)

	response, err := cloud.clb.CreateLoadBalancer(request)
	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.createLoadBalancer: tencentcloud API error: %s\n", err)
		klog.V(3).Infof("tencentcloud.createLoadBalancer: return: %v\n", err)
		return err
	}
	if err != nil {
		klog.Warningf("tencentcloud.createLoadBalancer: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.createLoadBalancer: return: %s %v\n", "", err)
		return err
	}
	klog.V(3).Infof("tencentcloud.createLoadBalancer: requestId: %s\n", *response.Response.RequestId)

	apiTasks := make([]string, 0)
	apiTasks = append(apiTasks, *response.Response.RequestId)
	defer cloud.waitApiTasksDone(&apiTasks)

	klog.V(3).Infof("tencentcloud.createLoadBalancer: exit\n")
	return nil
}

// createLoadBalancer Delete Tencent Cloud Load Balancer
func (cloud *Cloud) deleteLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(3).Infof("tencentcloud.deleteLoadBalancer(\"%s, %T\"): entered\n", clusterName, service)

	loadBalancerName := cloud.getLoadBalancerName(ctx, clusterName, service)
	loadBalancer, err := cloud.getLoadBalancerByName(loadBalancerName)
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

	apiTasks := make([]string, 0)
	apiTasks = append(apiTasks, *response.Response.RequestId)
	defer cloud.waitApiTasksDone(&apiTasks)

	klog.V(3).Infof("tencentcloud.deleteLoadBalancer: exit\n")
	return nil
}

// getInstancesByMultiLanIp return Tencent Cloud []cvm.Instance
func (cloud *Cloud) getInstancesByMultiLanIp(ctx context.Context, ips []string) ([]cvm.Instance, error) {
	klog.V(3).Infof("tencentcloud.getInstancesByMultiLanIp(\"%+v\"): entered\n", ips)

	instances := make([]cvm.Instance, 0)
	for _, ip := range ips {
		instance, err := cloud.getInstanceByInstancePrivateIp(ctx, ip)
		if err != nil {
			klog.Warningf("tencentcloud.getInstancesByMultiLanIp: Get error: %s, %v\n", ip, err)
			break
		}
		instances = append(instances, *instance)
	}

	klog.V(3).Infof("tencentcloud.getInstancesByMultiLanIp: return: %T nil\n", instances)
	return instances, nil
}
