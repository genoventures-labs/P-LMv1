package skills

import (
	"encoding/json"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const defaultJITSkillsRoot = ".skills/jit"

var nonAlphaNumRE = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// JITSkillRequest describes a self-capability generation request.
type JITSkillRequest struct {
	Name          string
	Description   string
	Requirement   string
	ReasoningTier string
	TaskType      string
}

// JITSkillArtifact contains generated source + manifest metadata.
type JITSkillArtifact struct {
	SkillID      string
	RootDir      string
	SourcePath   string
	ManifestPath string
	PackageName  string
	CompileOK    bool
}

// JITGenerator generates Go-based self-skill artifacts.
type JITGenerator struct {
	Root string
}

func NewJITGenerator(root string) *JITGenerator {
	root = strings.TrimSpace(root)
	if root == "" {
		root = defaultJITSkillsRoot
	}
	return &JITGenerator{Root: root}
}

func (g *JITGenerator) Generate(req JITSkillRequest) (JITSkillArtifact, error) {
	if g == nil {
		g = NewJITGenerator("")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "generated_skill"
	}
	skillID := fmt.Sprintf("skill_%d", time.Now().UnixNano())
	dir := filepath.Join(g.Root, skillID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return JITSkillArtifact{}, err
	}

	pkgName := sanitizeIdent("jit_" + name)
	if pkgName == "" {
		pkgName = "jit_skill"
	}
	srcPath := filepath.Join(dir, "skill.go")
	src := g.generateSource(pkgName, req)
	formatted, err := format.Source([]byte(src))
	if err != nil {
		formatted = []byte(src)
	}
	if err := os.WriteFile(srcPath, formatted, 0o644); err != nil {
		return JITSkillArtifact{}, err
	}

	compileOK := validateGoSource(srcPath)
	manifestPath := filepath.Join(dir, "skill_manifest.json")
	manifest := map[string]interface{}{
		"skill_id":       skillID,
		"name":           name,
		"description":    strings.TrimSpace(req.Description),
		"requirement":    strings.TrimSpace(req.Requirement),
		"reasoning_tier": strings.TrimSpace(req.ReasoningTier),
		"task_type":      strings.TrimSpace(req.TaskType),
		"package":        pkgName,
		"source_path":    srcPath,
		"compile_ok":     compileOK,
		"created_at":     time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		return JITSkillArtifact{}, err
	}

	return JITSkillArtifact{
		SkillID:      skillID,
		RootDir:      dir,
		SourcePath:   srcPath,
		ManifestPath: manifestPath,
		PackageName:  pkgName,
		CompileOK:    compileOK,
	}, nil
}

func (g *JITGenerator) generateSource(pkgName string, req JITSkillRequest) string {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "generated_skill"
	}
	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = "Generated TALOS self-skill."
	}
	return fmt.Sprintf(`package %s

import (
	"context"
	"strings"
)

// Execute is a generated self-skill entrypoint.
func Execute(_ context.Context, input map[string]any) (map[string]any, error) {
	out := map[string]any{
		"skill": "%s",
		"description": "%s",
		"status": "ready",
	}
	if q, ok := input["query"].(string); ok && strings.TrimSpace(q) != "" {
		out["echo_query"] = strings.TrimSpace(q)
	}
	return out, nil
}
`, pkgName, escapeString(name), escapeString(desc))
}

func validateGoSource(path string) bool {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	return err == nil
}

func sanitizeIdent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNumRE.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return ""
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "x_" + s
	}
	return s
}

func escapeString(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), `"`, `\"`)
}
