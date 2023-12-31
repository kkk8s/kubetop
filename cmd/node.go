package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"metrics.k8s.io/kube"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// 互斥锁用于保证并发安全
	mutex sync.Mutex
	wg    sync.WaitGroup

	watermark  float64 = 20.0 // 内存及CPU阈值
	nodeHeader         = []string{"节点名称", "cpu|request剩余率", "cpu|实际使用率", "内存|request剩余率", "内存|实际使用率"}
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "print the CPU/Mem remaining of nodes",
	Run: func(cmd *cobra.Command, args []string) {
		GetNodeResource(rootCmd.Context())
	},
	Args:    cobra.NoArgs,
	Aliases: []string{"nodes", "no"},
}

// 定义一个结构体用于存储节点的资源信息
type nodeInfo struct {
	NodeName           string
	CPUTotal           int64
	CPUAllocated       int64
	CPURemaining       int64
	CPUPercentage      float64
	NodeCPUUtilization string // 节点实际CPU使用率
	MemoryTotal        int64
	MemoryAllocated    int64
	MemoryRemaining    int64
	MemoryPercentage   float64 // 节点实际内存使用率
	NodeMemUtilization string
}

// 定义一个结构体用于存储节点的资源信息
type nodeResource struct {
	cpuRequest    int64 // 节点总cpu请求量
	memoryRequest int64 // 节点总内存请求量
}

// 定义一个结构体用于存储节点的实际使用指标信息
type nodeMetrics struct {
	cpuPercentage string // 节点实际CPU用量
	memPercentage string // 节点实际内存用量
}

func GetNodeResource(ctx context.Context) {
	nodes, err := kube.GetK8sClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	kube.Error(err, "列出节点失败")

	pods, err := kube.GetK8sClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	kube.Error(err, "列出所有Pod失败")

	// 创建一个用于存储节点资源信息的映射，键为节点名称，值为节点资源信息
	nodeResources := make(map[string]nodeResource, 0)

	// 定义map存储节点名到v1.Node的映射
	nodeMap := make(map[string]v1.Node, 0)
	for _, node := range nodes.Items {
		nodeMap[node.Name] = node
	}

	// 使用 WaitGroup 来同步并发任务
	wg.Add(len(pods.Items))
	// 遍历所有 Pod，对每个 Pod 启动一个 Goroutine 并进行资源累加
	for _, pod := range pods.Items {
		go func(pod v1.Pod) {
			// 使用互斥锁保证并发更新 nodeResources 的安全性
			mutex.Lock()
			defer mutex.Unlock()
			defer wg.Done()

			nodeName := pod.Spec.NodeName
			cpuRequest, memoryRequest := calculatePodRequests(pod)

			// 更新节点的资源信息
			nodeResource := nodeResources[nodeName]
			nodeResource.cpuRequest += cpuRequest
			nodeResource.memoryRequest += memoryRequest
			nodeResources[nodeName] = nodeResource
		}(pod)
	}

	// 等待所有 Goroutine 完成
	wg.Wait()

	nodesMetrics := getNodeUtilization(ctx, nodeMap)

	// 遍历所有节点，计算节点的总资源和剩余资源，并输出结果
	var nodeInfoList []nodeInfo
	for _, node := range nodes.Items {
		nodeName := node.Name
		nodeResource := nodeResources[nodeName]
		nodeMetrics := nodesMetrics[nodeName]

		// 获取节点的可分配资源信息
		cpuTotal, memoryTotal := getNodeAllocatable(node)

		// 计算节点的剩余资源
		cpuRemaining := calculateRemaining(cpuTotal, nodeResource.cpuRequest)
		memoryRemaining := calculateRemaining(memoryTotal, nodeResource.memoryRequest)

		// 计算资源百分比
		cpuPercentage := calculateRemaingPercentage(cpuRemaining, cpuTotal)
		memoryPercentage := calculateRemaingPercentage(memoryRemaining, memoryTotal)

		if nodeMetrics.cpuPercentage == "" || nodeMetrics.memPercentage == ""  {
			kube.Info((fmt.Errorf("节点%s资源异常",nodeName)),"跳过该节点,")
			continue
		}

		// 添加节点信息到 nodeInfoList 切片中
		nodeInfoList = append(nodeInfoList, nodeInfo{
			NodeName: nodeName,
			// CPUTotal:           cpuTotal,
			// CPUAllocated:       nodeResource.cpuRequest,
			// CPURemaining:       cpuRemaining,
			CPUPercentage:      cpuPercentage,
			NodeCPUUtilization: nodeMetrics.cpuPercentage,
			// MemoryTotal:        memoryTotal,
			// MemoryAllocated:    nodeResource.memoryRequest,
			// MemoryRemaining:    memoryRemaining,
			MemoryPercentage:   memoryPercentage,
			NodeMemUtilization: nodeMetrics.memPercentage,
		})
	}

	// 根据用户传入的选项进行排序
	switch nodeSortBy {
	case "cpu.request":
		sort.Slice(nodeInfoList, func(i, j int) bool {
			return nodeInfoList[i].CPUPercentage < nodeInfoList[j].CPUPercentage
		})
	case "cpu.util":
		sort.Slice(nodeInfoList, func(i, j int) bool {
			cpuUtilI, err := strconv.ParseFloat(strings.TrimSuffix(nodeInfoList[i].NodeCPUUtilization, "%"), 64)
			kube.Error(err, "cpu使用率转换失败")

			cpuUtilJ, err := strconv.ParseFloat(strings.TrimSuffix(nodeInfoList[j].NodeCPUUtilization, "%"), 64)
			kube.Error(err, "cpu使用率转换失败")

			return cpuUtilI > cpuUtilJ
		})
	case "mem.request":
		sort.Slice(nodeInfoList, func(i, j int) bool {
			return nodeInfoList[i].MemoryPercentage < nodeInfoList[j].MemoryPercentage
		})
	case "mem.util":
		sort.Slice(nodeInfoList, func(i, j int) bool {
			memUtilI, err := strconv.ParseFloat(strings.TrimSuffix(nodeInfoList[i].NodeMemUtilization, "%"), 64)
			kube.Error(err, "内存使用率转换失败")

			memUtilJ, err := strconv.ParseFloat(strings.TrimSuffix(nodeInfoList[j].NodeMemUtilization, "%"), 64)
			kube.Error(err, "内存使用率转换失败")
			return memUtilI > memUtilJ
		})
	default:
		fmt.Println("未知的排序选项")
		return
	}

	nodeResults := make([][]string, 0)
	// 输出结果
	for _, nodeInfo := range nodeInfoList {
		result := []string{
			nodeInfo.NodeName,
			// strconv.FormatInt(nodeInfo.CPUTotal, 10),
			// strconv.FormatInt(nodeInfo.CPUAllocated, 10),
			// strconv.FormatInt(nodeInfo.CPURemaining, 10),
			colorize(nodeInfo.CPUPercentage),
			nodeInfo.NodeCPUUtilization,
			// strconv.FormatInt(nodeInfo.MemoryTotal, 10),
			// strconv.FormatInt(nodeInfo.MemoryAllocated, 10),
			// strconv.FormatInt(nodeInfo.MemoryRemaining, 10),
			colorize(nodeInfo.MemoryPercentage),
			nodeInfo.NodeMemUtilization,
		}
		nodeResults = append(nodeResults, result)
	}

	table := kube.NewTable()
	// table.SetHeader([]string{"节点名称", "cpu总量(m)", "cpu已分配(m)", "cpu剩余值(m)", "cpu剩余百分比", "cpu实际使用", "内存总量(MiB)", "内存已分配(MiB)", "内存剩余值(MiB)", "内存剩余百分比", "内存实际使用"})
	table.SetHeader(nodeHeader)
	table.AppendBulk(nodeResults)
	table.Render()
}

// 获取节点已经分配的资源
func calculatePodRequests(pod v1.Pod) (cpu, memory int64) {
	for _, container := range pod.Spec.Containers {
		cpuRequest := container.Resources.Requests[v1.ResourceCPU]
		memoryRequest := container.Resources.Requests[v1.ResourceMemory]
		cpu += cpuRequest.MilliValue()
		memory += memoryRequest.Value() / Mebibyte // 将字节转换为 MB
	}

	if len(pod.Spec.InitContainers) > 0 {
		initContainer := pod.Spec.InitContainers[0]
		cpuRequest := initContainer.Resources.Requests[v1.ResourceCPU]
		memoryRequest := initContainer.Resources.Requests[v1.ResourceMemory]
		cpu += cpuRequest.MilliValue()
		memory += memoryRequest.Value() / Mebibyte // 将字节转换为 MB
	}

	return cpu, memory
}

// 获取节点的可分配的CPU及内存
func getNodeAllocatable(node v1.Node) (cpu, memory int64) {
	cpuRequest := node.Status.Allocatable[v1.ResourceCPU]       // cpu 可分配值
	memoryRequest := node.Status.Allocatable[v1.ResourceMemory] // 内存 可分配值

	return cpuRequest.MilliValue(), memoryRequest.Value() / Mebibyte // 将字节转换为 MB
}

// 获取节点的最大的CPU及内存
func getNodeCapacity(node v1.Node) (cpu, memory int64) {
	cpuRequest := node.Status.Capacity[v1.ResourceCPU]       // cpu 可分配值
	memoryRequest := node.Status.Capacity[v1.ResourceMemory] // 内存 可分配值

	return cpuRequest.MilliValue(), memoryRequest.Value() / Mebibyte // 将字节转换为 MB
}

// 计算剩余资源值
func calculateRemaining(request, allocated int64) int64 {
	remaining := request - allocated
	if remaining < 0 {
		return 0
	}
	return remaining
}

// 返回节点实际资源使用率
func getNodeUtilization(ctx context.Context, nodeMap map[string]v1.Node) map[string]nodeMetrics {
	nodeMetricsList, err := kube.GetMetricsClient().MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	kube.Error(err, "列出所有节点metrics指标失败")

	// 定义映射来存储每个节点的资源使用百分比
	nodeMetricsMap := make(map[string]nodeMetrics)

	for _, nm := range nodeMetricsList.Items {
		nodeName := nm.Name
		cpuUsage := nm.Usage.Cpu().MilliValue()          // 将毫核转换为核
		memUsage := nm.Usage.Memory().Value() / Mebibyte // 将字节转换为 MB

		// 从映射中获取节点信息
		node := nodeMap[nodeName]

		// 获取节点的可分配资源
		cpuTotal, memTotal := getNodeCapacity(node)
		cpuPercentage := calculateRemaingPercentage(cpuUsage, cpuTotal)
		memPercentage := calculateRemaingPercentage(memUsage, memTotal)

		nodeMetricsMap[nodeName] = nodeMetrics{
			cpuPercentage: fmt.Sprintf("%.2f%%", cpuPercentage),
			memPercentage: fmt.Sprintf("%.2f%%", memPercentage),
		}
	}

	return nodeMetricsMap
}

func calculateRemaingPercentage(remain, total int64) float64 {
	return float64(remain) / float64(total) * 100
}

func colorize(percentage float64) string {
	if percentage < watermark {
		return fmt.Sprintf("\x1b[31m%.2f%%\x1b[0m", percentage)
	}
	return fmt.Sprintf("%.2f%%", percentage)
}
