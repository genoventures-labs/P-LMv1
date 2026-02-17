package taloscli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Thynaptic/P-LMv1/pkg/skills"
	"github.com/Thynaptic/P-LMv1/pkg/tools"
	"github.com/spf13/cobra"
)

var userSkillName string
var userSkillIntent string
var userSkillDescription string
var userSkillReasoningTier string
var userSkillTaskType string
var userSkillRequestedTools []string
var userSkillRequestedDomains []string
var userSkillShowID string

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Create and inspect user-defined TALOS skills.",
}

var skillsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a persistent TALOS self-skill (requires preflight allow).",
	Run: func(cmd *cobra.Command, args []string) {
		if strings.TrimSpace(userSkillName) == "" {
			fmt.Println("Error: --name is required")
			return
		}
		if strings.TrimSpace(userSkillIntent) == "" {
			fmt.Println("Error: --intent is required")
			return
		}
		tc, err := tools.NewGLMToolClient()
		if err != nil {
			fmt.Printf("Error initializing tool client: %v\n", err)
			return
		}
		creator := skills.NewUserSkillCreator(tc)
		reqTools, err := parseRequestedTools(userSkillRequestedTools)
		if err != nil {
			fmt.Printf("Error parsing --requested-tool: %v\n", err)
			return
		}
		res, err := creator.Create(context.Background(), skills.UserSkillCreateRequest{
			Name:             strings.TrimSpace(userSkillName),
			Intent:           strings.TrimSpace(userSkillIntent),
			Description:      strings.TrimSpace(userSkillDescription),
			ReasoningTier:    strings.TrimSpace(userSkillReasoningTier),
			TaskType:         strings.TrimSpace(userSkillTaskType),
			RequestedTools:   reqTools,
			RequestedDomains: dedupeSkillStrings(userSkillRequestedDomains),
		})
		if err != nil {
			fmt.Printf("Error creating user skill: %v\n", err)
			return
		}
		fmt.Println("User skill created.")
		fmt.Printf("skill_id=%s\n", strings.TrimSpace(res.Artifact.SkillID))
		fmt.Printf("root_dir=%s\n", strings.TrimSpace(res.Artifact.RootDir))
		fmt.Printf("source_path=%s\n", strings.TrimSpace(res.Artifact.SourcePath))
		fmt.Printf("manifest_path=%s\n", strings.TrimSpace(res.Artifact.ManifestPath))
		if res.Artifact.CompileOK {
			fmt.Println("compile_ok=true")
		} else {
			fmt.Println("compile_ok=false")
		}
		fmt.Printf("preflight_decision=%s\n", strings.TrimSpace(res.Preflight.Decision))
	},
}

var skillsPreflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run `/skills/preflight` for a proposed user skill.",
	Run: func(cmd *cobra.Command, args []string) {
		if strings.TrimSpace(userSkillName) == "" {
			fmt.Println("Error: --name is required")
			return
		}
		if strings.TrimSpace(userSkillIntent) == "" {
			fmt.Println("Error: --intent is required")
			return
		}
		tc, err := tools.NewGLMToolClient()
		if err != nil {
			fmt.Printf("Error initializing tool client: %v\n", err)
			return
		}
		reqTools, err := parseRequestedTools(userSkillRequestedTools)
		if err != nil {
			fmt.Printf("Error parsing --requested-tool: %v\n", err)
			return
		}
		resp, err := tc.SkillPreflight(tools.SkillPreflightRequest{
			SkillName:        strings.TrimSpace(userSkillName),
			Intent:           strings.TrimSpace(userSkillIntent),
			RequestedTools:   reqTools,
			RequestedDomains: dedupeSkillStrings(userSkillRequestedDomains),
		})
		if err != nil {
			fmt.Printf("Preflight error: %v\n", err)
			return
		}
		fmt.Printf("decision=%s\n", strings.TrimSpace(resp.Decision))
		fmt.Printf("reason=%s\n", strings.TrimSpace(resp.Reason))
		fmt.Printf("invoke_count=%d\n", len(resp.Invoke))
		for i, inv := range resp.Invoke {
			fmt.Printf("invoke_%d_path=%s\n", i+1, strings.TrimSpace(inv.Path))
			fmt.Printf("invoke_%d_url=%s\n", i+1, strings.TrimSpace(inv.URL))
		}
		if len(resp.Limits) > 0 {
			b, _ := json.Marshal(resp.Limits)
			fmt.Printf("limits=%s\n", string(b))
		}
	},
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List user-defined skills in .skills/permanent.",
	Run: func(cmd *cobra.Command, args []string) {
		reg := skills.NewSkillRegistry(skills.PermanentSkillsRoot())
		recs, err := reg.ListEnabled()
		if err == nil && len(recs) > 0 {
			sort.SliceStable(recs, func(i, j int) bool {
				return recs[i].UpdatedAt.After(recs[j].UpdatedAt)
			})
			fmt.Printf("Found %d enabled skill(s):\n", len(recs))
			for _, rec := range recs {
				line := "- " + strings.TrimSpace(rec.RootDir)
				if strings.TrimSpace(rec.SkillID) != "" {
					line += " skill_id=" + strings.TrimSpace(rec.SkillID)
				}
				if strings.TrimSpace(rec.Name) != "" {
					line += " name=" + strings.TrimSpace(rec.Name)
				}
				fmt.Println(line)
			}
			return
		}
		paths, err := listSkillManifests(".skills/permanent")
		if err != nil {
			fmt.Printf("Error listing skills: %v\n", err)
			return
		}
		if len(paths) == 0 {
			fmt.Println("No user skills found.")
			return
		}
		fmt.Printf("Found %d skill(s):\n", len(paths))
		for _, p := range paths {
			id, name := readSkillIdentity(p)
			line := "- " + filepath.Dir(p)
			if strings.TrimSpace(id) != "" {
				line += " skill_id=" + id
			}
			if strings.TrimSpace(name) != "" {
				line += " name=" + name
			}
			fmt.Println(line)
		}
	},
}

var skillsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show one user skill manifest by skill ID, name, or path suffix.",
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.TrimSpace(userSkillShowID)
		if query == "" {
			fmt.Println("Error: --id is required")
			return
		}
		reg := skills.NewSkillRegistry(skills.PermanentSkillsRoot())
		recs, err := reg.ListEnabled()
		if err == nil {
			for _, rec := range recs {
				if query == rec.SkillID || query == rec.Name || strings.Contains(filepath.ToSlash(rec.ManifestPath), query) {
					b, rerr := os.ReadFile(strings.TrimSpace(rec.ManifestPath))
					if rerr != nil {
						fmt.Printf("Error reading manifest: %v\n", rerr)
						return
					}
					fmt.Println(string(b))
					return
				}
			}
		}
		paths, err := listSkillManifests(".skills/permanent")
		if err != nil {
			fmt.Printf("Error loading skills: %v\n", err)
			return
		}
		for _, p := range paths {
			id, name := readSkillIdentity(p)
			if query == id || query == name || strings.Contains(filepath.ToSlash(p), query) {
				b, err := os.ReadFile(p)
				if err != nil {
					fmt.Printf("Error reading manifest: %v\n", err)
					return
				}
				fmt.Println(string(b))
				return
			}
		}
		fmt.Printf("Skill not found: %s\n", query)
	},
}

func init() {
	skillsCreateCmd.Flags().StringVar(&userSkillName, "name", "", "Skill name")
	skillsCreateCmd.Flags().StringVar(&userSkillIntent, "intent", "", "Intent/capability description")
	skillsCreateCmd.Flags().StringVar(&userSkillDescription, "description", "", "Optional description")
	skillsCreateCmd.Flags().StringVar(&userSkillReasoningTier, "reasoning-tier", "t2", "Reasoning tier tag")
	skillsCreateCmd.Flags().StringVar(&userSkillTaskType, "task-type", "general", "Task type tag")
	skillsCreateCmd.Flags().StringSliceVar(&userSkillRequestedTools, "requested-tool", nil, "Requested tool in kind:name format (repeatable)")
	skillsCreateCmd.Flags().StringSliceVar(&userSkillRequestedDomains, "requested-domain", nil, "Requested external domain (repeatable)")

	skillsPreflightCmd.Flags().StringVar(&userSkillName, "name", "", "Skill name")
	skillsPreflightCmd.Flags().StringVar(&userSkillIntent, "intent", "", "Intent/capability description")
	skillsPreflightCmd.Flags().StringSliceVar(&userSkillRequestedTools, "requested-tool", nil, "Requested tool in kind:name format (repeatable)")
	skillsPreflightCmd.Flags().StringSliceVar(&userSkillRequestedDomains, "requested-domain", nil, "Requested external domain (repeatable)")

	skillsShowCmd.Flags().StringVar(&userSkillShowID, "id", "", "Skill ID, name, or path fragment")

	skillsCmd.AddCommand(skillsCreateCmd)
	skillsCmd.AddCommand(skillsPreflightCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsShowCmd)
	rootCmd.AddCommand(skillsCmd)
}

func parseRequestedTools(raw []string) ([]tools.SkillPreflightToolRequest, error) {
	out := make([]tools.SkillPreflightToolRequest, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("invalid format %q, expected kind:name", item)
		}
		out = append(out, tools.SkillPreflightToolRequest{
			Kind: strings.TrimSpace(parts[0]),
			Name: strings.TrimSpace(parts[1]),
		})
	}
	return out, nil
}

func listSkillManifests(root string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "skill_manifest.json") {
			out = append(out, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return out, err
}

func readSkillIdentity(path string) (id, name string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return "", ""
	}
	id = strings.TrimSpace(fmt.Sprintf("%v", raw["skill_id"]))
	name = strings.TrimSpace(fmt.Sprintf("%v", raw["name"]))
	if id == "<nil>" {
		id = ""
	}
	if name == "<nil>" {
		name = ""
	}
	return id, name
}

func dedupeSkillStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}
