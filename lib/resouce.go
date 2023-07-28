package lib

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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
	ctx     context.Context
)

func init() {
	clients = NewClient() // 初始化两个客户端
}

func Validate(namespace string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespaces, err := clients.K8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("List namespaces timed out:", err.Error())
			os.Exit(1)
		}
		log.Fatalln("Error listing namespaces:", err.Error())
		os.Exit(1)
	}

	for _, ns := range namespaces.Items {
		if ns.Name == namespace {
			return true
		}
	}
	return false
}

func ParserCommonResouce(namespace string) ClientsetMap {
	ClientsetMap := make(map[string]ResourceStruct, 0)
	podList, err := clients.K8sClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		log.Fatalf("Failed to list pods from the namespace %s,error is %s\n", namespace, err.Error())
	}

	for _, pod := range podList.Items {
		cpuTotalLimits, cpuTotalRequests, memTotalRequest, memTotalLimits = 0, 0, 0, 0
		// 遍历容器中的资源值，不包括initContainers
		for i := range pod.Spec.Containers {
			cpuTotalRequests += pod.Spec.Containers[i].Resources.Requests.Cpu().MilliValue()
			cpuTotalLimits += pod.Spec.Containers[i].Resources.Limits.Cpu().MilliValue()
			memTotalRequest += pod.Spec.Containers[i].Resources.Requests.Memory().Value() / 1024 / 1024
			memTotalLimits += pod.Spec.Containers[i].Resources.Limits.Memory().Value() / 1024 / 1024
		}
		podResource := ResourceStruct{
			PodName:          pod.Name,
			NodeName:         pod.Spec.NodeName,
			Namespace:        namespace,
			CPUTotalRequests: strconv.FormatInt(cpuTotalRequests, 10),
			CPUTotalLimits:   strconv.FormatInt(cpuTotalLimits, 10),
			MemTotalRequest:  strconv.FormatInt(memTotalRequest, 10),
			MemTotalLimits:   strconv.FormatInt(memTotalLimits, 10),
		}

		key := encode(fmt.Sprintf("%s/%s", namespace, pod.Name))
		ClientsetMap[key] = podResource
	}
	return ClientsetMap
}

func ParserMetricsResouce(namespace string) MetricsMap {
	MetricsMap := make(map[string]MetricsStruct, 0)
	PodMetricsList, err := clients.MetricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.Background(), metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		log.Fatalf("不能从名称空间 %s 获取指标数据,确认已部署metrics-server", namespace)
	}

	for _, podMetrics := range PodMetricsList.Items {
		podCPUUsage, podMemUsage = 0, 0
		for i := 0; i < len(podMetrics.Containers); i++ {
			podCPUUsage += podMetrics.Containers[i].Usage.Cpu().MilliValue()
			podMemUsage += podMetrics.Containers[i].Usage.Memory().Value() / 1024 / 1024 // 内存以M为单位计算
		}
		podMetric := MetricsStruct{
			PodName:   podMetrics.Name,
			Namespace: podMetrics.Namespace,
			CPUUsage:  strconv.FormatInt(podCPUUsage, 10),
			MemUsage:  strconv.FormatInt(podMemUsage, 10),
		}

		key := encode(fmt.Sprintf("%s/%s", namespace, podMetrics.Name))
		MetricsMap[key] = podMetric
	}
	return MetricsMap
}

func encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
