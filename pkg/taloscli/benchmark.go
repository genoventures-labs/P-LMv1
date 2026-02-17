package taloscli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/Thynaptic/P-LMv1/pkg/benchmark/runners"
	"github.com/spf13/cobra"
)

var (
	benchmarkWarmup          bool
	benchmarkLoadConcurrency int
	benchmarkLoadModel       string
	benchmarkFullNoRun       bool
	benchmarkResultsPath     string
	benchmarkRecommendedPath string
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run TALOS model benchmarks.",
	Long:  "Runs model benchmarks against the configured Ollama endpoint for performance, structure, and tool-call readiness.",
}

var benchmarkRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run standard generation benchmark across installed models.",
	Run: func(cmd *cobra.Command, args []string) {
		models, ok := loadBenchmarkModels()
		if !ok {
			return
		}
		results, err := runners.RunStandardBenchmarks(models, benchmarkWarmup)
		if err != nil {
			fmt.Printf("Error running standard benchmarks: %v\n", err)
			return
		}
		printBasicResults("STANDARD BENCHMARK RESULTS", results)
	},
}

var benchmarkChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Run multi-turn chat benchmark across installed models.",
	Run: func(cmd *cobra.Command, args []string) {
		models, ok := loadBenchmarkModels()
		if !ok {
			return
		}
		results, err := runners.RunChatBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running chat benchmarks: %v\n", err)
			return
		}
		printBasicResults("CHAT BENCHMARK RESULTS", results)
	},
}

var benchmarkJSONCmd = &cobra.Command{
	Use:   "json",
	Short: "Run JSON generation benchmark across installed models.",
	Run: func(cmd *cobra.Command, args []string) {
		models, ok := loadBenchmarkModels()
		if !ok {
			return
		}
		results, err := runners.RunJSONBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running JSON benchmarks: %v\n", err)
			return
		}
		fmt.Println("JSON BENCHMARK RESULTS")
		for i, result := range results {
			fmt.Printf("%d) %s\n", i+1, result.ModelName)
			fmt.Printf("  time_to_first_token: %s\n", result.TimeToFirstToken)
			fmt.Printf("  total_time: %s\n", result.TotalTime)
			fmt.Printf("  tokens_per_second: %.2f\n", result.TokensPerSecond)
			fmt.Printf("  total_tokens: %d\n", result.TotalTokens)
			fmt.Printf("  json_valid: %t\n", result.JSONValid)
		}
	},
}

var benchmarkCodegenCmd = &cobra.Command{
	Use:   "codegen",
	Short: "Run code generation benchmark across installed models.",
	Run: func(cmd *cobra.Command, args []string) {
		models, ok := loadBenchmarkModels()
		if !ok {
			return
		}
		results, err := runners.RunCodegenBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running codegen benchmarks: %v\n", err)
			return
		}
		printBasicResults("CODEGEN BENCHMARK RESULTS", results)
	},
}

var benchmarkToolcallCmd = &cobra.Command{
	Use:   "toolcall",
	Short: "Run tool-call schema compliance benchmark across installed models.",
	Run: func(cmd *cobra.Command, args []string) {
		models, ok := loadBenchmarkModels()
		if !ok {
			return
		}
		results, err := runners.RunToolCallBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running tool-call benchmarks: %v\n", err)
			return
		}
		fmt.Println("TOOLCALL BENCHMARK RESULTS")
		for i, result := range results {
			fmt.Printf("%d) %s\n", i+1, result.ModelName)
			fmt.Printf("  time_to_first_token: %s\n", result.TimeToFirstToken)
			fmt.Printf("  total_time: %s\n", result.TotalTime)
			fmt.Printf("  tokens_per_second: %.2f\n", result.TokensPerSecond)
			fmt.Printf("  tool_call_valid: %t\n", result.ToolCallValid)
			if result.ToolName != "" {
				fmt.Printf("  tool_name: %s\n", result.ToolName)
			}
		}
	},
}

var benchmarkCognitionCmd = &cobra.Command{
	Use:   "cognition",
	Short: "Run cognition structure benchmark across installed models.",
	Run: func(cmd *cobra.Command, args []string) {
		models, ok := loadBenchmarkModels()
		if !ok {
			return
		}
		results, err := runners.RunCognitionBenchmarks(models)
		if err != nil {
			fmt.Printf("Error running cognition benchmarks: %v\n", err)
			return
		}
		fmt.Println("COGNITION BENCHMARK RESULTS")
		for i, result := range results {
			fmt.Printf("%d) %s\n", i+1, result.ModelName)
			fmt.Printf("  time_to_first_token: %s\n", result.TimeToFirstToken)
			fmt.Printf("  total_time: %s\n", result.TotalTime)
			fmt.Printf("  tokens_per_second: %.2f\n", result.TokensPerSecond)
			fmt.Printf("  structured_pass: %t\n", result.StructuredPass)
			fmt.Printf("  section_score: %.2f\n", result.SectionScore)
		}
	},
}

var benchmarkLoadCmd = &cobra.Command{
	Use:   "load",
	Short: "Run concurrent load test for one model.",
	Run: func(cmd *cobra.Command, args []string) {
		if benchmarkLoadModel == "" {
			fmt.Println("Please specify --model for load testing.")
			return
		}
		fmt.Printf("Running load test on %s with concurrency=%d\n", benchmarkLoadModel, benchmarkLoadConcurrency)
		var wg sync.WaitGroup
		resultsChan := make(chan *benchmark.BenchmarkResult, benchmarkLoadConcurrency)
		startTime := time.Now()
		for i := 0; i < benchmarkLoadConcurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := benchmark.RunBenchmark(context.Background(), benchmarkLoadModel, "What is the meaning of life?")
				if err != nil {
					fmt.Printf("Load worker error: %v\n", err)
					return
				}
				resultsChan <- res
			}()
		}
		wg.Wait()
		close(resultsChan)
		totalTime := time.Since(startTime)
		totalTokens := 0
		for result := range resultsChan {
			totalTokens += result.TotalTokens
		}
		fmt.Println("LOAD TEST RESULTS")
		fmt.Printf("  model: %s\n", benchmarkLoadModel)
		fmt.Printf("  concurrency: %d\n", benchmarkLoadConcurrency)
		fmt.Printf("  total_time: %s\n", totalTime)
		fmt.Printf("  total_tokens: %d\n", totalTokens)
		if totalTime.Seconds() > 0 {
			fmt.Printf("  combined_tokens_per_second: %.2f\n", float64(totalTokens)/totalTime.Seconds())
		}
	},
}

var benchmarkFullCmd = &cobra.Command{
	Use:   "full",
	Short: "Run full benchmark suite and generate ranked recommendations.",
	Run: func(cmd *cobra.Command, args []string) {
		var allResults []benchmark.ModelBenchmarkResults
		if !benchmarkFullNoRun {
			models, ok := loadBenchmarkModels()
			if !ok {
				return
			}
			for _, model := range models {
				fmt.Printf("\nRunning full suite for %s\n", model.Name)
				standardResults, err := runners.RunStandardBenchmarks([]benchmark.OllamaModel{model}, benchmarkWarmup)
				if err != nil {
					fmt.Printf("Standard benchmark failed for %s: %v\n", model.Name, err)
					continue
				}
				jsonResults, err := runners.RunJSONBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("JSON benchmark failed for %s: %v\n", model.Name, err)
					continue
				}
				codegenResults, err := runners.RunCodegenBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Codegen benchmark failed for %s: %v\n", model.Name, err)
					continue
				}
				chatResults, err := runners.RunChatBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Chat benchmark failed for %s: %v\n", model.Name, err)
					continue
				}
				toolCallResults, err := runners.RunToolCallBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Toolcall benchmark failed for %s: %v\n", model.Name, err)
					continue
				}
				cognitionResults, err := runners.RunCognitionBenchmarks([]benchmark.OllamaModel{model})
				if err != nil {
					fmt.Printf("Cognition benchmark failed for %s: %v\n", model.Name, err)
					continue
				}
				if len(standardResults) == 0 || len(jsonResults) == 0 || len(codegenResults) == 0 || len(chatResults) == 0 || len(toolCallResults) == 0 || len(cognitionResults) == 0 {
					continue
				}
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
			if err := writeJSON(benchmarkResultsPath, allResults); err != nil {
				fmt.Printf("Error writing benchmark results: %v\n", err)
				return
			}
			fmt.Printf("Saved benchmark results: %s\n", benchmarkResultsPath)
		} else {
			if err := readJSON(benchmarkResultsPath, &allResults); err != nil {
				fmt.Printf("Error loading benchmark results: %v\n", err)
				return
			}
		}

		var scores []benchmark.ModelScore
		for _, result := range allResults {
			scores = append(scores, benchmark.ModelScore{
				ModelName: result.ModelName,
				Score:     benchmark.CalculateScore(result),
			})
		}
		sort.Slice(scores, func(i, j int) bool { return scores[i].Score > scores[j].Score })
		fmt.Println("\nFULL BENCHMARK RANKING")
		for i, s := range scores {
			fmt.Printf("%d) %s (score=%.2f)\n", i+1, s.ModelName, s.Score)
		}

		top := make([]string, 0, 3)
		for i := 0; i < 3 && i < len(scores); i++ {
			top = append(top, scores[i].ModelName)
		}
		if err := writeJSON(benchmarkRecommendedPath, top); err != nil {
			fmt.Printf("Error writing recommended models: %v\n", err)
			return
		}
		fmt.Printf("Saved recommended models: %s\n", benchmarkRecommendedPath)
	},
}

func loadBenchmarkModels() ([]benchmark.OllamaModel, bool) {
	fmt.Println("Loading installed Ollama models...")
	models, err := benchmark.GetOllamaModels()
	if err != nil {
		fmt.Printf("Error getting models: %v\n", err)
		return nil, false
	}
	if len(models) == 0 {
		fmt.Println("No models found.")
		return nil, false
	}
	for _, model := range models {
		fmt.Printf("- %s (%s)\n", model.Name, model.Size)
	}
	return models, true
}

func printBasicResults(header string, results []benchmark.BenchmarkResult) {
	fmt.Println(header)
	for i, result := range results {
		fmt.Printf("%d) %s\n", i+1, result.ModelName)
		fmt.Printf("  time_to_first_token: %s\n", result.TimeToFirstToken)
		fmt.Printf("  total_time: %s\n", result.TotalTime)
		fmt.Printf("  tokens_per_second: %.2f\n", result.TokensPerSecond)
		fmt.Printf("  total_tokens: %d\n", result.TotalTokens)
	}
}

func writeJSON(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func readJSON(path string, dst interface{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func init() {
	benchmarkRunCmd.Flags().BoolVar(&benchmarkWarmup, "warmup", false, "Run model warmup before benchmark")

	benchmarkLoadCmd.Flags().IntVarP(&benchmarkLoadConcurrency, "concurrency", "c", 4, "Concurrent requests")
	benchmarkLoadCmd.Flags().StringVar(&benchmarkLoadModel, "model", "", "Model tag for load test")

	benchmarkFullCmd.Flags().BoolVar(&benchmarkWarmup, "warmup", false, "Run model warmup before benchmark")
	benchmarkFullCmd.Flags().BoolVar(&benchmarkFullNoRun, "no-run", false, "Skip running benchmarks and analyze existing results")
	benchmarkFullCmd.Flags().StringVar(&benchmarkResultsPath, "results", "benchmark_results.json", "Benchmark results JSON path")
	benchmarkFullCmd.Flags().StringVar(&benchmarkRecommendedPath, "recommended", "recommended_models.json", "Recommended models JSON path")

	benchmarkCmd.AddCommand(benchmarkRunCmd)
	benchmarkCmd.AddCommand(benchmarkChatCmd)
	benchmarkCmd.AddCommand(benchmarkJSONCmd)
	benchmarkCmd.AddCommand(benchmarkCodegenCmd)
	benchmarkCmd.AddCommand(benchmarkToolcallCmd)
	benchmarkCmd.AddCommand(benchmarkCognitionCmd)
	benchmarkCmd.AddCommand(benchmarkLoadCmd)
	benchmarkCmd.AddCommand(benchmarkFullCmd)
	rootCmd.AddCommand(benchmarkCmd)
}
