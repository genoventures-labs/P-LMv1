package taloscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	updateModulePath       = "github.com/Thynaptic/P-LMv1"
	updateCmdPath          = "github.com/Thynaptic/P-LMv1/cmd/talos"
	defaultAutoUpdateHours = 24
)

type updateConfig struct {
	Enabled       bool      `json:"enabled"`
	AutoApply     bool      `json:"auto_apply"`
	IntervalHours int       `json:"interval_hours"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	LastSeen      string    `json:"last_seen,omitempty"`
}

var (
	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Check for updates and self-update TALOS.",
	}
	updateCheckCmd = &cobra.Command{
		Use:   "check",
		Short: "Check latest available TALOS version.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runUpdateCheck(true); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		},
	}
	updateApplyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Install latest TALOS binary using Go modules.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runUpdateApply(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		},
	}
	updateAutoCmd = &cobra.Command{
		Use:   "auto",
		Short: "Configure background auto-update checks.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runUpdateAutoConfig(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		},
	}
	updateAutoEnable   bool
	updateAutoDisable  bool
	updateAutoApply    bool
	updateAutoNoApply  bool
	updateAutoInterval time.Duration
)

func init() {
	updateCmd.AddCommand(updateCheckCmd)
	updateCmd.AddCommand(updateApplyCmd)
	updateCmd.AddCommand(updateAutoCmd)

	updateAutoCmd.Flags().BoolVar(&updateAutoEnable, "enable", false, "Enable automatic update checks")
	updateAutoCmd.Flags().BoolVar(&updateAutoDisable, "disable", false, "Disable automatic update checks")
	updateAutoCmd.Flags().BoolVar(&updateAutoApply, "apply", false, "Enable automatic update apply when a newer version is found")
	updateAutoCmd.Flags().BoolVar(&updateAutoNoApply, "no-apply", false, "Disable automatic update apply")
	updateAutoCmd.Flags().DurationVar(&updateAutoInterval, "interval", 0, "Auto-check interval (for example: 12h, 24h)")

	rootCmd.AddCommand(updateCmd)
}

func runUpdateCheck(verbose bool) error {
	latest, err := fetchLatestVersion(8 * time.Second)
	if err != nil {
		return err
	}
	current := currentTalosVersion()
	cmp := compareSemver(current, latest)
	if verbose {
		fmt.Println("TALOS UPDATE CHECK")
		fmt.Printf("  Current: %s\n", current)
		fmt.Printf("  Latest:  %s\n", latest)
	}
	if cmp < 0 {
		fmt.Printf("Update available: %s -> %s\n", current, latest)
		fmt.Println("Run: talos update apply")
		return nil
	}
	if cmp == 0 {
		fmt.Println("TALOS is up to date.")
		return nil
	}
	fmt.Println("Current version is newer than latest module tag.")
	return nil
}

func runUpdateApply() error {
	latest, err := fetchLatestVersion(8 * time.Second)
	if err != nil {
		return err
	}
	fmt.Printf("Installing TALOS %s ...\n", latest)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "go", "install", updateCmdPath+"@latest").CombinedOutput()
	if err != nil {
		return fmt.Errorf("go install failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	binPath := resolveTalosBinPath()
	fmt.Println("Update complete.")
	fmt.Printf("Binary path: %s\n", binPath)
	fmt.Println("Run: talos version")
	return nil
}

func runUpdateAutoConfig() error {
	cfg, err := loadUpdateConfig()
	if err != nil {
		return err
	}
	if updateAutoEnable && updateAutoDisable {
		return errors.New("cannot use --enable and --disable together")
	}
	if updateAutoApply && updateAutoNoApply {
		return errors.New("cannot use --apply and --no-apply together")
	}
	changed := false
	if updateAutoEnable {
		cfg.Enabled = true
		changed = true
	}
	if updateAutoDisable {
		cfg.Enabled = false
		changed = true
	}
	if updateAutoApply {
		cfg.AutoApply = true
		changed = true
	}
	if updateAutoNoApply {
		cfg.AutoApply = false
		changed = true
	}
	if updateAutoInterval > 0 {
		hours := int(updateAutoInterval.Hours())
		if hours < 1 {
			hours = 1
		}
		cfg.IntervalHours = hours
		changed = true
	}
	if changed {
		if err := saveUpdateConfig(cfg); err != nil {
			return err
		}
		fmt.Println("Auto-update configuration updated.")
	}
	fmt.Println("AUTO-UPDATE CONFIG")
	fmt.Printf("  Enabled:       %t\n", cfg.Enabled)
	fmt.Printf("  Auto-Apply:    %t\n", cfg.AutoApply)
	fmt.Printf("  IntervalHours: %d\n", cfg.IntervalHours)
	if !cfg.LastCheckedAt.IsZero() {
		fmt.Printf("  LastChecked:   %s\n", cfg.LastCheckedAt.UTC().Format(time.RFC3339))
	}
	if strings.TrimSpace(cfg.LastSeen) != "" {
		fmt.Printf("  LastSeen:      %s\n", cfg.LastSeen)
	}
	return nil
}

func maybeAutoUpdate() {
	cfg, err := loadUpdateConfig()
	if err != nil || !cfg.Enabled {
		return
	}
	if cfg.IntervalHours <= 0 {
		cfg.IntervalHours = defaultAutoUpdateHours
	}
	if !cfg.LastCheckedAt.IsZero() && time.Since(cfg.LastCheckedAt) < time.Duration(cfg.IntervalHours)*time.Hour {
		return
	}
	latest, err := fetchLatestVersion(4 * time.Second)
	if err != nil {
		return
	}
	cfg.LastCheckedAt = time.Now().UTC()
	cfg.LastSeen = latest
	_ = saveUpdateConfig(cfg)

	current := currentTalosVersion()
	if compareSemver(current, latest) >= 0 {
		return
	}
	fmt.Printf("NOTICE: TALOS update available (%s -> %s)\n", current, latest)
	if !cfg.AutoApply {
		fmt.Println("Run: talos update apply")
		return
	}
	if err := runUpdateApply(); err != nil {
		fmt.Printf("NOTICE: auto-update failed: %v\n", err)
	}
}

func updateConfigPath() string {
	return filepath.Join(".memory", "talos_update_config.json")
}

func loadUpdateConfig() (updateConfig, error) {
	cfg := updateConfig{
		Enabled:       true,
		AutoApply:     false,
		IntervalHours: defaultAutoUpdateHours,
	}
	path := updateConfigPath()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			_ = saveUpdateConfig(cfg)
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.IntervalHours <= 0 {
		cfg.IntervalHours = defaultAutoUpdateHours
	}
	return cfg, nil
}

func saveUpdateConfig(cfg updateConfig) error {
	path := updateConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func fetchLatestVersion(timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "go", "list", "-m", "-json", updateModulePath+"@latest").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed querying latest module version: %w", err)
	}
	var payload struct {
		Version string `json:"Version"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("failed parsing latest module version: %w", err)
	}
	v := strings.TrimSpace(payload.Version)
	if v == "" {
		return "", errors.New("latest module version is empty")
	}
	return v, nil
}

func resolveTalosBinPath() string {
	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		return filepath.Join(gobin, "talos")
	}
	out, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return "talos (path unknown)"
	}
	gopath := strings.TrimSpace(string(out))
	if gopath == "" {
		return "talos (path unknown)"
	}
	return filepath.Join(gopath, "bin", "talos")
}

func compareSemver(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	var out [3]int
	v = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v), "v"))
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(v)
	if len(m) != 4 {
		return out
	}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}
		}
		out[i] = n
	}
	return out
}
