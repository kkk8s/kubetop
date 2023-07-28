package lib

// 定义单个pod要采集的资源
type ResourceStruct struct {
	PodName          string
	NodeName         string  // Pod所在的节点
	Namespace        string
	CPUTotalRequests string
	CPUTotalLimits   string
	MemTotalRequest  string
	MemTotalLimits   string
}

// 定义单个pod要采集的指标
type MetricsStruct struct {
	PodName   string
	Namespace string
	CPUUsage  string
	MemUsage  string
}

var (
	PodsResource []ResourceStruct
	PodsMetric   []MetricsStruct
)
