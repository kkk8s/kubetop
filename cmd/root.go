package cmd

import (
	"context"
	"time"

	"metrics.k8s.io/kube"

	"github.com/spf13/cobra"
)

var (
	kubetopLong = `
	1. 展示pod资源申请与实际值的差异(资源申请与限额仅计算Containers，initContainers不作计算)
	2. 展示node节点的资源剩余百分比/实际使用率并排序
	`
	kubetopExample = `
	# 1. 展示 kube-system 命名空间下资源量并按照pod实际cpu使用量/request的百分比进行排序
	kubetop pod -n kube-system --sort-by=cpu.request

	# 2. 展示 kube-system 命名空间下资源量并按照pod实际内存使用量/limit的百分比进行排序
	kubetop pod -n kube-system --sort-by=mem.limit

	# 3. 展示 kube-system 命名空间下资源量并按照pod实际内存使用量/limit的百分比进行排序，同时显示各个容器的指标
	kubetop pod -n kube-system -c --sort-by=mem.limit

	# 3. 展示node节点cpu-request资源剩余率(默认)
	kubetop node --sort-by=cpu.request

	# 4. 展示node节点资源剩余情况并按照内存实际使用率排序
	kubetop node --sort-by=mem.util

	# 5. pod排序规则包括cpu.request、mem.request、cpu.limit、mem.limit
	     node排序规则包括cpu.request、mem.request、cpu.util、mem.util
	
	# 6. 命令行补齐:
	source <(kubetop completion zsh)
	加入到$HOME/.bashrc或者/etc/profile永久生效
	`
	
	namespace          string
	podSortBy          string
	nodeSortBy         string
	podSortByContainer bool

	NamespacesList []string
)

const (
	Kilobyte = 1000
	Mebibyte = Kilobyte * 1000
	Gibibyte = Mebibyte * 1000
)

var rootCmd = &cobra.Command{
	// DisableFlagParsing: false,
	Short:                 "k8s资源查看工具",
	Long:                  kubetopLong,
	Example:               kubetopExample,
	DisableAutoGenTag:     true,
	DisableFlagsInUseLine: true,
	Use:                   "kubetop pod -n [namespace]|node --sort-by=cpu.request",
}

func init() {
	rootCmd.AddCommand(podCmd)
	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(versionCmd)

	// 隐藏help子命令
	rootCmd.SetHelpCommand(&cobra.Command{
		Hidden: true,
	})

	rootCmd.PersistentFlags().StringP("loglevel", "v", "warning", "设置日志级别")

	// 为 podCmd 添加 -n 或 --namespace 选项
	podCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "指定查询的命名空间")
	podCmd.MarkFlagRequired("namespace")
	podCmd.Flags().StringVar(&podSortBy, "sort-by", "cpu.request", "按cpu.request | mem.request | cpu.limit | mem.limit进行排序")
	podCmd.Flags().BoolVarP(&podSortByContainer, "container", "c", false, "Sort by container-level resources")

	// 为nodeCmd添加--sort选项
	nodeCmd.Flags().StringVar(&nodeSortBy, "sort-by", "cpu.request", "按cpu.request | cpu.util | mem.request | mem.util排序")
}

func Execute() error {
	timeout := time.Second * 5 // 全局超时时间
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	loglevel, _ := rootCmd.Flags().GetString("loglevel")
	switch loglevel {
	case "info":
		kube.SetLogLevel(kube.INFO)
	case "warning":
		kube.SetLogLevel(kube.WARNING)
	case "error":
		kube.SetLogLevel(kube.ERROR)
	default:
		kube.SetLogLevel(kube.INFO)
	}

	NamespacesList = ListNamespace(ctx)
	podCmd.RegisterFlagCompletionFunc("namespace", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return NamespacesList, cobra.ShellCompDirectiveDefault
	})

	return rootCmd.ExecuteContext(ctx)
}
