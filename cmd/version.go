package cmd

import (
  "fmt"

  "github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of kubetop",
	Run: func(cmd *cobra.Command, args []string) {
	  fmt.Println("v1.2.0-dev")
	},
}