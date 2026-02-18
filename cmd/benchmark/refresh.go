package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/Thynaptic/P-LMv1/pkg/registry"
	"github.com/Thynaptic/P-LMv1/pkg/remote"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh the models on the VPS based on the latest registry data and benchmarks.",
	Long:  `The refresh command dynamically updates the models on the Ollama VPS. It fetches all models from the registry, identifies the best models for the system based on specs and performance, pulls the top 3, and removes any non-recommended models.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting the JIT model refresh process...")

		fmt.Println("Fetching models from the registry...")
		registryModels, err := registry.GetModels()
		if err != nil {
			fmt.Printf("Error fetching models from registry: %v\n", err)
			return
		}
		fmt.Printf("Found %d models in the registry.\n", len(registryModels))

		fmt.Println("Filtering models based on system specs (16GB RAM, max 40% for single model)...")
		vpsRamGB := 16.0
		filteredModels, err := registry.FilterModels(registryModels, vpsRamGB)
		if err != nil {
			fmt.Printf("Error filtering models: %v\n", err)
			return
		}
		fmt.Printf("%d models passed the filter.\n", len(filteredModels))

		// Score the filtered models
		fmt.Println("Scoring models based on popularity and benchmark results...")
		scoredModels, err := registry.ScoreModels(filteredModels, "benchmark_results.json", vpsRamGB)
		if err != nil {
			fmt.Printf("Error scoring models: %v\n", err)
			return
		}

		fmt.Println("\n--- Top Recommended Models ---")
		var top3RecommendedTags []string
		seen := make(map[string]bool)
		for _, scoredModel := range scoredModels {
			if len(top3RecommendedTags) >= 3 {
				break
			}
			// Registry now returns full pull tags in Labels; prefer those for model sync.
			pullTag := pickPullTag(scoredModel.RegistryModel)
			if pullTag == "" {
				continue
			}
			if seen[pullTag] {
				continue
			}
			seen[pullTag] = true
			fmt.Printf("%d. %s (from %s, Score: %.2f, Pulls: %d)\n", len(top3RecommendedTags)+1, pullTag, scoredModel.ModelIdentifier, scoredModel.Score, scoredModel.Pulls)
			top3RecommendedTags = append(top3RecommendedTags, pullTag)
		}

		// Save top 3 recommended models to a file
		top3JSON, err := json.MarshalIndent(top3RecommendedTags, "", "  ")
		if err != nil {
			fmt.Println("\nError marshaling top 3 recommended models to JSON:", err)
			return
		}

		err = ioutil.WriteFile("recommended_models.json", top3JSON, 0644)
		if err != nil {
			fmt.Println("\nError writing top 3 recommended models to file:", err)
			return
		}
		fmt.Println("\n✅ Top 3 recommended models (pull tags) saved to recommended_models.json")

		// --- Sync Models on VPS ---
		fmt.Println("\nSyncing models on the VPS...")
		installedModels, err := remote.ListInstalledModelsRemote()
		if err != nil {
			fmt.Printf("Error getting installed models from remote: %v\n", err)
			return
		}

		installedModelMap := make(map[string]bool)
		for _, model := range installedModels {
			installedModelMap[canonicalModelTag(model.Name)] = true
		}

		top3RecommendedMap := make(map[string]bool)
		for _, modelTag := range top3RecommendedTags {
			top3RecommendedMap[canonicalModelTag(modelTag)] = true
		}

		// Delete non-recommended models
		for _, installedModel := range installedModels {
			if !top3RecommendedMap[canonicalModelTag(installedModel.Name)] {
				fmt.Printf("Deleting non-recommended model: %s\n", installedModel.Name)
				if err := remote.DeleteModelRemote(installedModel.Name); err != nil {
					fmt.Printf("Error deleting model %s from remote: %v\n", installedModel.Name, err)
				}
			}
		}

		// Pull top 3 recommended models if not already installed
		for _, modelTag := range top3RecommendedTags {
			if !installedModelMap[canonicalModelTag(modelTag)] {
				fmt.Printf("Pulling recommended model: %s\n", modelTag)
				if err := remote.PullModelRemote(modelTag); err != nil {
					fmt.Printf("Error pulling model %s to remote: %v\n", modelTag, err)
				}
			}
		}

		fmt.Println("\n✅ Model synchronization complete.")
	},
}

func pickPullTag(m registry.RegistryModel) string {
	for _, label := range m.Labels {
		tag := strings.TrimSpace(label)
		if tag != "" {
			return tag
		}
	}
	if m.ModelIdentifier != "" {
		return m.ModelIdentifier
	}
	return m.ModelName
}

func canonicalModelTag(tag string) string {
	normalized := strings.TrimSpace(strings.ToLower(tag))
	if normalized == "" {
		return normalized
	}
	if !strings.Contains(normalized, ":") {
		return normalized + ":latest"
	}
	return normalized
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}
