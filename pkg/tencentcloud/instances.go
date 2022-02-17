package tencentcloud

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	cloudErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudProvider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

var (
	cacheNamePreVmIp string = "vm_ip_" //cache key name pre for vm ip
	cacheNamePreVmID string = "vm_id_" //cache key name pre for vm id
)

// NodeAddresses returns the addresses of the specified instance.
func (cloud *Cloud) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	klog.V(3).Infof("tencentcloud.NodeAddresses(\"%s\"): entered\n", string(name))
	node, err := cloud.getInstanceByInstancePrivateIp(ctx, string(name))
	if err != nil {
		klog.Warningf("tencentcloud.NodeAddresses: tencentcloud API error: %v\n", err)
		klog.V(3).Infof("tencentcloud.NodeAddresses: return: {}, %v\n", err)
		return []v1.NodeAddress{}, err
	}
	addresses := make([]v1.NodeAddress, len(node.PrivateIpAddresses)+len(node.PublicIpAddresses))
	for idx, ip := range node.PrivateIpAddresses {
		addresses[idx] = v1.NodeAddress{Type: v1.NodeInternalIP, Address: *ip}
	}
	for idx, ip := range node.PublicIpAddresses {
		addresses[len(node.PrivateIpAddresses)+idx] = v1.NodeAddress{Type: v1.NodeExternalIP, Address: *ip}
	}

	klog.V(3).Infof("tencentcloud.NodeAddresses: return: %v, nil\n", addresses)
	return addresses, nil
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses
func (cloud *Cloud) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.V(3).Infof("tencentcloud.NodeAddressesByProviderID(\"%s\"): entered\n", providerID)
	instance, err := cloud.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		klog.Warningf("tencentcloud.NodeAddressesByProviderID: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.NodeAddressesByProviderID: return: {}, %v\n", err)
		return []v1.NodeAddress{}, err
	}
	addresses := make([]v1.NodeAddress, len(instance.PrivateIpAddresses)+len(instance.PublicIpAddresses))
	for idx, ip := range instance.PrivateIpAddresses {
		addresses[idx] = v1.NodeAddress{Type: v1.NodeInternalIP, Address: *ip}
	}
	for idx, ip := range instance.PublicIpAddresses {
		addresses[len(instance.PrivateIpAddresses)+idx] = v1.NodeAddress{Type: v1.NodeExternalIP, Address: *ip}
	}

	klog.V(3).Infof("tencentcloud.NodeAddressesByProviderID: return: %v, nil\n", addresses)
	return addresses, nil
}

// ExternalID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist or is no longer running, we must return ("", cloudprovider.InstanceNotFound)
func (cloud *Cloud) ExternalID(ctx context.Context, nodeName types.NodeName) (string, error) {
	klog.V(3).Infof("tencentcloud.ExternalID(\"%s\"): entered\n", string(nodeName))
	node, err := cloud.getInstanceByInstancePrivateIp(ctx, string(nodeName))
	if err != nil {
		klog.Warningf("tencentcloud.ExternalID: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.ExternalID: return: '', %v\n", err)
		return "", err
	}

	klog.V(3).Infof("tencentcloud.ExternalID: return: %v, nil\n", *node.InstanceId)
	return *node.InstanceId, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
func (cloud *Cloud) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	klog.V(3).Infof("tencentcloud.InstanceID(\"%s\"): entered\n", string(nodeName))
	node, err := cloud.getInstanceByInstancePrivateIp(ctx, string(nodeName))
	if err != nil {
		klog.Warningf("tencentcloud.InstanceID: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.InstanceID: return: '', %v\n", err)
		return "", err
	}

	ret := fmt.Sprintf("/%s/%s", *node.Placement.Zone, *node.InstanceId)
	klog.V(3).Infof("tencentcloud.InstanceID: return: %s, nil\n", ret)
	return ret, nil
}

// InstanceType returns the type of the specified instance.
func (cloud *Cloud) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	klog.V(3).Infof("tencentcloud.InstanceType(\"%s\"): entered\n", string(name))
	node, err := cloud.getInstanceByInstancePrivateIp(ctx, string(name))
	if err != nil {
		klog.Warningf("tencentcloud.InstanceType: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.InstanceType: return: '', %v\n", err)
		return "", err
	}

	klog.V(3).Infof("tencentcloud.InstanceType: return: %v, nil\n", *node.InstanceType)
	return *node.InstanceType, nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (cloud *Cloud) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	klog.V(3).Infof("tencentcloud.InstanceTypeByProviderID(\"%s\"): entered\n", providerID)
	node, err := cloud.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		klog.Warningf("tencentcloud.InstanceTypeByProviderID: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.InstanceTypeByProviderID: return: '', %v\n", err)
		return "", err
	}

	klog.V(3).Infof("tencentcloud.InstanceTypeByProviderID: return: %v, nil\n", *node.InstanceType)
	return *node.InstanceType, nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (cloud *Cloud) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudProvider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (cloud *Cloud) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(""), cloudProvider.NotImplemented
}

// InstanceExistsByProviderID returns true if the instance for the given provider id still is running.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
func (cloud *Cloud) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.V(3).Infof("tencentcloud.InstanceExistsByProviderID(\"%s\"): entered\n", providerID)
	_, err := cloud.getInstanceByProviderID(ctx, providerID)
	if err == cloudProvider.InstanceNotFound {
		klog.Warningf("tencentcloud.InstanceExistsByProviderID: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.InstanceExistsByProviderID: return: false, %v\n", err)
		return false, err
	}

	klog.V(3).Infof("tencentcloud.InstanceExistsByProviderID: return: true, %v\n", err)
	return true, err
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (cloud *Cloud) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.V(3).Infof("tencentcloud.InstanceShutdownByProviderID(\"%s\"): entered\n", providerID)
	instance, err := cloud.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		klog.Warningf("tencentcloud.InstanceShutdownByProviderID: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.InstanceShutdownByProviderID: return: false, %v\n", err)
		return false, err
	}
	if *instance.InstanceState != "RUNNING" {
		klog.V(3).Infof("tencentcloud.InstanceShutdownByProviderID: return: true, %v\n", err)
		return true, err
	}

	klog.V(3).Infof("tencentcloud.InstanceShutdownByProviderID: return: false, %v\n", err)
	return false, err
}

// getInstanceByInstancePrivateIp returns Tencent Cloud Instance for private ip
func (cloud *Cloud) getInstanceByInstancePrivateIp(ctx context.Context, privateIp string) (*cvm.Instance, error) {
	klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIp(\"%s\"): entered\n", privateIp)

	cacheKey := cacheNamePreVmIp + privateIp
	cacheValue, exist := cloud.cache.Get(cacheKey)
	if exist {
		klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIp: cache return(ip:%s):  %T, nil\n", privateIp, cacheValue.(*cvm.Instance))
		return cacheValue.(*cvm.Instance), nil
	}

	ips := []string{privateIp}
	instances, err := cloud.getInstanceByInstancePrivateIps(ctx, ips)
	if err != nil {
		klog.Warningf("tencentcloud.getInstanceByInstancePrivateIp: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIp: return: nil, %v\n", err)
		return nil, err
	}
	for _, instance := range instances {
		for _, ip := range instance.PrivateIpAddresses {
			if *ip == privateIp {
				klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIp: return(ip:%s): %T, nil\n", privateIp, *instance)
				return instance, nil
			}
		}
	}

	klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIp: return: nil, %v\n", cloudProvider.InstanceNotFound)
	return nil, cloudProvider.InstanceNotFound
}

// getInstanceByInstancePrivateIps returns Tencent Cloud Instance for multi private ip
func (cloud *Cloud) getInstanceByInstancePrivateIps(ctx context.Context, privateIps []string) ([]*cvm.Instance, error) {
	klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps(\"%v\"): entered\n", privateIps)

	ips := make([]string, 0)
	instances := make([]*cvm.Instance, 0)
	for _, ip := range privateIps {
		cacheKey := cacheNamePreVmIp + ip
		cacheValue, exist := cloud.cache.Get(cacheKey)
		if exist {
			klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps: cache exist(ip:%s):  %T\n", ip, cacheValue.(*cvm.Instance))
			instances = append(instances, cacheValue.(*cvm.Instance))
			continue
		}
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps: ips: %v\n", ips)

	count := len(ips)
	requestIps := make([]string, 0)
	for i := 0; i < count; i++ {
		requestIps = append(requestIps, ips[i])
		//klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps: ips: %v\n", ips)
		//klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps: requestIps: %v, %d\n", requestIps, i)
		if (i > 0 && (i+1)%5 == 0) || i == count-1 {
			request := cvm.NewDescribeInstancesRequest()
			request.Filters = []*cvm.Filter{
				{
					Values: common.StringPtrs(requestIps),
					Name:   common.StringPtr("private-ip-address"),
				},
			}

			response, err := cloud.cvm.DescribeInstances(request)
			if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
				klog.Warningf("tencentcloud.getInstanceByInstancePrivateIps: tencentcloud API error: %v, requestIps: %v\n", err, requestIps)
				klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps: return: nil, %v\n", err)
				return nil, err
			}
			if err != nil {
				klog.Warningf("tencentcloud.getInstanceByInstancePrivateIps: Get error: %v, requestIps: %v\n", err, requestIps)
				klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps: return: nil, %v\n", err)
				return nil, err
			}
			for _, instance := range response.Response.InstanceSet {
				if *instance.VirtualPrivateCloud.VpcId != cloud.txConfig.VpcId {
					continue
				}
				for _, privateIp := range instance.PrivateIpAddresses {
					if isExist(*privateIp, ips) {
						klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps: get instance from tencentcloud API(ip:%s): %T\n", *privateIp, *instance)
						cacheKey := cacheNamePreVmIp + *privateIp
						cloud.cache.Set(cacheKey, instance)
						instances = append(instances, instance)
						continue
					}
				}
			}
			requestIps = nil
		}
	}

	klog.V(3).Infof("tencentcloud.getInstanceByInstancePrivateIps: return: instances count %d, nil\n", len(instances))
	return instances, nil
}

// getInstanceByInstanceID returns Tencent Cloud Instance for instanceID
func (cloud *Cloud) getInstanceByInstanceID(ctx context.Context, instanceID string) (*cvm.Instance, error) {
	klog.V(3).Infof("tencentcloud.getInstanceByInstanceID(\"%s\"): entered\n", instanceID)

	cacheKey := cacheNamePreVmID + instanceID
	cacheValue, exist := cloud.cache.Get(cacheKey)
	if exist {
		klog.V(3).Infof("tencentcloud.getInstanceByInstanceID: cache return(instanceID:%s):  %T, nil\n", instanceID, cacheValue.(*cvm.Instance))
		return cacheValue.(*cvm.Instance), nil
	}

	request := cvm.NewDescribeInstancesRequest()
	request.Filters = []*cvm.Filter{
		{
			Values: common.StringPtrs([]string{instanceID}),
			Name:   common.StringPtr("instance-id"),
		},
	}

	response, err := cloud.cvm.DescribeInstances(request)
	if _, ok := err.(*cloudErrors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.getInstanceByInstanceID: tencentcloud API error: %v\n", err)
		klog.V(3).Infof("tencentcloud.getInstanceByInstanceID: return: nil, %v\n", err)
		return nil, err
	}
	if err != nil {
		klog.Warningf("tencentcloud.getInstanceByInstanceID: Get error: %v\n", err)
		klog.V(3).Infof("tencentcloud.getInstanceByInstanceID: return: nil, %v\n", err)
		return nil, err
	}
	for _, instance := range response.Response.InstanceSet {
		if *instance.VirtualPrivateCloud.VpcId != cloud.txConfig.VpcId {
			continue
		}
		if instanceID == *instance.InstanceId {
			cloud.cache.Set(cacheKey, instance)
			klog.V(3).Infof("tencentcloud.getInstanceByInstanceID: return(instanceID:%s):  %T, nil\n", instanceID, *instance)
			return instance, nil
		}
	}

	klog.V(3).Infof("tencentcloud.getInstanceByInstanceID: return:  nil, %v\n", cloudProvider.InstanceNotFound)
	return nil, cloudProvider.InstanceNotFound
}

// getInstanceIdByProviderID returns the addresses of the specified instance.
func (cloud *Cloud) getInstanceByProviderID(ctx context.Context, providerID string) (*cvm.Instance, error) {
	klog.V(3).Infof("tencentcloud.getInstanceByProviderID(\"%s\"): entered\n", providerID)
	id := strings.TrimPrefix(providerID, fmt.Sprintf("%s://", providerName))
	parts := strings.Split(id, "/")
	if len(parts) == 3 {
		instance, err := cloud.getInstanceByInstanceID(ctx, parts[2])
		if err != nil {
			klog.V(3).Infof("tencentcloud.getInstanceByProviderID: return:  nil, %v\n", err)
			return nil, err
		}
		klog.V(3).Infof("tencentcloud.getInstanceByProviderID: return: %v, nil\n", err)
		return instance, nil
	}

	err := errors.New(fmt.Sprintf("invalid format for providerId %s", providerID))
	klog.V(3).Infof("tencentcloud.getInstanceByProviderID: return:  nil, %v\n", err)
	return nil, err
}
