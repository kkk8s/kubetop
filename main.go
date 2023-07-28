package main

import (
	"fmt"
	"os"

	"metrics.k8s.io/cmd"
)

func main(){
	if err := cmd.Execute();err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}