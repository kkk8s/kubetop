package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

var (
	kubetopLong = `
	1. 展示pod资源申请与实际值的差异(资源申请与限额仅计算Containers，initContainers不作计算)
	2. 展示node节点的资源剩余百分比
	`
	kubetopExample = `
	# 1. 展示 kube-system 命名空间下资源申请量与使用量
	kubetop pod -n [namespace]
	eg: kubetop pod -n kube-system

	# 2. 展示node节点资源申请值剩余情况
	kubetop node
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

	// 为nodeCmd添加--sort选项
	nodeCmd.Flags().StringVar(&sortBy, "sort-by", "cpu", "使用剩余CPU或者剩余内存进行排序")

	timeout = time.Second * 5 // 全局超时时间
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
}

func Execute() error {
	return rootCmd.Execute()
}
