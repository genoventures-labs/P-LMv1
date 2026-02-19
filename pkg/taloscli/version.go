package taloscli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show TALOS version and build metadata.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TALOS VERSION")
		fmt.Println()
		fmt.Println("COMMAND")
		fmt.Println("  talos version")
		fmt.Println()
		fmt.Println("STATUS")
		fmt.Println("  SUCCESS")
		fmt.Println()
		fmt.Println("RESULTS")
		fmt.Printf("  version:    %s\n", currentTalosVersion())
		fmt.Printf("  commit:     %s\n", currentTalosCommit())
		fmt.Printf("  build_date: %s\n", currentTalosBuildDate())
		fmt.Printf("  runtime:    %s\n", runtimeDescriptor())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
