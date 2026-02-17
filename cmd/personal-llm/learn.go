package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/Thynaptic/P-LMv1/pkg/memory"
	"github.com/Thynaptic/P-LMv1/pkg/rag"
	"github.com/spf13/cobra"
)

var learnFile string
var learnDir string
var learnRecursive bool
var learnExtensions string
var learnChunkChars int
var learnChunkOverlap int

var learnCmd = &cobra.Command{
	Use:   "learn [text]",
	Short: "Add knowledge to your personal LLM's memory.",
	Long:  `This command allows you to teach your personal LLM new information by adding text, a file, or indexing a directory of documents into its persistent knowledge base.`,
	Run: func(cmd *cobra.Command, args []string) {
		mm, err := memory.NewMemoryManager()
		if err != nil {
			fmt.Printf("Error initializing memory manager: %v\n", err)
			return
		}

		if learnDir != "" {
			opts := rag.DefaultIndexOptions()
			opts.Recursive = learnRecursive
			opts.MaxChunkChars = learnChunkChars
			opts.ChunkOverlap = learnChunkOverlap
			opts.Extensions = rag.ParseExtensionsCSV(learnExtensions)
			if len(opts.Extensions) == 0 {
				opts.Extensions = rag.DefaultIndexOptions().Extensions
			}

			fmt.Printf("Indexing directory: %s\n", learnDir)
			stats, err := rag.IndexDirectory(mm, learnDir, opts)
			if err != nil {
				fmt.Printf("Error indexing directory: %v\n", err)
				return
			}
			fmt.Printf("✅ Directory indexing complete. Files scanned: %d, indexed: %d, chunks: %d, unsupported: %d, binary skipped: %d, parse errors: %d\n",
				stats.FilesScanned, stats.FilesIndexed, stats.ChunksIndexed, stats.SkippedUnsupported, stats.SkippedBinary, stats.ParseErrors)
			return
		}

		var content string
		if learnFile != "" {
			data, err := ioutil.ReadFile(learnFile)
			if err != nil {
				fmt.Printf("Error reading file %s: %v\n", learnFile, err)
				return
			}
			content = string(data)
			fmt.Printf("Learning from file: %s\n", learnFile)
		} else if len(args) > 0 {
			content = strings.Join(args, " ")
			fmt.Println("Learning from provided text.")
		} else {
			fmt.Println("Please provide text to learn or use the --file flag.")
			return
		}

		err = mm.AddKnowledge(content, nil)
		if err != nil {
			fmt.Printf("Error adding knowledge to memory: %v\n", err)
			return
		}

		fmt.Println("✅ Knowledge successfully added to memory.")
	},
}

func init() {
	learnCmd.Flags().StringVarP(&learnFile, "file", "f", "", "Path to a file to learn from")
	learnCmd.Flags().StringVarP(&learnDir, "dir", "d", "", "Path to a directory of documents to index")
	learnCmd.Flags().BoolVarP(&learnRecursive, "recursive", "r", true, "Recursively index subdirectories when using --dir")
	learnCmd.Flags().IntVar(&learnChunkChars, "chunk-chars", 1200, "Maximum characters per indexed chunk when using --dir")
	learnCmd.Flags().IntVar(&learnChunkOverlap, "chunk-overlap", 200, "Overlap between chunks when using --dir")
	learnCmd.Flags().StringVar(&learnExtensions, "extensions", rag.ExtensionsToCSV(rag.DefaultIndexOptions().Extensions), "Comma-separated extensions to index with --dir (e.g. .pdf,.md,.txt)")
	rootCmd.AddCommand(learnCmd)
}
