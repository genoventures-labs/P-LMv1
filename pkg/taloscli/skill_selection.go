package taloscli

import (
	"fmt"
	"strings"

	"github.com/Thynaptic/P-LMv1/pkg/skills"
)

func resolveRequestedSkillSelection(query string) (*skills.SkillRecord, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	reg := skills.NewSkillRegistry(skills.PermanentSkillsRoot())
	recs, err := reg.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("load skills registry: %w", err)
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("no enabled skills are available")
	}

	qLower := strings.ToLower(query)
	for i := range recs {
		rec := recs[i]
		if strings.EqualFold(strings.TrimSpace(rec.SkillID), query) || strings.EqualFold(strings.TrimSpace(rec.Name), query) {
			out := rec
			return &out, nil
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(rec.ManifestPath)), qLower) ||
			strings.Contains(strings.ToLower(strings.TrimSpace(rec.RootDir)), qLower) {
			out := rec
			return &out, nil
		}
	}

	if match, ok, err := reg.FindMatch(query, query, ""); err == nil && ok && match != nil {
		return match, nil
	}
	if err != nil {
		return nil, fmt.Errorf("match skill: %w", err)
	}
	return nil, fmt.Errorf("skill not found: %s", query)
}

func skillContextBlock(rec *skills.SkillRecord) string {
	if rec == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("Active skill profile:\n")
	if strings.TrimSpace(rec.SkillID) != "" {
		b.WriteString("- skill_id: " + strings.TrimSpace(rec.SkillID) + "\n")
	}
	if strings.TrimSpace(rec.Name) != "" {
		b.WriteString("- name: " + strings.TrimSpace(rec.Name) + "\n")
	}
	if strings.TrimSpace(rec.Intent) != "" {
		b.WriteString("- intent: " + strings.TrimSpace(rec.Intent) + "\n")
	}
	if strings.TrimSpace(rec.Description) != "" {
		b.WriteString("- description: " + strings.TrimSpace(rec.Description) + "\n")
	}
	if strings.TrimSpace(rec.ReasoningTier) != "" {
		b.WriteString("- reasoning_tier: " + strings.TrimSpace(rec.ReasoningTier) + "\n")
	}
	if strings.TrimSpace(rec.TaskType) != "" {
		b.WriteString("- task_type: " + strings.TrimSpace(rec.TaskType) + "\n")
	}
	b.WriteString("- directive: Prioritize this skill's intent and approach while answering.\n")
	return strings.TrimSpace(b.String())
}
