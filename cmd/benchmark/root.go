package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "A benchmark tool for system specs and ollama models",
	Long:  `P-LMv1 is a CLI tool to benchmark system specifications and test installed ollama models.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default action when no command is provided
		fmt.Println("P-LMv1 Benchmark Tool")
		fmt.Println("Use 'benchmark --help' for more information.")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
