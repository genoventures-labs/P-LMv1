package main

import (
	"fmt"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/Thynaptic/P-LMv1/pkg/benchmark/runners"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Run a multi-turn conversation benchmark.",
	Long:  `This command runs a benchmark to test the model's ability to maintain context in a conversation.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running multi-turn conversation benchmark...")

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

		results, err := runners.RunChatBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running chat benchmarks: %v\n", err)
			return
		}

		fmt.Println("\n--- Benchmark Analysis ---")
		fmt.Println("\nBenchmark Results (Ranked by Performance):")
		for i, result := range results {
			fmt.Printf("%d. Model: %s\n", i+1, result.ModelName)
			// TimeToFirstToken is not measured in chat benchmark (yet)
			fmt.Printf("  Total time: %s\n", result.TotalTime)
			fmt.Printf("  Tokens per second: %.2f\n", result.TokensPerSecond)
			fmt.Printf("  Total tokens (approx.): %d\n", result.TotalTokens)
		}
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
}
