package lib

import (
	"context"
	"log"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	cpuTotalRequests int64
	cpuTotalLimits   int64
	memTotalRequest  int64
	memTotalLimits   int64

	podCPUUsage int64
	podMemUsage int64

	clients *Clients
)

func init() {
	clients = NewClient() // 初始化两个客户端
}

func ParserCommonResouce(namespace string) []ResourceStruct {
	podLists, err := clients.K8sClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		log.Fatalf("Failed to list pods from the namespace %s.\n", namespace)
	}

	for _, pod := range podLists.Items {
		cpuTotalLimits, cpuTotalRequests, memTotalRequest, memTotalLimits = 0, 0, 0, 0
		containerCount := len(pod.Spec.Containers)
		for i := 0; i < containerCount; i++ {
			cpuTotalRequests += pod.Spec.Containers[i].Resources.Requests.Cpu().MilliValue()
			cpuTotalLimits += pod.Spec.Containers[i].Resources.Limits.Cpu().MilliValue()
			memTotalRequest += pod.Spec.Containers[i].Resources.Requests.Memory().Value() / 1024 / 1024
			memTotalLimits += pod.Spec.Containers[i].Resources.Limits.Memory().Value() / 1024 / 1024
		}
		podResource := ResourceStruct{}
		podResource.PodName = pod.Name
		podResource.NodeName = pod.Spec.NodeName
		podResource.Namespace = namespace
		podResource.CPUTotalRequests = strconv.FormatInt(cpuTotalRequests, 10)
		podResource.CPUTotalLimits = strconv.FormatInt(cpuTotalLimits, 10)
		podResource.MemTotalRequest = strconv.FormatInt(memTotalRequest, 10)
		podResource.MemTotalLimits = strconv.FormatInt(memTotalLimits, 10)

		PodsResource = append(PodsResource, podResource)
	}
	return PodsResource
}

func ParserMetricsResouce(namespace string) []MetricsStruct {
	PodMetricsList, err := clients.MetricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.Background(), metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		log.Fatalf("不能从名称空间%s获取指标数据,是否已部署metrics-server", namespace)
	}
	
	for _, podMetrics := range PodMetricsList.Items {
		for i := 0; i < len(podMetrics.Containers); i++ {
			podCPUUsage += podMetrics.Containers[i].Usage.Cpu().MilliValue()
			podMemUsage += podMetrics.Containers[i].Usage.Memory().Value() / 1024 / 1024 // 内存以M为单位计算
		}
		podMetric := MetricsStruct{}
		podMetric.PodName = podMetrics.Name
		podMetric.Namespace = podMetrics.Namespace
		podMetric.CPUUsage = strconv.FormatInt(podCPUUsage, 10)
		podMetric.MemUsage = strconv.FormatInt(podMemUsage, 10)
		PodsMetric = append(PodsMetric, podMetric)
		podCPUUsage, podMemUsage = 0, 0
	}
	return PodsMetric
}
