package tencentcloud

import (
	"context"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	tke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"
	"k8s.io/apimachinery/pkg/types"
	cloudProvider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

// ListRoutes lists all managed routes that belong to the specified clusterName
func (cloud *Cloud) ListRoutes(ctx context.Context, clusterName string) ([]*cloudProvider.Route, error) {
	klog.V(3).Infof("tencentcloud.ListRoutes(\"%s\"): entered\n", clusterName)
	request := tke.NewDescribeClusterRoutesRequest()
	request.RouteTableName = common.StringPtr(cloud.txConfig.ClusterRouteTable)

	cloudRoutes, err := cloud.tke.DescribeClusterRoutes(request)
	if _, ok := err.(*errors.TencentCloudSDKError); ok {
		klog.Warningf("tencentcloud.ListRoutes: tencentcloud API error: %s\n", err)
		klog.V(3).Infof("tencentcloud.ListRoutes: return: {}, %v\n", err)
		return []*cloudProvider.Route{}, err
	}

	if err != nil {
		klog.Warningf("tencentcloud.ListRoutes: Get error: %s\n", err)
		klog.V(3).Infof("tencentcloud.ListRoutes: return: {}, %v\n", err)
		return []*cloudProvider.Route{}, err
	}

	routes := make([]*cloudProvider.Route, len(cloudRoutes.Response.RouteSet))
	for idx, route := range cloudRoutes.Response.RouteSet {
		routes[idx] = &cloudProvider.Route{Name: *route.GatewayIp, TargetNode: types.NodeName(*route.GatewayIp), DestinationCIDR: *route.DestinationCidrBlock}
	}

	klog.V(3).Infof("tencentcloud.ListRoutes: return: %T, nil\n", routes)
	return routes, nil
}

// CreateRoute creates the described managed route
// route.Name will be ignored, although the cloud-provider may use nameHint
// to create a more user-meaningful name.
func (cloud *Cloud) CreateRoute(ctx context.Context, clusterName string, nameHint string, route *cloudProvider.Route) error {
	klog.V(3).Infof("tencentcloud.CreateRoute(\"%s, %s, %T\"): entered\n", clusterName, nameHint, route)
	request := tke.NewCreateClusterRouteRequest()
	request.RouteTableName = common.StringPtr(cloud.txConfig.ClusterRouteTable)
	request.GatewayIp = common.StringPtr(string(route.TargetNode))
	request.DestinationCidrBlock = common.StringPtr(route.DestinationCIDR)

	_, err := cloud.tke.CreateClusterRoute(request)
	if err != nil {
		klog.Warningf("tencentcloud.CreateRoute: Get error: %s\n", err)
	}

	klog.V(3).Infof("tencentcloud.CreateRoute: return: %v\n", err)
	return err
}

// DeleteRoute deletes the specified managed route
// Route should be as returned by ListRoutes
func (cloud *Cloud) DeleteRoute(ctx context.Context, clusterName string, route *cloudProvider.Route) error {
	klog.V(3).Infof("tencentcloud.DeleteRoute(\"%s, %T\"): entered\n", clusterName, route)
	request := tke.NewDeleteClusterRouteRequest()
	request.RouteTableName = common.StringPtr(cloud.txConfig.ClusterRouteTable)
	request.GatewayIp = common.StringPtr(string(route.TargetNode))
	request.DestinationCidrBlock = common.StringPtr(route.DestinationCIDR)

	_, err := cloud.tke.DeleteClusterRoute(request)
	if err != nil {
		klog.Warningf("tencentcloud.DeleteRoute: Get error: %s\n", err)
	}

	klog.V(3).Infof("tencentcloud.DeleteRoute: return: %v\n", err)
	return err
}
