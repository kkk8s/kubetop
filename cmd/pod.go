package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"metrics.k8s.io/lib"
)

// 定义单个pod要采集的资源
type ResourceStruct struct {
	NodeName    string // Pod所在的节点
	Namespace   string
	PodName     string
	CPURequests string
	CPULimits   string
	MemRequest  string
	MemLimits   string
}

// 定义单个pod要采集的指标
type MetricsStruct struct {
	Namespace string
	PodName   string
	CPUUsage  string
	MemUsage  string
}

type (
	ClientsetMap map[string]ResourceStruct
	MetricsMap   map[string]MetricsStruct
)

var (
	cpuRequests int64
	cpuLimits   int64
	memRequest  int64
	memLimits   int64

	podCPUUsage int64
	podMemUsage int64

	ctx    context.Context
	cancel context.CancelFunc

	timeout time.Duration
)

var podCmd = &cobra.Command{
	Use:   "pod",
	Short: "Print the usage of pod in namespace",
	Run: func(cmd *cobra.Command, args []string) {
		namespace, _ := cmd.Flags().GetString("namespace")
		Results(namespace)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		namespace, _ := cmd.Flags().GetString("namespace")
		if namespace == "" {
			return fmt.Errorf("namespace is required, use -n or --namespace to specify the namespace")
		}
		return nil
	},
	Aliases: []string{"po", "pods"},
}

func Validate(namespace string) bool {
	namespaces, err := lib.GetK8sClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
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
	podList, err := lib.GetK8sClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		log.Fatalf("Failed to list pods from the namespace %s,error is %s\n", namespace, err.Error())
	}

	for _, pod := range podList.Items {
		cpuLimits, cpuRequests, memRequest, memLimits = 0, 0, 0, 0
		// 遍历容器中的资源值，不包括initContainers
		for i := range pod.Spec.Containers {
			cpuRequests += pod.Spec.Containers[i].Resources.Requests.Cpu().MilliValue()
			cpuLimits += pod.Spec.Containers[i].Resources.Limits.Cpu().MilliValue()
			memRequest += pod.Spec.Containers[i].Resources.Requests.Memory().Value() / 1024 / 1024
			memLimits += pod.Spec.Containers[i].Resources.Limits.Memory().Value() / 1024 / 1024
		}
		podResource := ResourceStruct{
			PodName:     pod.Name,
			NodeName:    pod.Spec.NodeName,
			Namespace:   namespace,
			CPURequests: formatValue(cpuRequests),
			CPULimits:   formatValue(cpuLimits),
			MemRequest:  formatValue(memRequest),
			MemLimits:   formatValue(memLimits),
		}

		key := encode(fmt.Sprintf("%s/%s", namespace, pod.Name))
		ClientsetMap[key] = podResource
	}
	return ClientsetMap
}

func ParserMetricsResouce(namespace string) MetricsMap {
	MetricsMap := make(map[string]MetricsStruct, 0)
	PodMetricsList, err := lib.GetMetricsClient().MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		log.Fatalf("不能从名称空间 %s 获取指标数据,确认已部署metrics-server", namespace)
	}

	for _, podMetrics := range PodMetricsList.Items {
		podCPUUsage, podMemUsage = 0, 0
		for i := range podMetrics.Containers {
			podCPUUsage += podMetrics.Containers[i].Usage.Cpu().MilliValue()
			podMemUsage += podMetrics.Containers[i].Usage.Memory().Value() / 1024 / 1024 // 内存以M为单位计算
		}
		podMetric := MetricsStruct{
			PodName:   podMetrics.Name,
			Namespace: podMetrics.Namespace,
			CPUUsage:  formatValue(podCPUUsage),
			MemUsage:  formatValue(podMemUsage),
		}

		key := encode(fmt.Sprintf("%s/%s", namespace, podMetrics.Name))
		MetricsMap[key] = podMetric
	}
	return MetricsMap
}

func encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func Results(namespace string) {
	defer cancel()
	if !Validate(namespace) {
		log.Fatalln("No such namespace")
	}
	PodsResource := ParserCommonResouce(namespace)
	PodsMetric := ParserMetricsResouce(namespace)

	results := make([][]string, 2048)
	for key, podResource := range PodsResource {
		if podUsage, ok := PodsMetric[key]; ok {
			result := []string{podResource.NodeName, podResource.PodName, podResource.CPURequests, podUsage.CPUUsage, podResource.CPULimits, podResource.MemRequest, podUsage.MemUsage, podResource.MemLimits}
			results = append(results, result)
		}
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"运行节点", "pod名称", "cpu申请(m)", "cpu实际用量(m)", "cpu限额(m)", "内存申请(MiB)", "内存实际用量(MiB)", "内存限额(MiB)"})
	table.AppendBulk(results)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.Render()
}

func formatValue(val int64) string {
	if val == 0 {
		return "-"
	}
	return strconv.FormatInt(val, 10)
}
