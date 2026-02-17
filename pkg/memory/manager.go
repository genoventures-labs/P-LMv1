package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Thynaptic/P-LMv1/pkg/state"
	"github.com/ollama/ollama/api"
	chromem "github.com/philippgille/chromem-go"
)

const (
	embeddingModelName       = "nomic-embed-text:latest"
	collectionName           = "conversation_history"
	knowledgeCollection      = "knowledge_base"
	ollamaAPIHostEnv         = "OLLAMA_HOST"
	defaultBaseImportance    = 0.5
	defaultFreshnessHalfLife = 24 * time.Hour
	importanceEvalTimeout    = 8 * time.Second
)

var importanceEvalModels = []string{"llama3.2:1b", "qwen2.5:3b-instruct"}

const (
	metaTimestamp       = "timestamp"
	metaBaseImportance  = "base_importance"
	metaRetrievalCount  = "retrieval_count"
	metaLastRetrievedAt = "last_retrieved_at"
	metaClusterID       = "cluster_id"
	metaClusterLabel    = "cluster_label"
	metaClusterSize     = "cluster_size"
	metaArchived        = "archived"
	metaArchiveReason   = "archive_reason"
	metaLastReindexedAt = "last_reindexed_at"
	metaModality        = "modality"
	metaImagePath       = "image_path"
	metaVisualModel     = "visual_model"
	metaCrossModalLink  = "cross_modal_link_id"
)

// MemoryManager handles interaction with chromem-go for conversation memory.
type MemoryManager struct {
	db                  *chromem.DB
	historyCollection   *chromem.Collection
	knowledgeCollection *chromem.Collection
	client              *api.Client
	activeNamespace     string
	mu                  sync.Mutex
}

// KnowledgeSegment is a query result with source metadata for document orchestration.
type KnowledgeSegment struct {
	ID         string
	Content    string
	Similarity float64
	Metadata   map[string]string
}

// NewMemoryManager initializes a new MemoryManager with persistent storage.
func NewMemoryManager() (*MemoryManager, error) {
	// Initialize Ollama client for embeddings
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client for embeddings: %w", err)
	}

	// Create custom embedding function
	ef := func(ctx context.Context, text string) ([]float32, error) {
		req := &api.EmbeddingRequest{
			Model:  embeddingModelName,
			Prompt: text,
		}
		resp, err := client.Embeddings(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("ollama embedding error: %w", err)
		}
		if len(resp.Embedding) == 0 {
			return nil, fmt.Errorf("no embedding returned from ollama")
		}

		// Convert []float64 to []float32
		res := make([]float32, len(resp.Embedding))
		for i, f := range resp.Embedding {
			res[i] = float32(f)
		}
		return res, nil
	}

	// Initialize chromem-go database with persistence
	db, err := chromem.NewPersistentDB(".memory", true)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent chromem-go database: %w", err)
	}

	// Create or get collection for conversation history
	hCol, err := db.GetOrCreateCollection(collectionName, nil, ef)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create history collection: %w", err)
	}

	// Create or get collection for general knowledge
	kCol, err := db.GetOrCreateCollection(knowledgeCollection, nil, ef)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create knowledge collection: %w", err)
	}

	return &MemoryManager{
		db:                  db,
		historyCollection:   hCol,
		knowledgeCollection: kCol,
		client:              client,
		activeNamespace:     "",
	}, nil
}

// SetActiveNamespace scopes all Add/Retrieve operations to a namespace.
// Empty means global/unscoped behavior (no metadata filter).
func (mm *MemoryManager) SetActiveNamespace(namespace string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.activeNamespace = strings.TrimSpace(namespace)
}

// ActiveNamespace returns the currently scoped namespace.
func (mm *MemoryManager) ActiveNamespace() string {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	return mm.activeNamespace
}

// AddMessage adds a message (user or assistant) to the conversation memory.
func (mm *MemoryManager) AddMessage(role, content string) error {
	importance := mm.evaluateMessageImportance(role, content)
	now := time.Now().UTC()
	doc := chromem.Document{
		ID:      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Content: fmt.Sprintf("%s: %s", role, content),
		Metadata: map[string]string{
			"role":              role,
			"type":              "chat",
			metaTimestamp:       now.Format(time.RFC3339),
			metaBaseImportance:  formatFloat(importance),
			metaRetrievalCount:  "0",
			metaLastRetrievedAt: "",
			metaArchived:        "false",
		},
	}
	if ns := strings.TrimSpace(mm.ActiveNamespace()); ns != "" {
		doc.Metadata["namespace"] = ns
	}

	err := mm.historyCollection.AddDocument(context.Background(), doc)
	if err != nil {
		return fmt.Errorf("failed to add message to history: %w", err)
	}

	return nil
}

// RetrieveDynamicContext retrieves context using semantic similarity, importance, and freshness weighting.
//
// Score = (SemanticSimilarity * 0.5) + (Importance * 0.3) + (Freshness * 0.2)
func (mm *MemoryManager) RetrieveDynamicContext(query string, k int) ([]string, error) {
	count := mm.historyCollection.Count()
	if count == 0 {
		return nil, nil
	}
	if k > count {
		k = count
	}
	if k <= 0 {
		return nil, nil
	}

	// Query a broader candidate set before weighted reranking.
	candidatesN := minInt(maxInt(k*8, 16), count)
	results, err := mm.historyCollection.Query(context.Background(), query, candidatesN, mm.namespaceWhereFilter(), nil)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	type scored struct {
		id      string
		content string
		score   float64
	}
	scoredResults := make([]scored, 0, len(results))
	for _, result := range results {
		if isArchived(result.Metadata) {
			continue
		}
		importance := parseMetaFloat(result.Metadata, metaBaseImportance, defaultBaseImportance)
		freshness := freshnessScore(parseMetaTime(result.Metadata, metaTimestamp), now)
		semantic := similarityToUnit(result.Similarity)
		finalScore := (semantic * 0.5) + (importance * 0.3) + (freshness * 0.2)
		scoredResults = append(scoredResults, scored{
			id:      result.ID,
			content: result.Content,
			score:   finalScore,
		})
	}

	sort.SliceStable(scoredResults, func(i, j int) bool {
		return scoredResults[i].score > scoredResults[j].score
	})
	if len(scoredResults) > k {
		scoredResults = scoredResults[:k]
	}

	contextMessages := make([]string, 0, k)
	selectedIDs := make(map[string]bool)
	clusterIDs := make(map[string]bool)
	for _, s := range scoredResults {
		if len(contextMessages) >= k {
			break
		}
		contextMessages = append(contextMessages, s.content)
		selectedIDs[s.id] = true
		mm.bumpRetrievalCount(mm.historyCollection, s.id, now)
		if doc, err := mm.historyCollection.GetByID(context.Background(), s.id); err == nil {
			if cid := strings.TrimSpace(doc.Metadata[metaClusterID]); cid != "" {
				clusterIDs[cid] = true
			}
		}
	}
	if len(contextMessages) < k && len(clusterIDs) > 0 {
		extra := mm.clusterCompanions(mm.historyCollection, clusterIDs, selectedIDs, k-len(contextMessages), now)
		contextMessages = append(contextMessages, extra...)
	}

	return contextMessages, nil
}

// RetrieveContext retrieves relevant past conversation context based on the current query.
func (mm *MemoryManager) RetrieveContext(query string, k int) ([]string, error) {
	count := mm.historyCollection.Count()
	if count == 0 {
		return nil, nil
	}
	if k > count {
		k = count
	}

	results, err := mm.historyCollection.Query(context.Background(), query, k, mm.namespaceWhereFilter(), nil)
	if err != nil {
		return nil, err
	}

	var contextMessages []string
	for _, result := range results {
		contextMessages = append(contextMessages, result.Content)
	}

	return contextMessages, nil
}

// AddKnowledge adds information to the persistent knowledge base.
func (mm *MemoryManager) AddKnowledge(content string, metadata map[string]string) error {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["type"] = "knowledge"
	if _, ok := metadata[metaTimestamp]; !ok {
		metadata[metaTimestamp] = time.Now().UTC().Format(time.RFC3339)
	}
	if _, ok := metadata[metaBaseImportance]; !ok {
		metadata[metaBaseImportance] = formatFloat(defaultBaseImportance)
	}
	if _, ok := metadata[metaRetrievalCount]; !ok {
		metadata[metaRetrievalCount] = "0"
	}
	if _, ok := metadata[metaLastRetrievedAt]; !ok {
		metadata[metaLastRetrievedAt] = ""
	}
	if _, ok := metadata[metaArchived]; !ok {
		metadata[metaArchived] = "false"
	}
	if ns := strings.TrimSpace(mm.ActiveNamespace()); ns != "" {
		if _, ok := metadata["namespace"]; !ok {
			metadata["namespace"] = ns
		}
	}

	doc := chromem.Document{
		ID:       fmt.Sprintf("kb_%d", time.Now().UnixNano()),
		Content:  content,
		Metadata: metadata,
	}

	err := mm.knowledgeCollection.AddDocument(context.Background(), doc)
	if err != nil {
		return fmt.Errorf("failed to add knowledge: %w", err)
	}

	return nil
}

// RefineImportance re-evaluates importance from retrieval frequency and updates metadata.
// Intended to be called periodically as a background maintenance task.
func (mm *MemoryManager) RefineImportance() error {
	now := time.Now().UTC()
	if err := mm.refineCollectionImportance(mm.historyCollection, now); err != nil {
		return err
	}
	if err := mm.refineCollectionImportance(mm.knowledgeCollection, now); err != nil {
		return err
	}
	return nil
}

// RetrieveKnowledge retrieves relevant info from the knowledge base.
func (mm *MemoryManager) RetrieveKnowledge(query string, k int) ([]string, error) {
	count := mm.knowledgeCollection.Count()
	if count == 0 {
		return nil, nil
	}
	if k > count {
		k = count
	}

	results, err := mm.knowledgeCollection.Query(context.Background(), query, k, mm.namespaceWhereFilter(), nil)
	if err != nil {
		return nil, err
	}

	var knowledge []string
	selectedIDs := make(map[string]bool)
	clusterIDs := make(map[string]bool)
	now := time.Now().UTC()
	for _, result := range results {
		if isArchived(result.Metadata) {
			continue
		}
		knowledge = append(knowledge, result.Content)
		selectedIDs[result.ID] = true
		mm.bumpRetrievalCount(mm.knowledgeCollection, result.ID, now)
		if cid := strings.TrimSpace(result.Metadata[metaClusterID]); cid != "" {
			clusterIDs[cid] = true
		}
		if len(knowledge) >= k {
			break
		}
	}
	if len(knowledge) < k && len(clusterIDs) > 0 {
		extra := mm.clusterCompanions(mm.knowledgeCollection, clusterIDs, selectedIDs, k-len(knowledge), now)
		knowledge = append(knowledge, extra...)
	}
	if len(knowledge) < k {
		linkIDs := mm.collectCrossModalLinkIDs(mm.knowledgeCollection, selectedIDs)
		if len(linkIDs) > 0 {
			extra := mm.crossModalCompanions(mm.knowledgeCollection, linkIDs, selectedIDs, k-len(knowledge), now)
			knowledge = append(knowledge, extra...)
		}
	}

	return knowledge, nil
}

// RetrieveKnowledgeSegments returns chunk-level knowledge matches with metadata.
func (mm *MemoryManager) RetrieveKnowledgeSegments(query string, k int) ([]KnowledgeSegment, error) {
	count := mm.knowledgeCollection.Count()
	if count == 0 {
		return nil, nil
	}
	if k > count {
		k = count
	}
	if k <= 0 {
		return nil, nil
	}

	results, err := mm.knowledgeCollection.Query(context.Background(), query, k, mm.namespaceWhereFilter(), nil)
	if err != nil {
		return nil, err
	}

	segments := make([]KnowledgeSegment, 0, len(results))
	for _, result := range results {
		if isArchived(result.Metadata) {
			continue
		}
		meta := make(map[string]string, len(result.Metadata))
		for mk, mv := range result.Metadata {
			meta[mk] = mv
		}
		segments = append(segments, KnowledgeSegment{
			ID:         result.ID,
			Content:    result.Content,
			Similarity: similarityToUnit(result.Similarity),
			Metadata:   meta,
		})
	}
	if len(segments) < k {
		selected := make(map[string]bool, len(segments))
		for _, s := range segments {
			selected[s.ID] = true
		}
		linkIDs := mm.collectCrossModalLinkIDs(mm.knowledgeCollection, selected)
		if len(linkIDs) > 0 {
			extra := mm.crossModalCompanionSegments(mm.knowledgeCollection, linkIDs, selected, k-len(segments), time.Now().UTC())
			segments = append(segments, extra...)
		}
	}
	return segments, nil
}

// HistoryCount returns the number of items in the history collection.
func (mm *MemoryManager) HistoryCount() int {
	return mm.historyCollection.Count()
}

// GetHistoryCollection returns the underlying history collection.
func (mm *MemoryManager) GetHistoryCollection() *chromem.Collection {
	return mm.historyCollection
}

// KnowledgeCount returns the number of items in the knowledge collection.
func (mm *MemoryManager) KnowledgeCount() int {
	return mm.knowledgeCollection.Count()
}

func (mm *MemoryManager) evaluateMessageImportance(role, content string) float64 {
	base := heuristicImportance(role, content)
	if mm.client == nil || strings.TrimSpace(content) == "" {
		return base
	}

	system := `You score memory importance for a personal AI system.
Return JSON only: {"importance": 0.0}
importance must be in [0.0,1.0], where:
- 0.0 means trivial/chit-chat
- 1.0 means mission-critical for future reasoning.`

	user := fmt.Sprintf(`{"role":%q,"content":%q}`, role, content)

	for _, model := range importanceEvalModels {
		opts, _ := state.ResolveEntropyOptions(user)
		req := &api.ChatRequest{
			Model:   model,
			Options: opts,
			Messages: []api.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), importanceEvalTimeout)
		var out strings.Builder
		err := mm.client.Chat(ctx, req, func(resp api.ChatResponse) error {
			out.WriteString(resp.Message.Content)
			return nil
		})
		cancel()
		if err != nil {
			continue
		}

		if imp, ok := parseImportanceJSON(out.String()); ok {
			return imp
		}
	}
	return base
}

func heuristicImportance(role, content string) float64 {
	c := strings.ToLower(content)
	score := 0.45
	if role == "user" {
		score += 0.05
	}
	keywords := []string{
		"goal", "deadline", "must", "critical", "important", "remember",
		"production", "incident", "bug", "regression", "security", "password",
		"architecture", "design", "constraint", "requirement",
	}
	for _, kw := range keywords {
		if strings.Contains(c, kw) {
			score += 0.04
		}
	}
	if len(content) > 300 {
		score += 0.05
	}
	return clamp01(score)
}

func parseImportanceJSON(raw string) (float64, bool) {
	raw = strings.TrimSpace(stripCodeFence(raw))
	type payload struct {
		Importance float64 `json:"importance"`
	}
	var p payload
	if err := json.Unmarshal([]byte(raw), &p); err == nil {
		return clamp01(p.Importance), true
	}
	// best effort: extract JSON object if model wrapped text around it
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &p); err == nil {
			return clamp01(p.Importance), true
		}
	}
	return 0, false
}

func stripCodeFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") || !strings.HasSuffix(t, "```") {
		return t
	}
	lines := strings.Split(t, "\n")
	if len(lines) < 3 {
		return t
	}
	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}

func (mm *MemoryManager) bumpRetrievalCount(col *chromem.Collection, docID string, now time.Time) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	doc, err := col.GetByID(context.Background(), docID)
	if err != nil {
		return
	}
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]string)
	}
	count := parseMetaInt(doc.Metadata, metaRetrievalCount, 0) + 1
	doc.Metadata[metaRetrievalCount] = strconv.Itoa(count)
	doc.Metadata[metaLastRetrievedAt] = now.Format(time.RFC3339)
	// Keep timestamps for older docs that were created before metadata enhancement.
	if _, ok := doc.Metadata[metaTimestamp]; !ok {
		doc.Metadata[metaTimestamp] = now.Format(time.RFC3339)
	}
	_ = col.AddDocument(context.Background(), doc)
}

func (mm *MemoryManager) refineCollectionImportance(col *chromem.Collection, now time.Time) error {
	// Use a broad self-query as a pragmatic way to enumerate persisted documents.
	count := col.Count()
	if count == 0 {
		return nil
	}
	results, err := col.Query(context.Background(), " ", count, nil, nil)
	if err != nil {
		return err
	}

	for _, r := range results {
		doc, getErr := col.GetByID(context.Background(), r.ID)
		if getErr != nil {
			continue
		}
		if doc.Metadata == nil {
			doc.Metadata = make(map[string]string)
		}

		current := parseMetaFloat(doc.Metadata, metaBaseImportance, defaultBaseImportance)
		retrievals := parseMetaInt(doc.Metadata, metaRetrievalCount, 0)

		// Replay rule: frequently retrieved memories gain importance.
		boost := math.Min(0.30, math.Log1p(float64(retrievals))*0.08)
		target := clamp01(defaultBaseImportance + boost)
		updated := clamp01((current * 0.7) + (target * 0.3))

		doc.Metadata[metaBaseImportance] = formatFloat(updated)
		if _, ok := doc.Metadata[metaTimestamp]; !ok {
			doc.Metadata[metaTimestamp] = now.Format(time.RFC3339)
		}
		if _, ok := doc.Metadata[metaLastRetrievedAt]; !ok {
			doc.Metadata[metaLastRetrievedAt] = ""
		}
		if _, ok := doc.Metadata[metaRetrievalCount]; !ok {
			doc.Metadata[metaRetrievalCount] = "0"
		}

		if addErr := col.AddDocument(context.Background(), doc); addErr != nil {
			continue
		}
	}
	return nil
}

func freshnessScore(createdAt *time.Time, now time.Time) float64 {
	if createdAt == nil || createdAt.IsZero() || !now.After(*createdAt) {
		return 1.0
	}
	age := now.Sub(*createdAt)
	return clamp01(math.Pow(2, -age.Seconds()/defaultFreshnessHalfLife.Seconds()))
}

func similarityToUnit(sim float32) float64 {
	// cosine similarity from chromem is in [-1,1]; project to [0,1]
	return clamp01((float64(sim) + 1.0) / 2.0)
}

func parseMetaFloat(meta map[string]string, key string, fallback float64) float64 {
	if meta == nil {
		return fallback
	}
	raw, ok := meta[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return clamp01(v)
}

func parseMetaInt(meta map[string]string, key string, fallback int) int {
	if meta == nil {
		return fallback
	}
	raw, ok := meta[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < 0 {
		return 0
	}
	return v
}

func parseMetaTime(meta map[string]string, key string) *time.Time {
	if meta == nil {
		return nil
	}
	raw, ok := meta[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}

func isArchived(meta map[string]string) bool {
	if meta == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(meta[metaArchived]))
	return v == "true" || v == "1" || v == "yes"
}

func (mm *MemoryManager) clusterCompanions(col *chromem.Collection, clusterIDs map[string]bool, exclude map[string]bool, limit int, now time.Time) []string {
	if limit <= 0 || len(clusterIDs) == 0 {
		return nil
	}
	count := col.Count()
	if count == 0 {
		return nil
	}

	results, err := col.Query(context.Background(), " ", count, mm.namespaceWhereFilter(), nil)
	if err != nil {
		return nil
	}

	var out []string
	for _, r := range results {
		if len(out) >= limit {
			break
		}
		if exclude[r.ID] || isArchived(r.Metadata) {
			continue
		}
		cid := strings.TrimSpace(r.Metadata[metaClusterID])
		if cid == "" || !clusterIDs[cid] {
			continue
		}
		exclude[r.ID] = true
		out = append(out, r.Content)
		mm.bumpRetrievalCount(col, r.ID, now)
	}
	return out
}

func (mm *MemoryManager) collectCrossModalLinkIDs(col *chromem.Collection, selected map[string]bool) map[string]bool {
	if len(selected) == 0 {
		return nil
	}
	out := map[string]bool{}
	for id := range selected {
		doc, err := col.GetByID(context.Background(), id)
		if err != nil || doc.Metadata == nil {
			continue
		}
		if link := strings.TrimSpace(doc.Metadata[metaCrossModalLink]); link != "" {
			out[link] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (mm *MemoryManager) crossModalCompanions(col *chromem.Collection, linkIDs map[string]bool, exclude map[string]bool, limit int, now time.Time) []string {
	if limit <= 0 || len(linkIDs) == 0 {
		return nil
	}
	count := col.Count()
	if count == 0 {
		return nil
	}

	results, err := col.Query(context.Background(), " ", count, mm.namespaceWhereFilter(), nil)
	if err != nil {
		return nil
	}

	var out []string
	for _, r := range results {
		if len(out) >= limit {
			break
		}
		if exclude[r.ID] || isArchived(r.Metadata) {
			continue
		}
		link := strings.TrimSpace(r.Metadata[metaCrossModalLink])
		if link == "" || !linkIDs[link] {
			continue
		}
		exclude[r.ID] = true
		out = append(out, r.Content)
		mm.bumpRetrievalCount(col, r.ID, now)
	}
	return out
}

func (mm *MemoryManager) crossModalCompanionSegments(col *chromem.Collection, linkIDs map[string]bool, exclude map[string]bool, limit int, now time.Time) []KnowledgeSegment {
	if limit <= 0 || len(linkIDs) == 0 {
		return nil
	}
	count := col.Count()
	if count == 0 {
		return nil
	}
	results, err := col.Query(context.Background(), " ", count, mm.namespaceWhereFilter(), nil)
	if err != nil {
		return nil
	}

	out := make([]KnowledgeSegment, 0, limit)
	for _, r := range results {
		if len(out) >= limit {
			break
		}
		if exclude[r.ID] || isArchived(r.Metadata) {
			continue
		}
		link := strings.TrimSpace(r.Metadata[metaCrossModalLink])
		if link == "" || !linkIDs[link] {
			continue
		}
		exclude[r.ID] = true
		meta := make(map[string]string, len(r.Metadata))
		for k, v := range r.Metadata {
			meta[k] = v
		}
		out = append(out, KnowledgeSegment{
			ID:         r.ID,
			Content:    r.Content,
			Similarity: similarityToUnit(r.Similarity),
			Metadata:   meta,
		})
		mm.bumpRetrievalCount(col, r.ID, now)
	}
	return out
}

// PruneLowSignalContext archives low-signal history items to reduce active context pressure.
// Returns number of newly archived history records.
func (mm *MemoryManager) PruneLowSignalContext(maxArchive int) (int, error) {
	if mm == nil || mm.historyCollection == nil {
		return 0, fmt.Errorf("memory manager not initialized")
	}
	if maxArchive <= 0 {
		maxArchive = 8
	}
	count := mm.historyCollection.Count()
	if count == 0 {
		return 0, nil
	}
	results, err := mm.historyCollection.Query(context.Background(), " ", count, mm.namespaceWhereFilter(), nil)
	if err != nil {
		return 0, err
	}

	type candidate struct {
		doc     chromem.Document
		score   float64
		staleHr float64
	}
	var cands []candidate
	now := time.Now().UTC()
	for _, r := range results {
		doc, err := mm.historyCollection.GetByID(context.Background(), r.ID)
		if err != nil || doc.Metadata == nil || isArchived(doc.Metadata) {
			continue
		}
		imp := parseMetaFloat(doc.Metadata, metaBaseImportance, defaultBaseImportance)
		retr := parseMetaInt(doc.Metadata, metaRetrievalCount, 0)
		created := parseMetaTime(doc.Metadata, metaTimestamp)
		stale := 0.0
		if created != nil && !created.IsZero() && now.After(*created) {
			stale = now.Sub(*created).Hours()
		}
		// Lower score = lower signal.
		score := (imp * 0.7) + math.Min(float64(retr)/20.0, 0.3)
		cands = append(cands, candidate{doc: doc, score: score, staleHr: stale})
	}
	if len(cands) == 0 {
		return 0, nil
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score == cands[j].score {
			return cands[i].staleHr > cands[j].staleHr
		}
		return cands[i].score < cands[j].score
	})

	archived := 0
	for _, c := range cands {
		if archived >= maxArchive {
			break
		}
		if c.doc.Metadata == nil {
			c.doc.Metadata = map[string]string{}
		}
		c.doc.Metadata[metaArchived] = "true"
		c.doc.Metadata[metaArchiveReason] = "attention_pressure_prune"
		c.doc.Metadata[metaLastReindexedAt] = now.Format(time.RFC3339)
		if err := mm.historyCollection.AddDocument(context.Background(), c.doc); err != nil {
			continue
		}
		archived++
	}
	return archived, nil
}

// AdjustKnowledgeImportanceByID applies additive importance delta to a knowledge document.
func (mm *MemoryManager) AdjustKnowledgeImportanceByID(docID string, delta float64, reason string) error {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return fmt.Errorf("docID is empty")
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	doc, err := mm.knowledgeCollection.GetByID(context.Background(), docID)
	if err != nil {
		return err
	}
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]string)
	}
	cur := parseMetaFloat(doc.Metadata, metaBaseImportance, defaultBaseImportance)
	doc.Metadata[metaBaseImportance] = formatFloat(cur + delta)
	doc.Metadata["belief_shift_reason"] = strings.TrimSpace(reason)
	doc.Metadata["belief_shift_at"] = time.Now().UTC().Format(time.RFC3339)
	return mm.knowledgeCollection.AddDocument(context.Background(), doc)
}

// ApplyBeliefShift downgrades an old belief and elevates the winning new belief.
func (mm *MemoryManager) ApplyBeliefShift(loserID string, winnerID string, reason string) error {
	if strings.TrimSpace(loserID) == "" || strings.TrimSpace(winnerID) == "" {
		return fmt.Errorf("loserID and winnerID are required")
	}
	if err := mm.AdjustKnowledgeImportanceByID(loserID, -0.25, "downgraded: "+reason); err != nil {
		return err
	}
	if err := mm.AdjustKnowledgeImportanceByID(winnerID, +0.25, "elevated: "+reason); err != nil {
		return err
	}
	return nil
}

func (mm *MemoryManager) namespaceWhereFilter() map[string]string {
	ns := strings.TrimSpace(mm.ActiveNamespace())
	if ns == "" {
		return nil
	}
	return map[string]string{"namespace": ns}
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(clamp01(v), 'f', 4, 64)
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
