package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

var (
	kubetopLong = `
	1. 展示pod资源申请与实际值的差异(资源申请与限额仅计算Containers，initContainers不作计算)
	2. 展示node节点的资源剩余百分比并排序
	`
	kubetopExample = `
	# 1. 展示 kube-system 命名空间下资源量并按照pod实际cpu使用量/申请值的百分比进行排序
	kubetop pod -n kube-system --sort-by=cpu

	# 2. 展示 kube-system 命名空间下资源量并按照pod实际内存使用量/申请值的百分比进行排序
	kubetop pod -n kube-system --sort-by=mem

	# 3. 展示node节点资源剩余情况并按照cpu排序(默认行为)
	kubetop node --sort-by=cpu

	# 4. 展示node节点资源剩余情况并按照内存排序
	kubetop node --sort-by=mem
	`
	namespace string
	sortBy    string
)

var rootCmd = &cobra.Command{
	DisableFlagParsing: true,
	Short:              "k8s资源查看工具",
	Long:               kubetopLong,
	Example:            kubetopExample,
}

func init() {
	rootCmd.AddCommand(podCmd)
	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(versionCmd)

	// 为 podCmd 添加 -n 或 --namespace 选项
	podCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "指定查询的命名空间")
	podCmd.Flags().StringVar(&sortBy, "sort-by", "cpu", "使用CPU剩余率或者内存剩余率进行排序")

	// 为nodeCmd添加--sort选项
	nodeCmd.Flags().StringVar(&sortBy, "sort-by", "cpu", "使用剩余CPU或者剩余内存进行排序")

	timeout = time.Second * 5 // 全局超时时间
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
}

func Execute() error {
	return rootCmd.Execute()
}
