package tencentcloud

import (
	"context"
	"errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

var (
	ErrCloudLoadBalancerNotFound = errors.New("LoadBalancer not found")
)

// GetLoadBalancer returns whether the specified load balancer exists, and
// if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (cloud *Cloud) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	klog.V(3).Infof("tencentcloud.GetLoadBalancer(\"%s, %T\"): entered\n", clusterName, *service)
	loadBalancerName := cloud.getLoadBalancerName(ctx, clusterName, service)
	loadBalancer, err := cloud.getLoadBalancerByName(loadBalancerName)
	if err != nil {
		klog.Warningf("tencentcloud.GetLoadBalancer: Get error: %v\n", err)
		if err == ErrCloudLoadBalancerNotFound {
			klog.V(3).Infof("tencentcloud.GetLoadBalancer: return:  nil, false, nil\n")
			return nil, false, nil
		}
		klog.V(3).Infof("tencentcloud.GetLoadBalancer: return:  nil, false, %v\n", err)
		return nil, false, err
	}

	ingresses := make([]v1.LoadBalancerIngress, len(loadBalancer.LoadBalancerVips))

	for i, vip := range loadBalancer.LoadBalancerVips {
		ingresses[i] = v1.LoadBalancerIngress{IP: *vip}
	}

	ret := &v1.LoadBalancerStatus{
		Ingress: ingresses,
	}
	klog.V(3).Infof("tencentcloud.GetLoadBalancer: return:  %v, %v, %v\n", *ret, true, nil)
	return ret, true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (cloud *Cloud) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	klog.V(3).Infof("tencentcloud.GetLoadBalancerName(\"%s, %T\"): entered\n", clusterName, *service)

	return cloud.getLoadBalancerName(ctx, clusterName, service)
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (cloud *Cloud) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	klog.V(3).Infof("tencentcloud.EnsureLoadBalancer(\"%s, %T, %T\"): entered\n", clusterName, *service, nodes)

	if service.Spec.SessionAffinity != v1.ServiceAffinityNone {
		klog.Warningf("tencentcloud.EnsureLoadBalancer: Get error: service (nameSpace:%s,name:%s) SessionAffinity is not supported currently\n", service.Namespace, service.Name)
		klog.V(3).Infof("tencentcloud.EnsureLoadBalancer: return: error: SessionAffinity is not supported currently\n")
		return nil, errors.New("SessionAffinity is not supported currently")
	}

	// TODO check if kubernetes has already do validate
	// 1. ensure loadbalancer created
	err := cloud.ensureLoadBalancerInstance(ctx, clusterName, service)
	if err != nil {
		return nil, err
	}
	// 2. ensure loadbalancer listener created
	err = cloud.ensureLoadBalancerListeners(ctx, clusterName, service)
	if err != nil {
		return nil, err
	}
	// 3. ensure right hosts is bounded to loadbalancer
	err = cloud.ensureLoadBalancerBackends(ctx, clusterName, service, nodes)
	if err != nil {
		return nil, err
	}

	loadBalancer, err := cloud.getLoadBalancerByName(cloud.getLoadBalancerName(ctx, clusterName, service))
	if err != nil {
		return nil, err
	}

	ingresses := make([]v1.LoadBalancerIngress, len(loadBalancer.LoadBalancerVips))

	for i, vip := range loadBalancer.LoadBalancerVips {
		ingresses[i] = v1.LoadBalancerIngress{IP: *vip}
	}

	ret := &v1.LoadBalancerStatus{
		Ingress: ingresses,
	}
	klog.V(3).Infof("tencentcloud.EnsureLoadBalancer: return:  %+v, nil\n", *ret, nil)
	return ret, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (cloud *Cloud) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.V(3).Infof("tencentcloud.UpdateLoadBalancer(\"%s, %T, %T\"): entered\n", clusterName, *service, nodes)
	return cloud.ensureLoadBalancerBackends(ctx, clusterName, service, nodes)
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (cloud *Cloud) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(3).Infof("tencentcloud.EnsureLoadBalancerDeleted(\"%s, %T\"): entered\n", clusterName, *service)
	_, err := cloud.getLoadBalancerByName(cloud.GetLoadBalancerName(ctx, clusterName, service))
	if err != nil {
		if err == ErrCloudLoadBalancerNotFound {
			klog.V(3).Infof("tencentcloud.EnsureLoadBalancerDeleted: return:  nil\n")
			return nil
		}
	}

	return cloud.deleteLoadBalancer(ctx, clusterName, service)
}
