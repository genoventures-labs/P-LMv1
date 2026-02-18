package main

import (
	"fmt"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/Thynaptic/P-LMv1/pkg/benchmark/runners"
	"github.com/spf13/cobra"
)

var jsonCmd = &cobra.Command{
	Use:   "json",
	Short: "Run a benchmark for JSON generation.",
	Long:  `This command runs a benchmark to test the model's ability to generate valid JSON.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running JSON generation benchmark...")

		fmt.Println("\nInstalled Ollama Models:")
		models, err := benchmark.GetOllamaModels()
		if err != nil {
			fmt.Printf("Error getting ollama models: %v\n", err)
			return
		}

		if len(models) == 0 {
			fmt.Println("No ollama models found on the remote server.")
			return
		}

		for _, model := range models {
			fmt.Printf("- %s (%s)\n", model.Name, model.Size)
		}

		results, err := runners.RunJSONBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running JSON benchmarks: %v\n", err)
			return
		}

		fmt.Println("\n--- Benchmark Analysis ---")
		fmt.Println("\nBenchmark Results (Ranked by Performance):")
		for i, result := range results {
			fmt.Printf("%d. Model: %s\n", i+1, result.ModelName)
			fmt.Printf("  Time to first token: %s\n", result.TimeToFirstToken)
			fmt.Printf("  Total time: %s\n", result.TotalTime)
			fmt.Printf("  Tokens per second: %.2f\n", result.TokensPerSecond)
			fmt.Printf("  Total tokens: %d\n", result.TotalTokens)
			fmt.Printf("  JSON Valid: %t\n", result.JSONValid)
		}
	},
}

func init() {
	rootCmd.AddCommand(jsonCmd)
}
