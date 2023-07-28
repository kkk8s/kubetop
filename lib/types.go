package lib

// 定义单个pod要采集的资源
type ResourceStruct struct {
	NodeName         string // Pod所在的节点
	Namespace        string
	PodName          string
	CPUTotalRequests string
	CPUTotalLimits   string
	MemTotalRequest  string
	MemTotalLimits   string
}

// 定义单个pod要采集的指标
type MetricsStruct struct {
	Namespace string
	PodName   string
	CPUUsage  string
	MemUsage  string
}

type (
	ClientsetMap  map[string]ResourceStruct
	MetricsMap map[string]MetricsStruct
)
