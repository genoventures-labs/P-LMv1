package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultRegistryFile = "index.json"

type SkillRecord struct {
	SkillID       string    `json:"skill_id"`
	Name          string    `json:"name"`
	Intent        string    `json:"intent"`
	Description   string    `json:"description,omitempty"`
	ReasoningTier string    `json:"reasoning_tier,omitempty"`
	TaskType      string    `json:"task_type,omitempty"`
	RootDir       string    `json:"root_dir"`
	SourcePath    string    `json:"source_path"`
	ManifestPath  string    `json:"manifest_path"`
	PackageName   string    `json:"package_name,omitempty"`
	CompileOK     bool      `json:"compile_ok"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SkillRegistry struct {
	Root      string
	IndexPath string
}

func NewSkillRegistry(root string) *SkillRegistry {
	root = strings.TrimSpace(root)
	if root == "" {
		root = PermanentSkillsRoot()
	}
	return &SkillRegistry{
		Root:      root,
		IndexPath: filepath.Join(root, defaultRegistryFile),
	}
}

func (r *SkillRegistry) Upsert(record SkillRecord) error {
	if r == nil {
		r = NewSkillRegistry("")
	}
	record.SkillID = strings.TrimSpace(record.SkillID)
	record.Name = strings.TrimSpace(record.Name)
	record.Intent = strings.TrimSpace(record.Intent)
	record.TaskType = strings.TrimSpace(record.TaskType)
	record.RootDir = strings.TrimSpace(record.RootDir)
	record.SourcePath = strings.TrimSpace(record.SourcePath)
	record.ManifestPath = strings.TrimSpace(record.ManifestPath)
	record.PackageName = strings.TrimSpace(record.PackageName)
	if record.SkillID == "" || record.RootDir == "" || record.SourcePath == "" || record.ManifestPath == "" {
		return os.ErrInvalid
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now

	all, err := r.loadAll()
	if err != nil {
		return err
	}
	found := false
	for i := range all {
		if strings.TrimSpace(all[i].SkillID) == record.SkillID {
			record.CreatedAt = firstNonZeroTime(all[i].CreatedAt, record.CreatedAt)
			all[i] = record
			found = true
			break
		}
	}
	if !found {
		all = append(all, record)
	}
	return r.writeAll(all)
}

func (r *SkillRegistry) ListEnabled() ([]SkillRecord, error) {
	all, err := r.loadAll()
	if err != nil {
		return nil, err
	}
	out := make([]SkillRecord, 0, len(all))
	for _, rec := range all {
		if rec.Enabled {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *SkillRegistry) FindMatch(name, intent, taskType string) (*SkillRecord, bool, error) {
	all, err := r.loadAll()
	if err != nil {
		return nil, false, err
	}
	name = strings.ToLower(strings.TrimSpace(name))
	intent = strings.ToLower(strings.TrimSpace(intent))
	taskType = strings.ToLower(strings.TrimSpace(taskType))
	bestIdx := -1
	bestScore := -1

	for i, rec := range all {
		if !rec.Enabled {
			continue
		}
		score := 0
		recName := strings.ToLower(strings.TrimSpace(rec.Name))
		recIntent := strings.ToLower(strings.TrimSpace(rec.Intent))
		recTask := strings.ToLower(strings.TrimSpace(rec.TaskType))
		if name != "" && recName == name {
			score += 8
		}
		if taskType != "" && recTask != "" && taskType == recTask {
			score += 2
		}
		score += tokenOverlapScore(intent, recIntent)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 || bestScore < 3 {
		return nil, false, nil
	}
	matched := all[bestIdx]
	return &matched, true, nil
}

func (r *SkillRegistry) loadAll() ([]SkillRecord, error) {
	if r == nil {
		r = NewSkillRegistry("")
	}
	raw, err := os.ReadFile(r.IndexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var wrapped struct {
		Skills []SkillRecord `json:"skills"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		return wrapped.Skills, nil
	}
	var out []SkillRecord
	if err := json.Unmarshal(raw, &out); err == nil {
		return out, nil
	}
	return nil, os.ErrInvalid
}

func (r *SkillRegistry) writeAll(records []SkillRecord) error {
	if r == nil {
		r = NewSkillRegistry("")
	}
	if err := os.MkdirAll(filepath.Dir(r.IndexPath), 0o755); err != nil {
		return err
	}
	payload := struct {
		Skills []SkillRecord `json:"skills"`
	}{
		Skills: records,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.IndexPath, b, 0o644)
}

func SkillRecordFromArtifact(req UserSkillCreateRequest, art JITSkillArtifact, enabled bool) SkillRecord {
	return SkillRecord{
		SkillID:       strings.TrimSpace(art.SkillID),
		Name:          strings.TrimSpace(req.Name),
		Intent:        strings.TrimSpace(req.Intent),
		Description:   strings.TrimSpace(req.Description),
		ReasoningTier: strings.TrimSpace(req.ReasoningTier),
		TaskType:      strings.TrimSpace(req.TaskType),
		RootDir:       strings.TrimSpace(art.RootDir),
		SourcePath:    strings.TrimSpace(art.SourcePath),
		ManifestPath:  strings.TrimSpace(art.ManifestPath),
		PackageName:   strings.TrimSpace(art.PackageName),
		CompileOK:     art.CompileOK,
		Enabled:       enabled,
	}
}

func ArtifactFromSkillRecord(rec SkillRecord) JITSkillArtifact {
	return JITSkillArtifact{
		SkillID:      strings.TrimSpace(rec.SkillID),
		RootDir:      strings.TrimSpace(rec.RootDir),
		SourcePath:   strings.TrimSpace(rec.SourcePath),
		ManifestPath: strings.TrimSpace(rec.ManifestPath),
		PackageName:  strings.TrimSpace(rec.PackageName),
		CompileOK:    rec.CompileOK,
	}
}

func tokenOverlapScore(a, b string) int {
	at := registryTokenSet(a)
	bt := registryTokenSet(b)
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	score := 0
	for k := range at {
		if bt[k] {
			score++
		}
	}
	return score
}

func registryTokenSet(s string) map[string]bool {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(",", " ", ".", " ", ";", " ", ":", " ", "/", " ", "_", " ", "-", " ").Replace(s)
	parts := strings.Fields(s)
	out := map[string]bool{}
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		out[p] = true
	}
	return out
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, v := range values {
		if !v.IsZero() {
			return v
		}
	}
	return time.Time{}
}
