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
		fmt.Printf("  Version:    %s\n", currentTalosVersion())
		fmt.Printf("  Commit:     %s\n", currentTalosCommit())
		fmt.Printf("  Build Date: %s\n", currentTalosBuildDate())
		fmt.Printf("  Runtime:    %s\n", runtimeDescriptor())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
