package main

import (
	"fmt"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/Thynaptic/P-LMv1/pkg/benchmark/runners"
	"github.com/spf13/cobra"
)

var toolcallCmd = &cobra.Command{
	Use:   "toolcall",
	Short: "Run a benchmark for tool-call schema compliance.",
	Long:  `This command benchmarks model ability to emit valid structured tool calls used by TALOS tool orchestration.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running tool-call benchmark...")

		models, err := benchmark.GetOllamaModels()
		if err != nil {
			fmt.Printf("Error getting ollama models: %v\n", err)
			return
		}
		if len(models) == 0 {
			fmt.Println("No ollama models found on the remote server.")
			return
		}

		results, err := runners.RunToolCallBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running tool-call benchmarks: %v\n", err)
			return
		}

		fmt.Println("\n--- Tool-Call Benchmark Results ---")
		for i, result := range results {
			fmt.Printf("%d. Model: %s\n", i+1, result.ModelName)
			fmt.Printf("  Time to first token: %s\n", result.TimeToFirstToken)
			fmt.Printf("  Total time: %s\n", result.TotalTime)
			fmt.Printf("  Tokens per second: %.2f\n", result.TokensPerSecond)
			fmt.Printf("  Tool call valid: %t\n", result.ToolCallValid)
			if result.ToolName != "" {
				fmt.Printf("  Tool name: %s\n", result.ToolName)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(toolcallCmd)
}
