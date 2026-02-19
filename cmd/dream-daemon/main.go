package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/envload"
	"github.com/Thynaptic/P-LMv1/pkg/memory"
	"github.com/Thynaptic/P-LMv1/pkg/orchestration"
)

const (
	defaultStatePath = ".memory/dream_daemon/state.json"
	defaultCycleLog  = ".memory/dream_daemon/cycles.jsonl"
	defaultTick      = 45 * time.Second
	defaultIdleAfter = 90 * time.Second
)

type trackedFile struct {
	Path    string `json:"path"`
	ModUnix int64  `json:"mod_unix"`
	Size    int64  `json:"size"`
}

type daemonState struct {
	LastRunAt      time.Time              `json:"last_run_at"`
	LastActivityAt time.Time              `json:"last_activity_at"`
	Cycles         int                    `json:"cycles"`
	LastShards     int                    `json:"last_shards"`
	LastLinks      int                    `json:"last_links"`
	LastEdges      int                    `json:"last_edges"`
	Watched        map[string]trackedFile `json:"watched"`
}

type cycleRecord struct {
	Timestamp       time.Time `json:"timestamp"`
	Shards          int       `json:"shards"`
	Links           int       `json:"links"`
	EdgesReinforced int       `json:"edges_reinforced"`
	TopologyNodes   int       `json:"topology_nodes"`
	TopologyEdges   int       `json:"topology_edges"`
	DurationMS      int64     `json:"duration_ms"`
}

type dreamDaemon struct {
	mm         *memory.MemoryManager
	statePath  string
	cycleLog   string
	tick       time.Duration
	idleAfter  time.Duration
	maxShards  int
	maxLinks   int
	linkBoost  float64
	verbose    bool
	watchPaths []string

	state daemonState
}

func main() {
	if err := envload.Autoload(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: env autoload failed: %v\n", err)
	}

	var (
		statePath = flag.String("state-path", defaultStatePath, "dream daemon state path")
		cycleLog  = flag.String("cycle-log", defaultCycleLog, "dream daemon cycle log JSONL path")
		tick      = flag.Duration("interval", defaultTick, "consolidation poll interval")
		idleAfter = flag.Duration("idle-after", defaultIdleAfter, "required idle duration before dreaming")
		maxShards = flag.Int("max-shards", 400, "max knowledge shards sampled per dream cycle")
		maxLinks  = flag.Int("max-links", 240, "max cross-sectional links applied per dream cycle")
		linkBoost = flag.Float64("link-boost", 0.18, "base topology boost for cross-sectional links")
		runOnce   = flag.Bool("run-once", false, "run one dream cycle and exit")
		verbose   = flag.Bool("verbose", false, "print verbose daemon diagnostics")
	)
	flag.Parse()

	mm, err := memory.NewMemoryManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dream-daemon memory init failed: %v\n", err)
		os.Exit(1)
	}

	d := &dreamDaemon{
		mm:        mm,
		statePath: strings.TrimSpace(*statePath),
		cycleLog:  strings.TrimSpace(*cycleLog),
		tick:      *tick,
		idleAfter: *idleAfter,
		maxShards: *maxShards,
		maxLinks:  *maxLinks,
		linkBoost: *linkBoost,
		verbose:   *verbose,
		watchPaths: []string{
			".memory/decision_feed.jsonl",
			".memory/session_state.json",
			".memory/reasoning_mirror_briefs.jsonl",
		},
	}
	if d.statePath == "" {
		d.statePath = defaultStatePath
	}
	if d.cycleLog == "" {
		d.cycleLog = defaultCycleLog
	}
	if d.tick <= 0 {
		d.tick = defaultTick
	}
	if d.idleAfter <= 0 {
		d.idleAfter = defaultIdleAfter
	}
	if d.maxShards <= 0 {
		d.maxShards = 400
	}
	if d.maxLinks <= 0 {
		d.maxLinks = 240
	}
	if d.linkBoost <= 0 {
		d.linkBoost = 0.18
	}

	if err := d.loadState(); err != nil {
		fmt.Fprintf(os.Stderr, "dream-daemon state load warning: %v\n", err)
	}
	if d.state.LastActivityAt.IsZero() {
		d.state.LastActivityAt = time.Now().UTC()
	}
	_ = d.captureActivity()
	_ = d.persistState()

	if *runOnce {
		if d.isIdleNow() {
			if err := d.runDreamCycle(); err != nil {
				fmt.Fprintf(os.Stderr, "dream-daemon cycle failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
		fmt.Println("Dream cycle skipped: system is not idle yet.")
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	fmt.Printf("glm-dreamd online. interval=%s idle_after=%s\n", d.tick, d.idleAfter)
	if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "glm-dreamd exited with error: %v\n", err)
		os.Exit(1)
	}
}

func (d *dreamDaemon) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.tick)
	defer ticker.Stop()
	for {
		if err := d.captureActivity(); err != nil && d.verbose {
			fmt.Printf("glm-dreamd: activity scan warning: %v\n", err)
		}
		if d.isIdleNow() {
			if err := d.runDreamCycle(); err != nil {
				fmt.Printf("glm-dreamd: dream cycle warning: %v\n", err)
			}
		}
		if err := d.persistState(); err != nil && d.verbose {
			fmt.Printf("glm-dreamd: state persist warning: %v\n", err)
		}

		select {
		case <-ctx.Done():
			_ = d.persistState()
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (d *dreamDaemon) runDreamCycle() error {
	start := time.Now()
	segs, err := d.mm.SnapshotKnowledgeSegments(d.maxShards)
	if err != nil {
		return err
	}
	links := orchestration.BuildSectionCrossLinksFromSegments(segs, d.maxLinks)
	memLinks := make([]memory.CrossSectionalLink, 0, len(links))
	for _, l := range links {
		memLinks = append(memLinks, memory.CrossSectionalLink{
			Entity:   l.Entity,
			SourceA:  l.SourceA,
			SectionA: l.SectionA,
			SourceB:  l.SourceB,
			SectionB: l.SectionB,
			Strength: l.Strength,
			LinkType: l.LinkType,
		})
	}
	edges, err := d.mm.ReinforceTopologyFromCrossSectionLinks(memLinks, d.linkBoost)
	if err != nil {
		return err
	}
	stats := d.mm.TopologyStats()
	rec := cycleRecord{
		Timestamp:       time.Now().UTC(),
		Shards:          len(segs),
		Links:           len(memLinks),
		EdgesReinforced: edges,
		TopologyNodes:   stats.Nodes,
		TopologyEdges:   stats.Edges,
		DurationMS:      time.Since(start).Milliseconds(),
	}
	if err := appendCycleRecord(d.cycleLog, rec); err != nil {
		return err
	}
	d.state.LastRunAt = rec.Timestamp
	d.state.Cycles++
	d.state.LastShards = rec.Shards
	d.state.LastLinks = rec.Links
	d.state.LastEdges = rec.EdgesReinforced
	printCycleStatus(rec, stats)
	return nil
}

func (d *dreamDaemon) isIdleNow() bool {
	return time.Since(d.state.LastActivityAt) >= d.idleAfter
}

func (d *dreamDaemon) captureActivity() error {
	if d.state.Watched == nil {
		d.state.Watched = map[string]trackedFile{}
	}
	now := time.Now().UTC()
	changed := false
	for _, p := range d.watchPaths {
		info, err := os.Stat(strings.TrimSpace(p))
		if err != nil {
			continue
		}
		tf := trackedFile{
			Path:    p,
			ModUnix: info.ModTime().Unix(),
			Size:    info.Size(),
		}
		prev, ok := d.state.Watched[p]
		if !ok || prev.ModUnix != tf.ModUnix || prev.Size != tf.Size {
			changed = true
			d.state.Watched[p] = tf
		}
	}
	if changed {
		d.state.LastActivityAt = now
	}
	return nil
}

func printCycleStatus(rec cycleRecord, stats memory.TopologyStats) {
	fmt.Println("ACTIVE DREAMING CYCLE")
	fmt.Printf("  timestamp: %s\n", rec.Timestamp.Format(time.RFC3339))
	fmt.Printf("  shards: %d | cross_section_links: %d | edges_reinforced: %d\n", rec.Shards, rec.Links, rec.EdgesReinforced)
	fmt.Printf("  topology_nodes: %d | topology_edges: %d | enabled: %t\n", stats.Nodes, stats.Edges, stats.Enabled)
	fmt.Printf("  duration_ms: %d\n", rec.DurationMS)
}

func appendCycleRecord(path string, rec cycleRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func (d *dreamDaemon) loadState() error {
	b, err := os.ReadFile(d.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			d.state = daemonState{Watched: map[string]trackedFile{}}
			return nil
		}
		return err
	}
	var st daemonState
	if err := json.Unmarshal(b, &st); err != nil {
		return err
	}
	if st.Watched == nil {
		st.Watched = map[string]trackedFile{}
	}
	d.state = st
	return nil
}

func (d *dreamDaemon) persistState() error {
	if err := os.MkdirAll(filepath.Dir(d.statePath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.statePath, b, 0o644)
}
