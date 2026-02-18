package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/Thynaptic/P-LMv1/pkg/benchmark/runners"
	"github.com/spf13/cobra"
)

var noRun bool

var orchestratorCmd = &cobra.Command{
	Use:   "orchestrate",
	Short: "Run all benchmarks and generate a results file.",
	Long:  `The orchestrator runs all available benchmarks (run, json, codegen, chat, toolcall, cognition) against all models, analyzes the results, and stores them in JSON files.`,
	Run: func(cmd *cobra.Command, args []string) {
		var allResults []benchmark.ModelBenchmarkResults

		if !noRun {
			fmt.Println("Starting the benchmark orchestrator...")

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

			// Run all benchmarks for each model
			for _, model := range models {
				fmt.Printf("\n--- Running all benchmarks for %s ---\n", model.Name)

				// Standard Benchmark
				standardResults, err := runners.RunStandardBenchmarks([]benchmark.OllamaModel{model}, false)
				if err != nil {
					fmt.Printf("Error in standard benchmark for %s: %v\n", model.Name, err)
					continue
				}

				// JSON Benchmark
				jsonResults, err := runners.RunJSONBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Error in JSON benchmark for %s: %v\n", model.Name, err)
					continue
				}

				// Codegen Benchmark
				codegenResults, err := runners.RunCodegenBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Error in codegen benchmark for %s: %v\n", model.Name, err)
					continue
				}

				// Chat Benchmark
				chatResults, err := runners.RunChatBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Error in chat benchmark for %s: %v\n", model.Name, err)
					continue
				}
				toolCallResults, err := runners.RunToolCallBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Error in tool-call benchmark for %s: %v\n", model.Name, err)
					continue
				}
				cognitionResults, err := runners.RunCognitionBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Error in cognition benchmark for %s: %v\n", model.Name, err)
					continue
				}

				if len(standardResults) > 0 &&
					len(jsonResults) > 0 &&
					len(codegenResults) > 0 &&
					len(chatResults) > 0 &&
					len(toolCallResults) > 0 &&
					len(cognitionResults) > 0 {
					allResults = append(allResults, benchmark.ModelBenchmarkResults{
						ModelName:          model.Name,
						StandardBenchmark:  standardResults[0],
						JSONBenchmark:      jsonResults[0],
						CodegenBenchmark:   codegenResults[0],
						ChatBenchmark:      chatResults[0],
						ToolCallBenchmark:  toolCallResults[0],
						CognitionBenchmark: cognitionResults[0],
					})
				}
			}

			// Write results to JSON file
			resultsJSON, err := json.MarshalIndent(allResults, "", "  ")
			if err != nil {
				fmt.Println("Error marshaling results to JSON:", err)
				return
			}

			err = ioutil.WriteFile("benchmark_results.json", resultsJSON, 0644)
			if err != nil {
				fmt.Println("Error writing results to file:", err)
				return
			}

			fmt.Println("\n✅ Orchestration complete. Raw results saved to P-LMv1/benchmark_results.json")
		} else {
			fmt.Println("Skipping benchmark runs. Analyzing existing results...")
			data, err := ioutil.ReadFile("P-LMv1/benchmark_results.json")
			if err != nil {
				fmt.Println("Error reading P-LMv1/benchmark_results.json:", err)
				return
			}
			err = json.Unmarshal(data, &allResults)
			if err != nil {
				fmt.Println("Error unmarshaling benchmark results:", err)
				return
			}
		}

		// --- Analysis ---
		fmt.Println("\n--- Overall Model Analysis (Ranked by Score) ---")

		var scores []benchmark.ModelScore
		for _, result := range allResults {
			score := benchmark.CalculateScore(result)
			scores = append(scores, benchmark.ModelScore{ModelName: result.ModelName, Score: score})
		}

		// Sort models by score in descending order
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].Score > scores[j].Score
		})

		for i, score := range scores {
			fmt.Printf("%d. Model: %s (Score: %.2f)\n", i+1, score.ModelName, score.Score)
		}

		// --- Store Top 3 Models ---
		var top3Models []string
		for i := 0; i < 3 && i < len(scores); i++ {
			top3Models = append(top3Models, scores[i].ModelName)
		}

		top3JSON, err := json.MarshalIndent(top3Models, "", "  ")
		if err != nil {
			fmt.Println("\nError marshaling top 3 models to JSON:", err)
			return
		}

		err = ioutil.WriteFile("P-LMv1/recommended_models.json", top3JSON, 0644)
		if err != nil {
			fmt.Println("\nError writing top 3 models to file:", err)
			return
		}

		fmt.Println("\n✅ Top 3 recommended models saved to P-LMv1/recommended_models.json")
	},
}

func init() {
	orchestratorCmd.Flags().BoolVar(&noRun, "no-run", false, "Skip running benchmarks and only analyze existing results")
	rootCmd.AddCommand(orchestratorCmd)
}
