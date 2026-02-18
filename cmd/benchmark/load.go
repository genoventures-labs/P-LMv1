package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/spf13/cobra"
)

var concurrency int
var loadTestModel string

var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Run a concurrent load test on a model.",
	Long:  `This command runs a load test by sending multiple concurrent requests to a single model.`,
	Run: func(cmd *cobra.Command, args []string) {
		if loadTestModel == "" {
			fmt.Println("Please specify a model to test with the --model flag.")
			return
		}

		fmt.Printf("Running load test on model '%s' with %d concurrent requests...\n", loadTestModel, concurrency)

		var wg sync.WaitGroup
		resultsChan := make(chan *benchmark.BenchmarkResult, concurrency)
		startTime := time.Now()

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				prompt := "What is the meaning of life?"
				res, err := benchmark.RunBenchmark(context.Background(), loadTestModel, prompt)
				if err != nil {
					fmt.Printf("Error in one of the benchmark runs: %v\n", err)
					return
				}
				resultsChan <- res
			}()
		}

		wg.Wait()
		close(resultsChan)
		totalTime := time.Since(startTime)

		var totalTokens int
		var totalTokensPerSecond float64
		var results []*benchmark.BenchmarkResult

		for result := range resultsChan {
			results = append(results, result)
			totalTokens += result.TotalTokens
			totalTokensPerSecond += result.TokensPerSecond
		}

		fmt.Println("\n--- Load Test Results ---")
		fmt.Printf("Model: %s\n", loadTestModel)
		fmt.Printf("Concurrency Level: %d\n", concurrency)
		fmt.Printf("Total Time Elapsed: %s\n", totalTime)
		fmt.Printf("Total Tokens Generated: %d\n", totalTokens)
		if totalTime.Seconds() > 0 {
			fmt.Printf("Combined Tokens per Second: %.2f\n", float64(totalTokens)/totalTime.Seconds())
		}
		if len(results) > 0 {
			fmt.Printf("Average Tokens per Second (per request): %.2f\n", totalTokensPerSecond/float64(len(results)))
		}
	},
}

func init() {
	loadCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 4, "Number of concurrent requests to send")
	loadCmd.Flags().StringVar(&loadTestModel, "model", "", "The model to run the load test against")
	rootCmd.AddCommand(loadCmd)
}
