package main

import (
	"fmt"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/Thynaptic/P-LMv1/pkg/benchmark/runners"
	"github.com/spf13/cobra"
)

var warmup bool

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the benchmark tests against a remote Ollama server",
	Long:  `This command runs the benchmark tests against models installed on a remote Ollama server.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running remote benchmark tests...")

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

		modelSizes := make(map[string]string)
		for _, model := range models {
			fmt.Printf("- %s (%s)\n", model.Name, model.Size)
			modelSizes[model.Name] = model.Size
		}

		results, err := runners.RunStandardBenchmarks(models, warmup)
		if err != nil {
			fmt.Printf("Error running benchmarks: %v\n", err)
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
		}

		fmt.Println("\n--- Recommendations ---")
		vpsRamGB := 16.0 // As provided by the user

		if len(results) > 0 {
			fmt.Printf("🏆 Best Model (by Tokens/Second): %s\n", results[0].ModelName)
		}
		if len(results) > 1 {
			fmt.Printf("🥈 2nd Best Model (by Tokens/Second): %s\n", results[1].ModelName)
		}

		// Recommendation logic: fastest model that is < 50% of VPS RAM
		recommendedModel := "N/A"
		recommendationReason := "No models fit the criteria."
		for _, result := range results {
			sizeStr := modelSizes[result.ModelName]
			sizeGB, err := benchmark.ParseSizeGB(sizeStr)
			if err != nil {
				// Handle parsing error if necessary, for now, skip
				continue
			}

			if sizeGB < (vpsRamGB * 0.5) {
				recommendedModel = result.ModelName
				recommendationReason = fmt.Sprintf("Fastest model under 50%% of VPS RAM (%.1f GB).", vpsRamGB)
				break // Found the best one that fits
			}
		}

		fmt.Printf("✅ Recommended Model (for this system): %s\n", recommendedModel)
		fmt.Printf("   Reason: %s\n", recommendationReason)

	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&warmup, "warmup", false, "Run a warmup routine before the benchmark")
}
