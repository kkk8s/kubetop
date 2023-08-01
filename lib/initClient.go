package lib

import (
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	k8sClient *kubernetes.Clientset
	metricsClient *metrics.Clientset
)

func init() {
	//生成config配置
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	CheckError(err)

	//metrics-client
	metricsClient, err = metrics.NewForConfig(config)
	CheckError(err)


	//common-client
	k8sClient, err = kubernetes.NewForConfig(config)
	CheckError(err)
}

func GetK8sClient() *kubernetes.Clientset {
	return k8sClient
}

func GetMetricsClient() *metrics.Clientset {
	return metricsClient
}

func CheckError(err error) {
	if err != nil {
		log.Fatalln(err.Error())
	}
}
