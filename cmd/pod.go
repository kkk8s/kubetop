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
	"strings"
	"sync"
	"time"

	"metrics.k8s.io/lib"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type ContainerResource struct {
	Name        string
	CPURequests int64
	CPULimits   int64
	MemRequest  int64
	MemLimits   int64
}

type PodResource struct {
	NodeName   string // Pod所在的节点
	PodName    string
	Containers map[string]*ContainerResource // 容器级别的资源信息
}

type ContainerMetric struct {
	Name     string
	CPUUsage int64
	MemUsage int64
}

type PodMetrics struct {
	PodName    string
	Containers map[string]*ContainerMetric
}

type ContainerRatio struct {
	CPUUsageToRequestRatio float64
	CPUUsageToLimitsRatio  float64
	MemUsageToRequestRatio float64
	MemUsageToLimitsRatio  float64
}

type PodInfo struct {
	PodResource
	PodMetrics
	ContainersRatio        map[string]*ContainerRatio
	CPUUsageToRequestRatio float64
	CPUUsageToLimitsRatio  float64
	MemUsageToRequestRatio float64
	MemUsageToLimitsRatio  float64
}

var (
	ctx               context.Context
	cancel            context.CancelFunc
	timeout           time.Duration
	podResourcesMutex sync.Mutex
	podMetricsMutex   sync.Mutex

	daemonsetPod = []string{"calico-node", "kube-proxy", "nginx-proxy"}
)

var podCmd = &cobra.Command{
	Use:   "pod",
	Short: "Print the usage of pod in namespace",
	Run: func(cmd *cobra.Command, args []string) {
		PrintResult(ctx, namespace)
	},
	Args:    cobra.NoArgs,
	Aliases: []string{"po", "pods"},
}

func LoadK8sResource(ctx context.Context, namespace string) map[string]PodResource {
	PodResources := make(map[string]PodResource, 0)
	podList, err := lib.GetK8sClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		log.Fatalf("Failed to list pods from the namespace %s, error: %s\n", namespace, err.Error())
	}

	var wg sync.WaitGroup
	wg.Add(len(podList.Items))

	for _, pod := range podList.Items {
		go func(pod corev1.Pod) {
			defer wg.Done()

			containersResource := make(map[string]*ContainerResource)

			// Iterate over containers in the pod
			for i := range pod.Spec.Containers {
				container := pod.Spec.Containers[i]
				containerResource := &ContainerResource{
					Name:        container.Name,
					CPURequests: container.Resources.Requests.Cpu().MilliValue(),
					CPULimits:   container.Resources.Limits.Cpu().MilliValue(),
					MemRequest:  container.Resources.Requests.Memory().Value() / 1024 / 1024,
					MemLimits:   container.Resources.Limits.Memory().Value() / 1024 / 1024,
				}
				containersResource[container.Name] = containerResource
			}

			podResource := PodResource{
				PodName:    pod.Name,
				NodeName:   pod.Spec.NodeName,
				Containers: containersResource,
			}

			podResourcesMutex.Lock()
			PodResources[encode(namespace, podResource.PodName)] = podResource
			podResourcesMutex.Unlock()
		}(pod)
	}

	wg.Wait()
	return PodResources
}

func LoadK8sMetrics(ctx context.Context, namespace string) map[string]PodMetrics {
	PodsMetrics := make(map[string]PodMetrics, 0)
	PodMetricsList, err := lib.GetMetricsClient().MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{ResourceVersion: "0"})
	if err != nil {
		log.Fatalf("Failed to get metrics from namespace %s, make sure metrics-server is deployed", namespace)
	}

	var wg sync.WaitGroup
	wg.Add(len(PodMetricsList.Items))

	for _, podMetric := range PodMetricsList.Items {
		go func(podMetric metricsv1beta1.PodMetrics) {
			defer wg.Done()

			podContainerMetrics := make(map[string]*ContainerMetric)

			for i := range podMetric.Containers {
				containerMetric := podMetric.Containers[i]
				containerMetricInfo := &ContainerMetric{
					Name:     containerMetric.Name,
					CPUUsage: containerMetric.Usage.Cpu().MilliValue(),
					MemUsage: containerMetric.Usage.Memory().Value() / 1024 / 1024,
				}
				podContainerMetrics[containerMetric.Name] = containerMetricInfo
			}

			podMetricInfo := PodMetrics{
				PodName:    podMetric.Name,
				Containers: podContainerMetrics,
			}

			podMetricsMutex.Lock()
			PodsMetrics[encode(namespace, podMetricInfo.PodName)] = podMetricInfo
			podMetricsMutex.Unlock()
		}(podMetric)
	}

	wg.Wait()
	return PodsMetrics
}

// 合并上面的Podresource及PodMetrics信息到podInfoList中
func CombinePodInfo(resources map[string]PodResource, metrics map[string]PodMetrics) []*PodInfo {
	podInfoList := make([]*PodInfo, 0)

	for key, resource := range resources {
		if metric, ok := metrics[key]; ok {
			podInfo := &PodInfo{
				PodResource: resource,
				PodMetrics:  metric,
			}

			totalCPUUsage, totalCPURequests, totalCPULimits := int64(0), int64(0), int64(0)
			totalMemUsage, totalMemRequests, totalMemLimits := int64(0), int64(0), int64(0)

			podInfo.ContainersRatio = make(map[string]*ContainerRatio)

			for containerName, containerResource := range podInfo.PodResource.Containers {
				containerMetric := podInfo.PodMetrics.Containers[containerName]

				totalCPUUsage += containerMetric.CPUUsage
				totalCPURequests += containerResource.CPURequests
				totalCPULimits += containerResource.CPULimits

				totalMemUsage += containerMetric.MemUsage
				totalMemRequests += containerResource.MemRequest
				totalMemLimits += containerResource.MemLimits

				containerRatio := &ContainerRatio{
					CPUUsageToRequestRatio: calculateRatio(containerMetric.CPUUsage, containerResource.CPURequests),
					CPUUsageToLimitsRatio:  calculateRatio(containerMetric.CPUUsage, containerResource.CPULimits),
					MemUsageToRequestRatio: calculateRatio(containerMetric.MemUsage, containerResource.MemRequest),
					MemUsageToLimitsRatio:  calculateRatio(containerMetric.MemUsage, containerResource.MemLimits),
				}

				podInfo.ContainersRatio[containerName] = containerRatio
			}

			podInfo.CPUUsageToRequestRatio = calculateRatio(totalCPUUsage, totalCPURequests)
			podInfo.CPUUsageToLimitsRatio = calculateRatio(totalCPUUsage, totalCPULimits)
			podInfo.MemUsageToRequestRatio = calculateRatio(totalMemUsage, totalMemRequests)
			podInfo.MemUsageToLimitsRatio = calculateRatio(totalMemUsage, totalMemLimits)

			podInfoList = append(podInfoList, podInfo)
		}
	}

	return podInfoList
}

func PrintResult(ctx context.Context, namespace string) {
	defer cancel()
	if !Validate(namespace) {
		fmt.Println("指定的命名空间不存在")
		os.Exit(1)
	}
	resources := LoadK8sResource(ctx, namespace)
	metrics := LoadK8sMetrics(ctx, namespace)

	combinedPodInfoList := SortPodInfo(CombinePodInfo(resources, metrics))

	podResults := make([][]string, 0)
	// 输出结果
	for _, podInfo := range combinedPodInfoList {
		if podSortByContainer {
			// 输出容器级别的信息
			for _, container := range podInfo.PodResource.Containers {
				containerRatio := podInfo.ContainersRatio[container.Name]
				result := []string{
					podInfo.NodeName,
					podInfo.PodResource.PodName,
					container.Name,
					formatValue(containerRatio.CPUUsageToRequestRatio),
					formatValue(containerRatio.MemUsageToRequestRatio),
					formatValue(containerRatio.CPUUsageToLimitsRatio),
					formatValue(containerRatio.MemUsageToLimitsRatio),
				}
				podResults = append(podResults, result)
			}
		} else {
			// 输出Pod级别的信息
			result := []string{
				podInfo.NodeName,
				podInfo.PodResource.PodName,
				formatValue(podInfo.CPUUsageToRequestRatio),
				formatValue(podInfo.MemUsageToRequestRatio),
				formatValue(podInfo.CPUUsageToLimitsRatio),
				formatValue(podInfo.MemUsageToLimitsRatio),
			}
			podResults = append(podResults, result)
		}
	}

	table := tablewriter.NewWriter(os.Stdout)
	if podSortByContainer {
		table.SetHeader([]string{"运行节点", "pod名称", "容器名称", "cpu用量/request占比", "内存用量/request占比", "cpu用量/limit占比", "内存用量/limit占比"})
	} else {
		table.SetHeader([]string{"运行节点", "pod名称", "cpu用量/request占比", "内存用量/request占比", "cpu用量/limit占比", "内存用量/limit占比"})
	}
	table.AppendBulk(podResults)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("\t") // pad with space
	table.SetNoWhiteSpace(true)
	table.Render()
}

func encode(ns, pod string) string {
	str := fmt.Sprintf("%s/%s", ns, pod)
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func SortPodInfo(combinedPodInfoList []*PodInfo) []*PodInfo {
	// 对combinedPodInfoList进行排序
	sort.Slice(combinedPodInfoList, func(i, j int) bool {
		podNamePartsI := strings.Split(combinedPodInfoList[i].PodResource.PodName, "-")
		podNamePartsJ := strings.Split(combinedPodInfoList[j].PodResource.PodName, "-")

		// 取前两位进行分组
		groupI := strings.Join(podNamePartsI[:2], "-")
		groupJ := strings.Join(podNamePartsJ[:2], "-")

		if groupI != groupJ {
			return groupI < groupJ
		}

		// 根据命令行选项来判断是否进行容器级别排序
		if podSortByContainer {
			return containerSortLess(combinedPodInfoList[i].PodResource.Containers, combinedPodInfoList[j].PodResource.Containers, func(container *ContainerResource) int64 {
				return container.CPURequests
			})
		}

		// 根据排序规则进行排序
		switch podSortBy {
		case "cpu.request":
			return combinedPodInfoList[i].CPUUsageToRequestRatio < combinedPodInfoList[j].CPUUsageToRequestRatio
		case "mem.request":
			return combinedPodInfoList[i].MemUsageToRequestRatio < combinedPodInfoList[j].MemUsageToRequestRatio
		case "cpu.limit":
			return combinedPodInfoList[i].CPUUsageToLimitsRatio < combinedPodInfoList[j].CPUUsageToLimitsRatio
		case "mem.limit":
			return combinedPodInfoList[i].MemUsageToLimitsRatio < combinedPodInfoList[j].MemUsageToLimitsRatio
		default:
			return false
		}
	})

	// 如果Pod是DaemonSet类型，只保留前10个PodInfo
	if isDaemonSetPod(combinedPodInfoList[0].PodResource.PodName) && len(combinedPodInfoList) > 10 {
		combinedPodInfoList = combinedPodInfoList[:10]
	}

	return combinedPodInfoList
}

func containerSortLess(containersA, containersB map[string]*ContainerResource, getMetric func(*ContainerResource) int64) bool {
	// Convert map to slice for sorting
	sliceA := make([]*ContainerResource, 0, len(containersA))
	for _, value := range containersA {
		sliceA = append(sliceA, value)
	}

	sliceB := make([]*ContainerResource, 0, len(containersB))
	for _, value := range containersB {
		sliceB = append(sliceB, value)
	}

	for i := 0; i < len(sliceA) && i < len(sliceB); i++ {
		if getMetric(sliceA[i]) != getMetric(sliceB[i]) {
			return getMetric(sliceA[i]) < getMetric(sliceB[i])
		}
	}
	return len(sliceA) < len(sliceB)
}

// 判断是否为StatefulSet类型的Pod
func isDaemonSetPod(podName string) bool {
	for _, keyword := range daemonsetPod {
		if strings.Contains(podName, keyword) {
			return true
		}
	}
	return false
}

// 计算usage和request/limit之间的比例
func calculateRatio(usage int64, requestOrLimits int64) float64 {
	if requestOrLimits == 0 {
		return 0 // 使用0表示无法计算
	}
	return float64(usage) / float64(requestOrLimits) * 100
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

// 判断是否存在指定的namespace
func Validate(namespace string) bool {
	namespaces, err := lib.GetK8sClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
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
