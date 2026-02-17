package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "personal-llm",
	Short: "A personal LLM interface powered by your Ollama VPS.",
	Long: `personal-llm is a CLI tool to interact with your personal LLM hosted on an Ollama VPS.
It leverages the JIT model router to ensure optimal models are available.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Personal LLM Interface")
		fmt.Println("Use 'personal-llm --help' for more information.")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
