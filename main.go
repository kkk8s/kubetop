package main

import (
	"fmt"
	"os"

	"metrics.k8s.io/cmd"
	_ "metrics.k8s.io/kube"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
