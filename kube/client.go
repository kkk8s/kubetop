package kube

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	k8sClient     *kubernetes.Clientset
	metricsClient *metrics.Clientset
)

func init() {
	//生成config配置
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	HandlerError(err, "构建kubeconfig配置文件失败")

	//metrics-client
	metricsClient, err = metrics.NewForConfig(config)
	HandlerError(err,"构建metrics客户端失败")

	//common-client
	k8sClient, err = kubernetes.NewForConfig(config)
	HandlerError(err,"构建rest客户端失败")
}

func GetK8sClient() *kubernetes.Clientset {
	return k8sClient
}

func GetMetricsClient() *metrics.Clientset {
	return metricsClient
}
