package cmd

import (
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"metrics.k8s.io/lib"
)

var (
	kubetopLong = `
	展示pod资源申请与实际值的差异
	kubetop让你pod资源申请值与实际的资源使用量
	(资源申请与限额仅计算Containers，initContainers不作计算)
	`
	kubetopExample = `
	# 展示 kube-system 命名空间下资源申请量与使用量
	kubetop -n kube-system
	`
	namespace string
)

var rootCmd = &cobra.Command{
	Use:                "kubetop -n [namespace]",
	DisableFlagParsing: false,
	Short:              "展示pod资源申请与实际值的差异",
	Long:               kubetopLong,
	Example:            kubetopExample,
	Run: func(cmd *cobra.Command, args []string) {
		Results(namespace)
	},
}

func init() {
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", namespace, "指定名称空间")
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func Results(namespace string) {
	PodsResource := lib.ParserCommonResouce(namespace)
	PodsMetric := lib.ParserMetricsResouce(namespace)
	
	Results := make([][]string, 2048)
	for _, podResource := range PodsResource {
		for _, podMetric := range PodsMetric {
			if podMetric.PodName == podResource.PodName && podMetric.Namespace == podResource.Namespace {
				result := []string{podResource.Namespace, podResource.NodeName, podResource.PodName, podResource.CPUTotalRequests, podMetric.CPUUsage, podResource.CPUTotalLimits, podResource.MemTotalRequest, podMetric.MemUsage, podResource.MemTotalLimits}
				Results = append(Results, result)
			}
		}
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"名称空间", "运行节点", "pod名", "cpu申请(m)", "cpu实际用量(m)", "cpu限额(m)", "内存申请(MiB)", "内存实际用量(MiB)", "内存限额(MiB)"})
	table.AppendBulk(Results)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.Render()
}
