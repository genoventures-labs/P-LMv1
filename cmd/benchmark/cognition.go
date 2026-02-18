package main

import (
	"fmt"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/Thynaptic/P-LMv1/pkg/benchmark/runners"
	"github.com/spf13/cobra"
)

var cognitionCmd = &cobra.Command{
	Use:   "cognition",
	Short: "Run a benchmark for structured cognition output quality.",
	Long:  `This command benchmarks model ability to produce structured planning/risk/decision output aligned with TALOS cognition workflows.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running cognition benchmark...")

		models, err := benchmark.GetOllamaModels()
		if err != nil {
			fmt.Printf("Error getting ollama models: %v\n", err)
			return
		}
		if len(models) == 0 {
			fmt.Println("No ollama models found on the remote server.")
			return
		}

		results, err := runners.RunCognitionBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running cognition benchmarks: %v\n", err)
			return
		}

		fmt.Println("\n--- Cognition Benchmark Results ---")
		for i, result := range results {
			fmt.Printf("%d. Model: %s\n", i+1, result.ModelName)
			fmt.Printf("  Time to first token: %s\n", result.TimeToFirstToken)
			fmt.Printf("  Total time: %s\n", result.TotalTime)
			fmt.Printf("  Tokens per second: %.2f\n", result.TokensPerSecond)
			fmt.Printf("  Structured pass: %t\n", result.StructuredPass)
			fmt.Printf("  Section score: %.2f\n", result.SectionScore)
		}
	},
}

func init() {
	rootCmd.AddCommand(cognitionCmd)
}
