module github.com/farmerluo/cloud-provider-tencent

go 1.15

require (
	github.com/golang/mock v1.4.1 // indirect
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go v1.0.105
	k8s.io/api v0.18.16
	k8s.io/apimachinery v0.18.16
	k8s.io/apiserver v0.18.16
	k8s.io/client-go v0.18.16
	k8s.io/cloud-provider v0.18.16
	k8s.io/component-base v0.18.16
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.18.16
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920 // indirect
)

replace (
	k8s.io/api v0.0.0 => k8s.io/api v0.18.16
	k8s.io/apiextensions-apiserver v0.0.0 => k8s.io/apiextensions-apiserver v0.18.16
	k8s.io/apimachinery v0.0.0 => k8s.io/apimachinery v0.18.16
	k8s.io/apiserver v0.0.0 => k8s.io/apiserver v0.18.16
	k8s.io/cli-runtime v0.0.0 => k8s.io/cli-runtime v0.18.16
	k8s.io/client-go v0.0.0 => k8s.io/client-go v0.18.16
	k8s.io/cloud-provider v0.0.0 => k8s.io/cloud-provider v0.18.16
	k8s.io/cluster-bootstrap v0.0.0 => k8s.io/cluster-bootstrap v0.18.16
	k8s.io/code-generator v0.0.0 => k8s.io/code-generator v0.18.16
	k8s.io/component-base v0.0.0 => k8s.io/component-base v0.18.16
	k8s.io/component-helpers v0.0.0 => k8s.io/component-helpers v0.18.0-alpha.5
	k8s.io/controller-manager v0.0.0 => k8s.io/controller-manager v0.18.0-alpha.5
	k8s.io/cri-api v0.0.0 => k8s.io/cri-api v0.18.16
	k8s.io/csi-translation-lib v0.0.0 => k8s.io/csi-translation-lib v0.18.16
	k8s.io/kube-aggregator v0.0.0 => k8s.io/kube-aggregator v0.18.16
	k8s.io/kube-controller-manager v0.0.0 => k8s.io/kube-controller-manager v0.18.16
	k8s.io/kube-proxy v0.0.0 => k8s.io/kube-proxy v0.18.16
	k8s.io/kube-scheduler v0.0.0 => k8s.io/kube-scheduler v0.18.16
	k8s.io/kubectl v0.0.0 => k8s.io/kubectl v0.18.16
	k8s.io/kubelet v0.0.0 => k8s.io/kubelet v0.18.16
	k8s.io/legacy-cloud-providers v0.0.0 => k8s.io/legacy-cloud-providers v0.18.16
	k8s.io/metrics v0.0.0 => k8s.io/metrics v0.18.16
	k8s.io/mount-utils v0.0.0 => k8s.io/mount-utils v0.18.0-alpha.5
	k8s.io/sample-apiserver v0.0.0 => k8s.io/sample-apiserver v0.18.16
)
