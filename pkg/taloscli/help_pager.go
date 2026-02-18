package taloscli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	helpPageCommandSize = 10
	helpPageExampleSize = 3
)

var helpStatePath = filepath.Join(".memory", "help_pagination_state.json")

type helpPaginationState struct {
	CommandOffset int `json:"command_offset"`
	ExampleOffset int `json:"example_offset"`
}

type helpCommandRow struct {
	Name        string
	Description string
}

func renderRootHelpPage(w io.Writer, reset bool) {
	rows := collectRootHelpRows()
	examples := collectRootHelpExamples()

	state := helpPaginationState{}
	if !reset {
		loaded, err := loadHelpPaginationState()
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(w, "No active help page session. Showing first page.")
				reset = true
			} else {
				fmt.Fprintf(w, "Error loading help pagination state: %v\n", err)
				return
			}
		} else {
			state = loaded
		}
	}
	if reset {
		state = helpPaginationState{}
	}

	cmdStart := clampNonNegative(state.CommandOffset)
	exStart := clampNonNegative(state.ExampleOffset)
	if cmdStart >= len(rows) && exStart >= len(examples) {
		clearHelpPaginationState()
		fmt.Fprintln(w, "No more help pages. Run \"talos --help\" to restart from page 1.")
		return
	}

	cmdEnd := minInt(cmdStart+helpPageCommandSize, len(rows))
	exEnd := minInt(exStart+helpPageExampleSize, len(examples))

	fmt.Fprintln(w, "TALOS CLI HELP")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "USAGE")
	fmt.Fprintln(w, "  talos [command] [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "COMMANDS")
	renderHelpCommandsTable(w, rows[cmdStart:cmdEnd])
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXAMPLES")
	if exStart >= len(examples) || len(examples[exStart:exEnd]) == 0 {
		fmt.Fprintln(w, "  [none]")
	} else {
		for i, ex := range examples[exStart:exEnd] {
			fmt.Fprintf(w, "  %d. %s\n", exStart+i+1, ex)
		}
	}

	hasMore := cmdEnd < len(rows) || exEnd < len(examples)
	if hasMore {
		next := helpPaginationState{CommandOffset: cmdEnd, ExampleOffset: exEnd}
		if err := saveHelpPaginationState(next); err != nil {
			fmt.Fprintf(w, "\nWarning: failed to persist help page state: %v\n", err)
			return
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "MORE")
		fmt.Fprintf(w, "  commands_shown: %d/%d\n", cmdEnd, len(rows))
		fmt.Fprintf(w, "  examples_shown: %d/%d\n", exEnd, len(examples))
		fmt.Fprintln(w, "  run: talos /next")
		return
	}

	clearHelpPaginationState()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "END")
	fmt.Fprintln(w, "  All help pages shown. Run \"talos --help\" to restart.")
}

func collectRootHelpRows() []helpCommandRow {
	cmds := rootCmd.Commands()
	rows := make([]helpCommandRow, 0, len(cmds))
	for _, c := range cmds {
		if c == nil || c.Hidden || !c.IsAvailableCommand() {
			continue
		}
		name := strings.TrimSpace(c.Name())
		if name == "" {
			continue
		}
		if name == "next" {
			name = "next (/next)"
		}
		rows = append(rows, helpCommandRow{
			Name:        name,
			Description: strings.TrimSpace(c.Short),
		})
	}
	return rows
}

func collectRootHelpExamples() []string {
	lines := strings.Split(rootCmd.Example, "\n")
	examples := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		examples = append(examples, line)
	}
	return examples
}

func renderHelpCommandsTable(w io.Writer, rows []helpCommandRow) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "  [none]")
		return
	}
	nameWidth := len("COMMAND")
	for _, r := range rows {
		if len(r.Name) > nameWidth {
			nameWidth = len(r.Name)
		}
	}
	fmt.Fprintf(w, "  %-*s | %s\n", nameWidth, "COMMAND", "DESCRIPTION")
	fmt.Fprintf(w, "  %s-+-%s\n", strings.Repeat("-", nameWidth), strings.Repeat("-", 60))
	for _, r := range rows {
		desc := r.Description
		if desc == "" {
			desc = "n/a"
		}
		fmt.Fprintf(w, "  %-*s | %s\n", nameWidth, r.Name, desc)
	}
}

func loadHelpPaginationState() (helpPaginationState, error) {
	blob, err := os.ReadFile(helpStatePath)
	if err != nil {
		return helpPaginationState{}, err
	}
	var st helpPaginationState
	if err := json.Unmarshal(blob, &st); err != nil {
		return helpPaginationState{}, err
	}
	return st, nil
}

func saveHelpPaginationState(st helpPaginationState) error {
	if err := os.MkdirAll(filepath.Dir(helpStatePath), 0o755); err != nil {
		return err
	}
	blob, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(helpStatePath, append(blob, '\n'), 0o644)
}

func clearHelpPaginationState() {
	_ = os.Remove(helpStatePath)
}

func clampNonNegative(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
