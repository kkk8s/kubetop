package cmd

import (
	"log"
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
	# 1. 展示 kube-system 命名空间下资源申请量与使用量
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

func init(){
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", namespace, "指定名称空间")
	rootCmd.MarkFlagRequired("namespace")
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func Results(namespace string) {
	if !lib.Validate(namespace) {
		log.Fatalln("No such namespace")
	}
	PodsResource := lib.ParserCommonResouce(namespace)
	PodsMetric := lib.ParserMetricsResouce(namespace)
	
	results := make([][]string, 2048)
	for key,podResource := range PodsResource {
		if podUsage,ok := PodsMetric[key];ok {
			result := []string{podResource.NodeName, podResource.PodName, podResource.CPUTotalRequests, podUsage.CPUUsage, podResource.CPUTotalLimits, podResource.MemTotalRequest, podUsage.MemUsage, podResource.MemTotalLimits}
			results = append(results,result)
		}
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"运行节点", "pod名称", "cpu申请(m)", "cpu实际用量(m)", "cpu限额(m)", "内存申请(MiB)", "内存实际用量(MiB)", "内存限额(MiB)"})
	table.AppendBulk(results)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.Render()
}