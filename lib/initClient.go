package lib

import (
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)


type Clients struct {
	K8sClient *kubernetes.Clientset
	MetricsClient *metrics.Clientset
}

func NewClient() *Clients {
	//生成config配置
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	CheckError(err)

	//metrics-client
	metricsClient, err := metrics.NewForConfig(config)
	CheckError(err)


	//common-client
	commonClient, err := kubernetes.NewForConfig(config)
	CheckError(err)

	return &Clients {
		K8sClient: commonClient,
		MetricsClient: metricsClient,
	}
}

func CheckError(err error) {
	if err != nil {
		log.Fatalln(err.Error())
	}
}
