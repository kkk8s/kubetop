package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"metrics.k8s.io/lib"
)

// 定义单个pod要采集的资源
type podResource struct {
	NodeName    string // Pod所在的节点
	Namespace   string
	PodName     string
	CPURequests int64
	CPULimits   int64
	MemRequest  int64
	MemLimits   int64
}

// 定义单个pod的指标信息
type podMetric struct {
	Namespace string
	PodName   string
	CPUUsage  int64
	MemUsage  int64
}

type podInfo struct {
	podResource
	podMetric
	CPUUsagePercentage float64
	MemUsagePercentage float64
}

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
		Results(namespace)
	},
	Args: cobra.NoArgs,
	Aliases: []string{"po", "pods"},
}

func ParserCommonResouce(namespace string) map[string]podResource {
	PodResources := make(map[string]podResource, 0)
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
		podResource := podResource{
			PodName:     pod.Name,
			NodeName:    pod.Spec.NodeName,
			Namespace:   namespace,
			CPURequests: cpuRequests,
			CPULimits:   cpuLimits,
			MemRequest:  memRequest,
			MemLimits:   memLimits,
		}

		ns_pod := encode(fmt.Sprintf("%s/%s", namespace, pod.Name))
		PodResources[ns_pod] = podResource
	}
	return PodResources
}

func ParserMetricsResouce(namespace string) map[string]podMetric {
	PodMetrics := make(map[string]podMetric, 0)
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
		podMetric := podMetric{
			PodName:   podMetrics.Name,
			Namespace: podMetrics.Namespace,
			CPUUsage:  podCPUUsage,
			MemUsage:  podMemUsage,
		}

		ns_pod := encode(fmt.Sprintf("%s/%s", namespace, podMetrics.Name))
		PodMetrics[ns_pod] = podMetric
	}
	return PodMetrics
}

func Results(namespace string) {
	defer cancel()
	if !Validate(namespace) {
		fmt.Println("指定的命名空间不存在")
		os.Exit(1)
	}
	PodsResource := ParserCommonResouce(namespace)
	PodsMetric := ParserMetricsResouce(namespace)

	var podInfoList []podInfo
	for key, pr := range PodsResource {
		if pu, ok := PodsMetric[key]; ok {
			result := podInfo{
				podResource: podResource{
					NodeName:    pr.NodeName,
					PodName:     pr.PodName,
					CPURequests: pr.CPURequests,
					CPULimits:   pr.CPULimits,
					MemRequest:  pr.MemRequest,
					MemLimits:   pr.MemLimits,
				},
				podMetric: podMetric{
					CPUUsage: pu.CPUUsage,
					MemUsage: pu.MemUsage,
				},
				CPUUsagePercentage: calculatePodUsagePercentage(pr.CPURequests, pu.CPUUsage),
				MemUsagePercentage: calculatePodUsagePercentage(pr.MemRequest, pu.MemUsage),
			}
			podInfoList = append(podInfoList, result)
		}
	}

	// 根据用户传入的选项进行排序
	switch sortBy {
	case "cpu":
		sort.Slice(podInfoList, func(i, j int) bool {
			return podInfoList[i].CPUUsagePercentage < podInfoList[j].CPUUsagePercentage
		})
	case "mem":
		sort.Slice(podInfoList, func(i, j int) bool {
			return podInfoList[i].MemUsagePercentage < podInfoList[j].MemUsagePercentage
		})
	default:
		fmt.Println("未知的排序选项")
		return
	}

	podResults := make([][]string, 1024)
	// 输出结果
	for _, podInfo := range podInfoList {
		result := []string{
			podInfo.NodeName,
			podInfo.podResource.NodeName,
			formatValue(podInfo.CPURequests),
			formatValue(podInfo.CPUUsage),
			formatValue(podInfo.CPUUsagePercentage),
			formatValue(podInfo.CPULimits),
			formatValue(podInfo.MemRequest),
			formatValue(podInfo.MemUsage),
			formatValue(podInfo.MemUsagePercentage),
			formatValue(podInfo.MemLimits),
		}
		podResults = append(podResults, result)
	}

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"运行节点", "pod名称", "cpu-request(m)", "cpu用量(m)", "cpu用量/request占比", "cpu-limits(m)", "内存-request(MiB)", "内存用量(MiB)", "内存用量/request占比", "内存-limits(MiB)"})
	table.AppendBulk(podResults)
	table.Render()
}

func encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

// int/float64 -> string，无数据时输出-
func formatValue(val interface{}) string {
	switch v := val.(type) {
	case int64:
		if v == 0 {
			return "-"
		}
		return strconv.FormatInt(v, 10)
	case float64:
		if v == 0 {
			return "-"
		}
		return strconv.FormatFloat(v, 'f', 2, 64) + "%"
	default:
		return fmt.Sprintf("Unsupported type: %v", reflect.TypeOf(val))
	}
}

func calculatePodUsagePercentage(request, usage int64) float64 {
	if request == 0 {
		return 0
	}

	percentage := float64(usage) / float64(request) * 100
	return percentage
}


// 判断是否存在指定的namespace
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