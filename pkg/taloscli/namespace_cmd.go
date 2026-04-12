package taloscli

import (
	"fmt"
	"strings"

	"github.com/Thynaptic/P-LMv1/pkg/state"
	"github.com/spf13/cobra"
)

var namespaceCmd = &cobra.Command{
	Use:   "namespace",
	Short: "Manage active workspace namespace scope.",
}

var namespaceClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear persisted active workspace namespace.",
	Run: func(cmd *cobra.Command, args []string) {
		sm, err := state.NewManager()
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Error initializing state manager: %v\n", err)
			return
		}
		sm.SetActiveNamespace("")
		if err := sm.Save(); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Error clearing namespace: %v\n", err)
			return
		}
		fmt.Fprintln(cmd.OutOrStdout(), "NAMESPACE CLEARED")
	},
}

var namespaceShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current persisted active workspace namespace.",
	Run: func(cmd *cobra.Command, args []string) {
		sm, err := state.NewManager()
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Error initializing state manager: %v\n", err)
			return
		}
		ns := strings.TrimSpace(sm.ActiveNamespace())
		if ns == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "NAMESPACE: none")
			return
		}
		fmt.Fprintf(cmd.OutOrStdout(), "NAMESPACE: %s\n", ns)
	},
}

func init() {
	namespaceCmd.AddCommand(namespaceShowCmd)
	namespaceCmd.AddCommand(namespaceClearCmd)
	rootCmd.AddCommand(namespaceCmd)
}
