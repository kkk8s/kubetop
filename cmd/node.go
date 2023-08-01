package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"metrics.k8s.io/lib"
)

var (
	cpuRequest, cpuAllocated, memoryRequest, memoryAllocated resource.Quantity
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "print the CPU/Mem remaining of nodes",
	Run: func(cmd *cobra.Command, args []string) {
		GetNodeResource()
	},
}

func GetNodeResource() {
	nodes, err := lib.GetK8sClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	handlerError(err)

	// 遍历所有节点，获取请求资源的剩余情况
	for _, node := range nodes.Items {
		fmt.Printf("节点名称: %s\n", node.Name)

		cpuRequest, memoryRequest := getNodeRequests(node)
		cpuAllocated, memoryAllocated := getNodeAllocatedResources(node)

		cpuRemaining := calculateRemainingResource(cpuRequest, cpuAllocated)
		memoryRemaining := calculateRemainingResource(memoryRequest, memoryAllocated)

		fmt.Printf("CPU 剩余资源: %d m\n", cpuRemaining)
		fmt.Printf("内存 剩余资源: %d MiB\n", memoryRemaining)
		fmt.Println("---------------------------")
	}
}

// 获取节点的可分配的CPU及内存信息
func getNodeRequests(node v1.Node) (cpu, memory int64) {
	cpuRequest = node.Status.Allocatable[v1.ResourceCPU]       // cpu 可分配值
	memoryRequest = node.Status.Allocatable[v1.ResourceMemory] // 内存 可分配值

	return cpuRequest.MilliValue(), memoryRequest.Value() / 1024 / 1024 // 将字节转换为 MiB
}

// 获取节点已经分配的资源
func getNodeAllocatedResources(node v1.Node) (cpu, memory int64) {
	pods, err := lib.GetK8sClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	handlerError(err)

	for _, pod := range pods.Items {
		if pod.Spec.NodeName == node.Name {

			for _, container := range pod.Spec.Containers {
				cpuRequest := container.Resources.Requests[v1.ResourceCPU]
				memoryRequest := container.Resources.Requests[v1.ResourceMemory]
				cpu += cpuRequest.MilliValue()
				memory += memoryRequest.Value() / 1024 / 1024 // 将字节转换为 MB
			}

			if len(pod.Spec.InitContainers) > 0 && (cpuRequest.MilliValue() == 0 || memoryRequest.Value() == 0) {
				initContainer := pod.Spec.InitContainers[0]
				cpuRequest := initContainer.Resources.Requests[v1.ResourceCPU]
				memoryRequest := initContainer.Resources.Requests[v1.ResourceMemory]
				cpu += cpuRequest.MilliValue()
				memory += memoryRequest.Value() / 1024 / 1024 // 将字节转换为 MB
			}
			fmt.Printf("podName=%s\tcpuRequest=%d\t\tmemoryRequest=%d\n",pod.Name,cpu,memory)
		}

	}
	return cpu, memory
}

func calculateRemainingResource(request, allocated int64) int64 {
	remaining := request - allocated
	if remaining < 0 {
		return 0
	}
	return remaining
}

// func isInitContainer(pod *v1.Pod, containerName string) bool {
// 	for _, initContainer := range pod.Spec.InitContainers {
// 		if initContainer.Name == containerName {
// 			return true
// 		}
// 	}
// 	return false
// }

func handlerError(err error) {
	if err != nil {
		log.Fatalln(err.Error())
	}
}
