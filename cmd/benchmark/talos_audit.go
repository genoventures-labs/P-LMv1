package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/benchmark"
	"github.com/spf13/cobra"
)

var talosAuditRequirePreflight bool
var talosAuditCreateSkill bool
var talosAuditAdminWrites bool
var talosAuditReportPath string

var talosAuditCmd = &cobra.Command{
	Use:   "talos-audit",
	Short: "Run a TALOS functionality benchmark audit report.",
	Long:  "Runs a capability matrix across TALOS core integrations and writes JSON report with pass/fail/skip plus remediation hints.",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		report := benchmark.RunTALOSAudit(ctx, benchmark.TALOSAuditOptions{
			RequirePreflight: talosAuditRequirePreflight,
			CreateSkill:      talosAuditCreateSkill,
			RunAdminWrites:   talosAuditAdminWrites,
			ReportPath:       strings.TrimSpace(talosAuditReportPath),
		})

		fmt.Printf("TALOS Audit Score: %.2f (%d pass, %d fail, %d skip)\n", report.Score, report.Pass, report.Fail, report.Skip)
		for _, c := range report.Checks {
			fmt.Printf("- [%s] %s (%dms)\n", strings.ToUpper(string(c.Status)), c.Name, c.DurationMS)
			if strings.TrimSpace(c.Detail) != "" {
				fmt.Printf("  detail: %s\n", strings.TrimSpace(c.Detail))
			}
			if c.Status == benchmark.TALOSCheckFail && strings.TrimSpace(c.Remediation) != "" {
				fmt.Printf("  remediation: %s\n", strings.TrimSpace(c.Remediation))
			}
		}

		path := strings.TrimSpace(talosAuditReportPath)
		if path == "" {
			path = "talos_benchmark_report.json"
		}
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling report: %v\n", err)
			return
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			fmt.Printf("Error writing report: %v\n", err)
			return
		}
		fmt.Printf("Report saved to %s\n", path)
	},
}

func init() {
	talosAuditCmd.Flags().BoolVar(&talosAuditRequirePreflight, "require-preflight", true, "Require /skills/preflight to allow skill creation checks")
	talosAuditCmd.Flags().BoolVar(&talosAuditCreateSkill, "create-skill", true, "Exercise user skill creation path")
	talosAuditCmd.Flags().BoolVar(&talosAuditAdminWrites, "admin-writes", false, "Run admin write checks (toolgen generate)")
	talosAuditCmd.Flags().StringVar(&talosAuditReportPath, "report", "talos_benchmark_report.json", "Output report path")
	rootCmd.AddCommand(talosAuditCmd)
}
