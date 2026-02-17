package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Thynaptic/P-LMv1/pkg/tools"
	"github.com/spf13/cobra"
)

var adminCreateClientID string
var adminRotateClientID string
var adminDeleteClientID string
var adminDeleteYes bool
var adminPluginID string
var adminPluginSource string
var adminPluginManifestPath string
var adminToolgenName string
var adminToolgenDescription string
var adminToolgenPublisher string
var adminToolgenGoal string
var adminToolgenRequirement string
var adminToolgenMetadata string
var adminToolgenJobID string

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Toolserver and external tool utilities.",
}

var toolsAdminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage GLM Toolserver client credentials via admin endpoints.",
	Long: `Admin endpoints require GLM_ADMIN_TOKEN and are typically localhost-only.

Uses GLM_TOOLSERVER_BASE_URL when set, otherwise defaults to the configured toolserver URL.`,
}

var toolsAdminListCmd = &cobra.Command{
	Use:   "list",
	Short: "List provisioned toolserver clients.",
	Run: func(cmd *cobra.Command, args []string) {
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		clients, err := ac.ListClients()
		if err != nil {
			fmt.Printf("Error listing clients: %v\n", err)
			return
		}
		if len(clients) == 0 {
			fmt.Println("No clients found.")
			return
		}
		fmt.Printf("Found %d client(s):\n", len(clients))
		for _, c := range clients {
			line := "- " + strings.TrimSpace(c.ClientID)
			if !c.CreatedAt.IsZero() {
				line += " (created " + c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z") + ")"
			}
			fmt.Println(line)
		}
	},
}

var toolsAdminCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new toolserver client keypair.",
	Long:  "Returns client_id and api_key once. Store api_key securely.",
	Run: func(cmd *cobra.Command, args []string) {
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		resp, err := ac.CreateClient(strings.TrimSpace(adminCreateClientID))
		if err != nil {
			fmt.Printf("Error creating client: %v\n", err)
			return
		}
		fmt.Println("Client created.")
		fmt.Printf("GLM_CLIENT_ID=%s\n", strings.TrimSpace(resp.ClientID))
		fmt.Printf("GLM_API_KEY=%s\n", strings.TrimSpace(resp.APIKey))
	},
}

var toolsAdminRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate an existing client API key.",
	Run: func(cmd *cobra.Command, args []string) {
		clientID := strings.TrimSpace(adminRotateClientID)
		if clientID == "" {
			fmt.Println("Error: --client-id is required")
			return
		}
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		resp, err := ac.RotateClientKey(clientID)
		if err != nil {
			fmt.Printf("Error rotating client key: %v\n", err)
			return
		}
		fmt.Printf("Rotated client key for %s.\n", clientID)
		fmt.Printf("GLM_CLIENT_ID=%s\n", strings.TrimSpace(resp.ClientID))
		fmt.Printf("GLM_API_KEY=%s\n", strings.TrimSpace(resp.APIKey))
	},
}

var toolsAdminDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete/revoke an existing client.",
	Run: func(cmd *cobra.Command, args []string) {
		clientID := strings.TrimSpace(adminDeleteClientID)
		if clientID == "" {
			fmt.Println("Error: --client-id is required")
			return
		}
		if !adminDeleteYes {
			fmt.Println("Refusing delete without --yes.")
			return
		}
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		if err := ac.DeleteClient(clientID); err != nil {
			fmt.Printf("Error deleting client: %v\n", err)
			return
		}
		fmt.Printf("Deleted client: %s\n", clientID)
	},
}

var toolsAdminPluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage tool plugins via /admin/plugins/* endpoints.",
}

var toolsAdminPluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins.",
	Run: func(cmd *cobra.Command, args []string) {
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		plugins, err := ac.ListPlugins()
		if err != nil {
			fmt.Printf("Error listing plugins: %v\n", err)
			return
		}
		if len(plugins) == 0 {
			fmt.Println("No plugins found.")
			return
		}
		fmt.Printf("Found %d plugin(s):\n", len(plugins))
		for _, p := range plugins {
			state := "disabled"
			if p.Enabled {
				state = "enabled"
			}
			name := strings.TrimSpace(p.Name)
			if name == "" {
				name = strings.TrimSpace(p.ID)
			}
			line := "- " + name + " [" + state + "]"
			if pub := strings.TrimSpace(p.Publisher); pub != "" {
				line += " publisher=" + pub
			}
			if id := strings.TrimSpace(p.ID); id != "" {
				line += " id=" + id
			}
			fmt.Println(line)
		}
	},
}

var toolsAdminPluginsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install a plugin.",
	Run: func(cmd *cobra.Command, args []string) {
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		req := tools.AdminPluginInstallRequest{
			PluginID: strings.TrimSpace(adminPluginID),
			Source:   strings.TrimSpace(adminPluginSource),
		}
		if strings.TrimSpace(adminPluginManifestPath) != "" {
			b, err := os.ReadFile(strings.TrimSpace(adminPluginManifestPath))
			if err != nil {
				fmt.Printf("Error reading manifest file: %v\n", err)
				return
			}
			var manifest map[string]interface{}
			if err := json.Unmarshal(b, &manifest); err != nil {
				fmt.Printf("Error parsing manifest JSON: %v\n", err)
				return
			}
			req.Manifest = manifest
		}
		if strings.TrimSpace(req.PluginID) == "" && len(req.Manifest) == 0 {
			fmt.Println("Error: --plugin-id or --manifest-file is required")
			return
		}
		resp, err := ac.InstallPlugin(req)
		if err != nil {
			fmt.Printf("Error installing plugin: %v\n", err)
			return
		}
		fmt.Println("Plugin installed.")
		fmt.Printf("plugin_id=%s\n", strings.TrimSpace(resp.PluginID))
		fmt.Printf("status=%s\n", strings.TrimSpace(resp.Status))
	},
}

var toolsAdminPluginsEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable a plugin by ID.",
	Run: func(cmd *cobra.Command, args []string) {
		pluginID := strings.TrimSpace(adminPluginID)
		if pluginID == "" {
			fmt.Println("Error: --plugin-id is required")
			return
		}
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		if err := ac.EnablePlugin(pluginID); err != nil {
			fmt.Printf("Error enabling plugin: %v\n", err)
			return
		}
		fmt.Printf("Enabled plugin: %s\n", pluginID)
	},
}

var toolsAdminPluginsDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable a plugin by ID.",
	Run: func(cmd *cobra.Command, args []string) {
		pluginID := strings.TrimSpace(adminPluginID)
		if pluginID == "" {
			fmt.Println("Error: --plugin-id is required")
			return
		}
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		if err := ac.DisablePlugin(pluginID); err != nil {
			fmt.Printf("Error disabling plugin: %v\n", err)
			return
		}
		fmt.Printf("Disabled plugin: %s\n", pluginID)
	},
}

var toolsAdminToolgenCmd = &cobra.Command{
	Use:   "toolgen",
	Short: "Generate plugins via async admin job endpoints.",
}

var toolsAdminToolgenGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Start async plugin generation (POST /admin/plugins).",
	Run: func(cmd *cobra.Command, args []string) {
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		if strings.TrimSpace(adminToolgenName) == "" {
			fmt.Println("Error: --name is required")
			return
		}
		req := tools.AdminToolgenRequest{
			Name:        strings.TrimSpace(adminToolgenName),
			Description: strings.TrimSpace(adminToolgenDescription),
			Publisher:   strings.TrimSpace(adminToolgenPublisher),
			Goal:        strings.TrimSpace(adminToolgenGoal),
			Requirement: strings.TrimSpace(adminToolgenRequirement),
		}
		if strings.TrimSpace(adminToolgenMetadata) != "" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(adminToolgenMetadata), &meta); err != nil {
				fmt.Printf("Error parsing --metadata JSON: %v\n", err)
				return
			}
			req.Metadata = meta
		}
		resp, err := ac.GenerateToolPlugin(req)
		if err != nil {
			fmt.Printf("Error generating plugin: %v\n", err)
			return
		}
		fmt.Println("Toolgen request submitted.")
		fmt.Printf("job_id=%s\n", strings.TrimSpace(resp.JobID))
		fmt.Printf("plugin_id=%s\n", strings.TrimSpace(resp.PluginID))
		fmt.Printf("name=%s\n", strings.TrimSpace(resp.Name))
		fmt.Printf("publisher=%s\n", strings.TrimSpace(resp.Publisher))
		fmt.Printf("status=%s\n", strings.TrimSpace(resp.Status))
		fmt.Printf("endpoint=%s\n", strings.TrimSpace(resp.Endpoint))
	},
}

var toolsAdminToolgenStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get toolgen job status by job ID (GET /admin/plugins/jobs/{id}).",
	Run: func(cmd *cobra.Command, args []string) {
		jobID := strings.TrimSpace(adminToolgenJobID)
		if jobID == "" {
			fmt.Println("Error: --job-id is required")
			return
		}
		ac, err := tools.NewGLMAdminClientFromEnv()
		if err != nil {
			fmt.Printf("Error initializing admin client: %v\n", err)
			return
		}
		resp, err := ac.GetToolgenJob(jobID)
		if err != nil {
			fmt.Printf("Error fetching toolgen job: %v\n", err)
			return
		}
		fmt.Printf("job_id=%s\n", strings.TrimSpace(resp.JobID))
		fmt.Printf("plugin_id=%s\n", strings.TrimSpace(resp.PluginID))
		fmt.Printf("name=%s\n", strings.TrimSpace(resp.Name))
		fmt.Printf("publisher=%s\n", strings.TrimSpace(resp.Publisher))
		fmt.Printf("status=%s\n", strings.TrimSpace(resp.Status))
		fmt.Printf("endpoint=%s\n", strings.TrimSpace(resp.Endpoint))
	},
}

func init() {
	toolsAdminCreateCmd.Flags().StringVar(&adminCreateClientID, "client-id", "", "Optional client ID (auto-generated when omitted)")
	toolsAdminRotateCmd.Flags().StringVar(&adminRotateClientID, "client-id", "", "Client ID to rotate")
	toolsAdminDeleteCmd.Flags().StringVar(&adminDeleteClientID, "client-id", "", "Client ID to delete")
	toolsAdminDeleteCmd.Flags().BoolVar(&adminDeleteYes, "yes", false, "Confirm deletion")
	toolsAdminPluginsInstallCmd.Flags().StringVar(&adminPluginID, "plugin-id", "", "Plugin ID")
	toolsAdminPluginsInstallCmd.Flags().StringVar(&adminPluginSource, "source", "", "Plugin source (optional)")
	toolsAdminPluginsInstallCmd.Flags().StringVar(&adminPluginManifestPath, "manifest-file", "", "Path to manifest JSON (optional)")
	toolsAdminPluginsEnableCmd.Flags().StringVar(&adminPluginID, "plugin-id", "", "Plugin ID")
	toolsAdminPluginsDisableCmd.Flags().StringVar(&adminPluginID, "plugin-id", "", "Plugin ID")
	toolsAdminToolgenGenerateCmd.Flags().StringVar(&adminToolgenName, "name", "", "Plugin/tool name")
	toolsAdminToolgenGenerateCmd.Flags().StringVar(&adminToolgenDescription, "description", "", "Description")
	toolsAdminToolgenGenerateCmd.Flags().StringVar(&adminToolgenPublisher, "publisher", "", "Publisher ID for trusted auto-enable policy")
	toolsAdminToolgenGenerateCmd.Flags().StringVar(&adminToolgenGoal, "goal", "", "Generation goal")
	toolsAdminToolgenGenerateCmd.Flags().StringVar(&adminToolgenRequirement, "requirement", "", "Tool requirement")
	toolsAdminToolgenGenerateCmd.Flags().StringVar(&adminToolgenMetadata, "metadata", "", "Metadata JSON object")
	toolsAdminToolgenStatusCmd.Flags().StringVar(&adminToolgenJobID, "job-id", "", "Toolgen job ID")

	toolsAdminCmd.AddCommand(toolsAdminListCmd)
	toolsAdminCmd.AddCommand(toolsAdminCreateCmd)
	toolsAdminCmd.AddCommand(toolsAdminRotateCmd)
	toolsAdminCmd.AddCommand(toolsAdminDeleteCmd)
	toolsAdminPluginsCmd.AddCommand(toolsAdminPluginsListCmd)
	toolsAdminPluginsCmd.AddCommand(toolsAdminPluginsInstallCmd)
	toolsAdminPluginsCmd.AddCommand(toolsAdminPluginsEnableCmd)
	toolsAdminPluginsCmd.AddCommand(toolsAdminPluginsDisableCmd)
	toolsAdminToolgenCmd.AddCommand(toolsAdminToolgenGenerateCmd)
	toolsAdminToolgenCmd.AddCommand(toolsAdminToolgenStatusCmd)
	toolsAdminCmd.AddCommand(toolsAdminPluginsCmd)
	toolsAdminCmd.AddCommand(toolsAdminToolgenCmd)

	toolsCmd.AddCommand(toolsAdminCmd)
	rootCmd.AddCommand(toolsCmd)
}
