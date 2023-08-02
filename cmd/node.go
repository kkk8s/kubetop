package cmd

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"metrics.k8s.io/lib"
)

var (
	cpuRequest, cpuAllocated, memoryRequest, memoryAllocated resource.Quantity
	// 互斥锁用于保证并发安全
	mutex sync.Mutex
	wg sync.WaitGroup

	watermark float64 = 20.0    // 内存及CPU阈值
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "print the CPU/Mem remaining of nodes",
	Run: func(cmd *cobra.Command, args []string) {
		GetNodeResource()
	},
}

// 定义一个结构体用于存储节点的资源信息
type nodeInfo struct {
	NodeName        string
	CPUTotal        int64
	CPUAllocated    int64
	CPURemaining    int64
	CPUPercentage   float64
	MemoryTotal     int64
	MemoryAllocated int64
	MemoryRemaining int64
	MemoryPercentage float64
}

// 定义一个结构体用于存储节点的资源信息
type nodeResource struct {
	cpuRequest    int64
	memoryRequest int64
}

func GetNodeResource() {
	nodes, err := lib.GetK8sClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	handlerError(err)

	pods, err := lib.GetK8sClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	handlerError(err)

	// 创建一个用于存储节点资源信息的映射，键为节点名称，值为节点资源信息
	nodeResources := make(map[string]nodeResource, 0)

	// 使用 WaitGroup 来同步并发任务
	wg.Add(len(pods.Items))
	// 遍历所有 Pod，对每个 Pod 启动一个 Goroutine 并进行资源累加
	for _, pod := range pods.Items {
		go func(pod v1.Pod) {
			defer wg.Done()

			nodeName := pod.Spec.NodeName
			cpuRequest, memoryRequest := calculatePodRequests(pod)

			// 使用互斥锁保证并发更新 nodeResources 的安全性
			mutex.Lock()
			defer mutex.Unlock()

			// 更新节点的资源信息
			nodeResource := nodeResources[nodeName]
			nodeResource.cpuRequest += cpuRequest
			nodeResource.memoryRequest += memoryRequest
			nodeResources[nodeName] = nodeResource
		}(pod)
	}

	// 等待所有 Goroutine 完成
	wg.Wait()

	// 遍历所有节点，输出节点的资源信息
	// 遍历所有节点，计算节点的总资源和剩余资源，并输出结果
	var nodeInfoList []nodeInfo
	for _, node := range nodes.Items {
		nodeName := node.Name
		nodeResource := nodeResources[nodeName]

		// 获取节点的可分配资源信息
		cpuTotal, memoryTotal := getNodeRequests(node)

		// 计算节点的剩余资源
		cpuRemaining := calculateRemainingResource(cpuTotal, nodeResource.cpuRequest)
		memoryRemaining := calculateRemainingResource(memoryTotal, nodeResource.memoryRequest)

		// 计算资源百分比
		// cpuTotal := cpuRequest.MilliValue()
		// memoryTotal := memoryRequest.Value() / 1024 / 1024
		cpuPercentage := float64(cpuRemaining) / float64(cpuTotal) * 100
		memoryPercentage := float64(memoryRemaining) / float64(memoryTotal) * 100

		// 添加节点信息到 nodeInfoList 切片中
		nodeInfoList = append(nodeInfoList, nodeInfo{
			NodeName:        nodeName,
			CPUTotal:        cpuTotal,
			CPUAllocated:    nodeResource.cpuRequest,
			CPURemaining:    cpuRemaining,
			CPUPercentage:   cpuPercentage,
			MemoryTotal:     memoryTotal,
			MemoryAllocated: nodeResource.memoryRequest,
			MemoryRemaining: memoryRemaining,
			MemoryPercentage: memoryPercentage,
		})
	}

	// 根据用户传入的选项进行排序
	switch sortBy {
	case "cpu":
		sort.Slice(nodeInfoList, func(i, j int) bool {
			return nodeInfoList[i].CPUPercentage < nodeInfoList[j].CPUPercentage
		})
	case "mem":
		sort.Slice(nodeInfoList, func(i, j int) bool {
			return nodeInfoList[i].MemoryPercentage < nodeInfoList[j].MemoryPercentage
		})
	default:
		fmt.Println("未知的排序选项")
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"节点名称","cpu总量(m)","cpu已分配(m)","cpu剩余值(m)","cpu剩余百分比","内存总量(MiB)","内存已分配(MiB)","内存剩余值(MiB)","内存剩余百分比"})
	nodeResults := make([][]string,256)

	// 输出结果
	for _, nodeInfo := range nodeInfoList {
		result := []string{
			nodeInfo.NodeName,
			strconv.FormatInt(nodeInfo.CPUTotal, 10),
			strconv.FormatInt(nodeInfo.CPUAllocated, 10),
			strconv.FormatInt(nodeInfo.CPURemaining, 10),
			// fmt.Sprintf("%.2f%%",nodeInfo.CPUPercentage),
			colorize(nodeInfo.CPUPercentage),
			strconv.FormatInt(nodeInfo.MemoryTotal, 10),
			strconv.FormatInt(nodeInfo.MemoryAllocated, 10),
			strconv.FormatInt(nodeInfo.MemoryRemaining, 10),
			// fmt.Sprintf("%.2f%%",nodeInfo.MemoryPercentage),
			colorize(nodeInfo.MemoryPercentage),
		}
		nodeResults = append(nodeResults,result)
	}

	table.AppendBulk(nodeResults)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.Render()
}


// 获取节点已经分配的资源
func calculatePodRequests(pod v1.Pod) (cpu, memory int64) {
	for _, container := range pod.Spec.Containers {
		cpuRequest := container.Resources.Requests[v1.ResourceCPU]
		memoryRequest := container.Resources.Requests[v1.ResourceMemory]
		cpu += cpuRequest.MilliValue()
		memory += memoryRequest.Value() / 1024 / 1024 // 将字节转换为 MB
	}

	if len(pod.Spec.InitContainers) > 0 {
		initContainer := pod.Spec.InitContainers[0]
		cpuRequest := initContainer.Resources.Requests[v1.ResourceCPU]
		memoryRequest := initContainer.Resources.Requests[v1.ResourceMemory]
		cpu += cpuRequest.MilliValue()
		memory += memoryRequest.Value() / 1024 / 1024 // 将字节转换为 MB
	}

	return cpu, memory
}


// 获取节点的可分配的CPU及内存信息
func getNodeRequests(node v1.Node) (cpu, memory int64) {
	cpuRequest := node.Status.Allocatable[v1.ResourceCPU]       // cpu 可分配值
	memoryRequest := node.Status.Allocatable[v1.ResourceMemory] // 内存 可分配值

	return cpuRequest.MilliValue(), memoryRequest.Value() / 1024 / 1024 // 将字节转换为 MiB
}

// 计算剩余资源值
func calculateRemainingResource(request, allocated int64) int64 {
	remaining := request - allocated
	if remaining < 0 {
		return 0
	}
	return remaining
}

func colorize(percentage float64) string {
	if percentage < watermark {
		return fmt.Sprintf("\x1b[31m%.2f%%\x1b[0m", percentage)
	}
	return fmt.Sprintf("%.2f%%", percentage)
}
func handlerError(err error) {
	if err != nil {
		log.Fatalln(err.Error())
	}
}
