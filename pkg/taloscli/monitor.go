package taloscli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	pathBlockStart = "# >>> TALOS PATH >>>"
	pathBlockEnd   = "# <<< TALOS PATH <<<"
)

var (
	monitorBinDir    string
	monitorShell     string
	monitorNoProfile bool
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Terminal integration utilities for TALOS.",
}

var monitorInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install TALOS to a PATH directory and wire shell profile.",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("determine home directory: %w", err)
		}
		binDir, err := expandHome(monitorBinDir, home)
		if err != nil {
			return err
		}
		if strings.TrimSpace(binDir) == "" {
			binDir = filepath.Join(home, ".local", "bin")
		}
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return fmt.Errorf("create bin directory %s: %w", binDir, err)
		}

		repoRoot, err := findRepoRoot()
		if err != nil {
			return err
		}

		target := filepath.Join(binDir, "talos")
		build := exec.Command("go", "build", "-o", target, "./cmd/talos")
		build.Dir = repoRoot
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			return fmt.Errorf("build talos binary: %w", err)
		}

		updatedFiles := make([]string, 0, 2)
		if !monitorNoProfile {
			rcFiles, err := shellProfileFiles(strings.TrimSpace(monitorShell), home)
			if err != nil {
				return err
			}
			for _, rc := range rcFiles {
				updated, err := ensurePathExportBlock(rc, binDir)
				if err != nil {
					return err
				}
				if updated {
					updatedFiles = append(updatedFiles, rc)
				}
			}
		}

		fmt.Printf("Installed TALOS binary: %s\n", target)
		if len(updatedFiles) > 0 {
			fmt.Println("Updated shell profile files:")
			for _, f := range updatedFiles {
				fmt.Printf("  %s\n", f)
			}
		}
		fmt.Println("Open a new terminal (or source your profile) and run: talos --help")
		return nil
	},
}

var monitorUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove TALOS terminal integration and installed binary.",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("determine home directory: %w", err)
		}
		binDir, err := expandHome(monitorBinDir, home)
		if err != nil {
			return err
		}
		if strings.TrimSpace(binDir) == "" {
			binDir = filepath.Join(home, ".local", "bin")
		}

		target := filepath.Join(binDir, "talos")
		removedBinary := false
		if err := os.Remove(target); err == nil {
			removedBinary = true
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove talos binary %s: %w", target, err)
		}

		updatedFiles := make([]string, 0, 2)
		if !monitorNoProfile {
			rcFiles, err := shellProfileFiles(strings.TrimSpace(monitorShell), home)
			if err != nil {
				return err
			}
			for _, rc := range rcFiles {
				updated, err := removePathExportBlock(rc)
				if err != nil {
					return err
				}
				if updated {
					updatedFiles = append(updatedFiles, rc)
				}
			}
		}

		if removedBinary {
			fmt.Printf("Removed TALOS binary: %s\n", target)
		} else {
			fmt.Printf("TALOS binary not found: %s\n", target)
		}
		if len(updatedFiles) > 0 {
			fmt.Println("Updated shell profile files:")
			for _, f := range updatedFiles {
				fmt.Printf("  %s\n", f)
			}
		}
		fmt.Println("Open a new terminal (or source your profile) to apply PATH changes.")
		return nil
	},
}

var monitorStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show terminal integration status for TALOS.",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := exec.LookPath("talos")
		if err != nil {
			fmt.Println("talos in PATH: no")
		} else {
			fmt.Println("talos in PATH: yes")
			fmt.Printf("resolved binary: %s\n", path)
		}
		fmt.Printf("active shell: %s\n", strings.TrimSpace(os.Getenv("SHELL")))
		fmt.Printf("PATH contains ~/.local/bin: %t\n", pathContainsLocalBin(os.Getenv("PATH")))
		return nil
	},
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine working directory: %w", err)
	}
	dir := wd
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "cmd", "talos", "main.go")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find TALOS repository root; run this from inside the repository")
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func expandHome(path string, home string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", nil
	}
	if p == "~" {
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

func shellProfileFiles(shell string, home string) ([]string, error) {
	sh := shell
	if sh == "" || sh == "auto" {
		sh = strings.ToLower(strings.TrimSpace(filepath.Base(os.Getenv("SHELL"))))
	}
	switch sh {
	case "bash":
		return []string{filepath.Join(home, ".bashrc")}, nil
	case "zsh":
		return []string{filepath.Join(home, ".zshrc")}, nil
	default:
		return nil, fmt.Errorf("unsupported shell %q (supported: auto, bash, zsh)", sh)
	}
}

func ensurePathExportBlock(rcFile string, binDir string) (bool, error) {
	block := fmt.Sprintf("%s\nexport PATH=\"%s:$PATH\"\n%s\n", pathBlockStart, binDir, pathBlockEnd)

	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read profile %s: %w", rcFile, err)
	}
	text := string(content)
	if strings.Contains(text, pathBlockStart) && strings.Contains(text, pathBlockEnd) {
		return false, nil
	}

	var out strings.Builder
	if strings.TrimSpace(text) != "" {
		out.WriteString(strings.TrimRight(text, "\n"))
		out.WriteString("\n\n")
	}
	out.WriteString(block)
	if err := os.WriteFile(rcFile, []byte(out.String()), 0o644); err != nil {
		return false, fmt.Errorf("write profile %s: %w", rcFile, err)
	}
	return true, nil
}

func removePathExportBlock(rcFile string) (bool, error) {
	content, err := os.ReadFile(rcFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read profile %s: %w", rcFile, err)
	}
	text := string(content)
	start := strings.Index(text, pathBlockStart)
	end := strings.Index(text, pathBlockEnd)
	if start == -1 || end == -1 || end < start {
		return false, nil
	}
	end += len(pathBlockEnd)

	newText := text[:start] + text[end:]
	newText = strings.TrimLeft(newText, "\n")
	newText = strings.TrimRight(newText, " \t\n") + "\n"

	if err := os.WriteFile(rcFile, []byte(newText), 0o644); err != nil {
		return false, fmt.Errorf("write profile %s: %w", rcFile, err)
	}
	return true, nil
}

func pathContainsLocalBin(pathValue string) bool {
	for _, part := range strings.Split(pathValue, ":") {
		if strings.HasSuffix(strings.TrimSpace(part), "/.local/bin") {
			return true
		}
	}
	return false
}

func init() {
	monitorInstallCmd.Flags().StringVar(&monitorBinDir, "bin-dir", "~/.local/bin", "directory where talos binary is installed")
	monitorInstallCmd.Flags().StringVar(&monitorShell, "shell", "auto", "shell profile to update (auto|bash|zsh)")
	monitorInstallCmd.Flags().BoolVar(&monitorNoProfile, "no-profile", false, "skip shell profile updates")
	monitorUninstallCmd.Flags().StringVar(&monitorBinDir, "bin-dir", "~/.local/bin", "directory where talos binary is installed")
	monitorUninstallCmd.Flags().StringVar(&monitorShell, "shell", "auto", "shell profile to update (auto|bash|zsh)")
	monitorUninstallCmd.Flags().BoolVar(&monitorNoProfile, "no-profile", false, "skip shell profile updates")
	monitorCmd.AddCommand(monitorInstallCmd)
	monitorCmd.AddCommand(monitorUninstallCmd)
	monitorCmd.AddCommand(monitorStatusCmd)
	rootCmd.AddCommand(monitorCmd)
}
