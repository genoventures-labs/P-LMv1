package taloscli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const talosHelpTemplate = `TALOS CLI

USAGE
  {{.UseLine}}

{{- if .Long}}
DESCRIPTION
  {{.Long}}

{{- end}}
{{- if .HasAvailableSubCommands}}
AVAILABLE COMMANDS
{{- range .Commands}}
{{- if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }}  {{.Short}}
{{- end}}
{{- end}}

{{- end}}
{{- if .HasAvailableLocalFlags}}
FLAGS
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

{{- end}}
{{- if .HasAvailableInheritedFlags}}
GLOBAL FLAGS
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}

{{- end}}
{{- if .Example}}
EXAMPLES
{{.Example}}

{{- end}}
MORE INFO
  Use "{{.CommandPath}} [command] --help" for more details on a command.
`

var rootCmd = &cobra.Command{
	Use:     "talos",
	Aliases: []string{"personal-llm"},
	Short:   "TALOS CLI interface powered by your Ollama VPS.",
	Long: `talos is a CLI tool to interact with your personal LLM hosted on an Ollama VPS.
It leverages the JIT model router to ensure optimal models are available.`,
	Example: `  talos chat "Summarize latest telemetry"
  talos research run "What changed in X this week?"
  talos benchmark full
  talos pipeline "research run 'What changed in X this week?' ; learn --from-research latest"
  talos "research run 'What changed in X this week?' ; learn --from-research latest"
  talos learn "Store this project note"
  talos learned --last 5
  talos tools admin list
  talos skills list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			chain := strings.TrimSpace(args[0])
			if strings.Contains(chain, ";") || strings.Contains(chain, "|") {
				return runPipelineChain(chain)
			}
		}
		return cmd.Help()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		path := strings.ToLower(strings.TrimSpace(cmd.CommandPath()))
		if strings.Contains(path, " update") || strings.HasSuffix(path, " version") {
			return
		}
		maybeAutoUpdate()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetHelpTemplate(talosHelpTemplate)
	rootCmd.SetUsageTemplate(talosHelpTemplate)
}
