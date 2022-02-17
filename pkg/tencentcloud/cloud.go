package tencentcloud

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	clb "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/clb/v20180317"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	tke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"
	"github.com/weimob-tech/cloud-provider-tencent/pkg/cache"
	cloudProvider "k8s.io/cloud-provider"
	"k8s.io/klog"

	"k8s.io/client-go/kubernetes"
)

const (
	providerName = "tencentcloud"
	TTLTime      = 60 * time.Second
)

type TxCloudConfig struct {
	Region            string `json:"region"`
	VpcId             string `json:"vpc_id"`
	CLBNamePrefix     string `json:"clb_name_prefix"`
	TagKey            string `json:"tag_key"`
	SecretId          string `json:"secret_id"`
	SecretKey         string `json:"secret_key"`
	ClusterRouteTable string `json:"cluster_route_table"`
}

type Cloud struct {
	txConfig   TxCloudConfig
	kubeClient kubernetes.Interface
	cvm        *cvm.Client
	tke        *tke.Client
	clb        *clb.Client
	cache      *cache.TTLCache
}

//NewCloud Cloud constructed function
func NewCloud(config io.Reader) (*Cloud, error) {
	var c TxCloudConfig
	if config != nil {
		cfg, err := ioutil.ReadAll(config)
		if err != nil {
			klog.V(3).Infof("tencentcloud.NewCloud: return: nil, %v\n", err)
			return nil, err
		}
		if err := json.Unmarshal(cfg, &c); err != nil {
			return nil, err
		}
	}

	if c.Region == "" {
		c.Region = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION")
	}
	if c.VpcId == "" {
		c.VpcId = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID")
	}
	if c.CLBNamePrefix == "" {
		c.CLBNamePrefix = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_NAME_PREFIX")
	}
	if c.TagKey == "" {
		c.TagKey = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_TAG_KEY")
	}
	if c.SecretId == "" {
		c.SecretId = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID")
	}
	if c.SecretKey == "" {
		c.SecretKey = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY")
	}
	if c.ClusterRouteTable == "" {
		c.ClusterRouteTable = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ROUTE_TABLE")
	}

	if err := checkConfig(c); err != nil {
		klog.V(3).Infof("tencentcloud.NewCloud: return: nil, %v\n", err)
		return nil, err
	}

	return &Cloud{txConfig: c}, nil
}

// checkConfig check cloud config
func checkConfig(c TxCloudConfig) error {
	if strings.TrimSpace(c.Region) == "" {
		klog.Error("tencentcloud.checkConfig: 'Region' config is null\n")
		return errors.New("'Region' config is null")
	}
	if strings.TrimSpace(c.VpcId) == "" {
		klog.Error("tencentcloud.checkConfig: 'VpcId' config is null\n")
		return errors.New("'VpcId' config is null")
	}
	if strings.TrimSpace(c.TagKey) == "" {
		klog.Error("tencentcloud.checkConfig: 'TagKey' config is null\n")
		return errors.New("'TagKey' config is null")
	}
	if strings.TrimSpace(c.CLBNamePrefix) == "" {
		klog.Error("tencentcloud.checkConfig: 'CLBNamePrefix' config is null\n")
		return errors.New("'CLBNamePrefix' config is null")
	}
	if strings.TrimSpace(c.SecretId) == "" {
		klog.Error("tencentcloud.checkConfig: 'SecretId' config is null\n")
		return errors.New("'SecretId' config is null")
	}
	if strings.TrimSpace(c.SecretKey) == "" {
		klog.Error("tencentcloud.checkConfig: 'SecretKey' config is null\n")
		return errors.New("'SecretKey' config is null")
	}
	if strings.TrimSpace(c.ClusterRouteTable) == "" {
		klog.Error("tencentcloud.checkConfig: 'ClusterRouteTable' config is null\n")
		return errors.New("'ClusterRouteTable' config is null")
	}
	return nil
}

// init Initialize cloudProvider
func init() {
	cloudProvider.RegisterCloudProvider(providerName,
		func(config io.Reader) (cloudProvider.Interface, error) {
			return NewCloud(config)
		})
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (cloud *Cloud) Initialize(clientBuilder cloudProvider.ControllerClientBuilder, stop <-chan struct{}) {
	cloud.kubeClient = clientBuilder.ClientOrDie("tencentcloud-cloud-provider")
	credential := common.NewCredential(
		//os.Getenv("TENCENTCLOUD_SECRET_ID"),
		//os.Getenv("TENCENTCLOUD_SECRET_KEY"),
		cloud.txConfig.SecretId,
		cloud.txConfig.SecretKey,
	)
	// 非必要步骤
	// 实例化一个客户端配置对象，可以指定超时时间等配置
	cpf := profile.NewClientProfile()
	// SDK有默认的超时时间，非必要请不要进行调整。
	// 如有需要请在代码中查阅以获取最新的默认值。
	cpf.HttpProfile.ReqTimeout = 10
	cvmClient, err := cvm.NewClient(credential, cloud.txConfig.Region, cpf)
	if err != nil {
		klog.Warningf("tencentcloud.Initialize().cvm.NewClient An tencentcloud API error has returned, message=[%v])\n", err)
	}
	cloud.cvm = cvmClient

	tkeClient, err := tke.NewClient(credential, cloud.txConfig.Region, cpf)
	if err != nil {
		klog.Warningf("tencentcloud.Initialize().tke.NewClient An tencentcloud API error has returned, message=[%v])\n", err)
	}
	cloud.tke = tkeClient

	clbClient, err := clb.NewClient(credential, cloud.txConfig.Region, cpf)
	if err != nil {
		klog.Warningf("tencentcloud.Initialize().clb.NewClient An tencentcloud API error has returned, message=[%v])\n", err)
	}
	cloud.clb = clbClient

	cloud.cache = cache.NewTTLCache(TTLTime)
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (cloud *Cloud) LoadBalancer() (cloudProvider.LoadBalancer, bool) {
	return cloud, true
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (cloud *Cloud) Instances() (cloudProvider.Instances, bool) {
	return cloud, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (cloud *Cloud) Zones() (cloudProvider.Zones, bool) {
	return nil, false
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (cloud *Cloud) Clusters() (cloudProvider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (cloud *Cloud) Routes() (cloudProvider.Routes, bool) {
	return cloud, true
}

// ProviderName returns the cloud provider ID.
func (cloud *Cloud) ProviderName() string {
	return providerName
}

// HasClusterID returns true if a ClusterID is required and set
func (cloud *Cloud) HasClusterID() bool {
	return false
}
