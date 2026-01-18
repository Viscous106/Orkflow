package vectorstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/philippgille/chromem-go"
)

// WorkflowVectorStore is a workflow-scoped vector store using ChromaDB.
// Each workflow run gets a unique collection based on UUID.
// Collections are kept after the run for debugging (not auto-deleted).
type WorkflowVectorStore struct {
	db             *chromem.DB
	collection     *chromem.Collection
	ctx            context.Context
	runID          string
	collectionName string
	persistPath    string
}

// WorkflowDocument represents a document stored in the workflow vector store
type WorkflowDocument struct {
	ID        string            // Unique identifier
	Content   string            // Text content
	AgentID   string            // Agent that created this document
	DocType   string            // "output", "message", "context"
	Timestamp time.Time         // When the document was created
	Metadata  map[string]string // Additional metadata
}

// NewWorkflowVectorStore creates a new vector store for a workflow run.
// Uses embedded ChromaDB mode (no external server required).
// runID should be a UUID to ensure isolation between runs.
func NewWorkflowVectorStore(persistPath string, runID string, embedder string) (*WorkflowVectorStore, error) {
	if runID == "" {
		runID = uuid.New().String()
	}

	if persistPath == "" {
		home, _ := os.UserHomeDir()
		persistPath = filepath.Join(home, ".orka/vectordb")
	}

	// Ensure directory exists
	if err := os.MkdirAll(persistPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create vectordb directory: %w", err)
	}

	ctx := context.Background()

	// Create persistent DB
	db, err := chromem.NewPersistentDB(persistPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create chromem db: %w", err)
	}

	// Get embedding function based on embedder type
	ef, err := getEmbeddingFunc(embedder)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding function: %w", err)
	}

	// Create collection with unique name for this run
	collectionName := fmt.Sprintf("workflow_run_%s", runID)
	collection, err := db.GetOrCreateCollection(collectionName, nil, ef)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	return &WorkflowVectorStore{
		db:             db,
		collection:     collection,
		ctx:            ctx,
		runID:          runID,
		collectionName: collectionName,
		persistPath:    persistPath,
	}, nil
}

// getEmbeddingFunc returns the appropriate embedding function based on embedder type
func getEmbeddingFunc(embedder string) (chromem.EmbeddingFunc, error) {
	switch embedder {
	case "ollama", "local", "":
		// Default to Ollama with nomic-embed-text model
		return chromem.NewEmbeddingFuncOllama("nomic-embed-text", ""), nil
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}
		return chromem.NewEmbeddingFuncOpenAI(apiKey, chromem.EmbeddingModelOpenAI3Small), nil
	case "gemini":
		// Gemini embeddings via Google AI
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY not set")
		}
		// Use OpenAI-compatible endpoint for Gemini (requires custom setup)
		// For now, fall back to Ollama
		return chromem.NewEmbeddingFuncOllama("nomic-embed-text", ""), nil
	default:
		return nil, fmt.Errorf("unknown embedder: %s", embedder)
	}
}

// StoreAgentOutput stores an agent's output in the vector store
func (w *WorkflowVectorStore) StoreAgentOutput(agentID string, content string) error {
	doc := chromem.Document{
		ID:      fmt.Sprintf("%s_output_%d", agentID, time.Now().UnixNano()),
		Content: content,
		Metadata: map[string]string{
			"agent_id":   agentID,
			"doc_type":   "output",
			"run_id":     w.runID,
			"timestamp":  time.Now().Format(time.RFC3339),
		},
	}
	return w.collection.AddDocument(w.ctx, doc)
}

// StoreMessage stores a message in the vector store
func (w *WorkflowVectorStore) StoreMessage(from, to, content string) error {
	doc := chromem.Document{
		ID:      fmt.Sprintf("msg_%s_%s_%d", from, to, time.Now().UnixNano()),
		Content: content,
		Metadata: map[string]string{
			"from":      from,
			"to":        to,
			"doc_type":  "message",
			"run_id":    w.runID,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}
	return w.collection.AddDocument(w.ctx, doc)
}

// Store stores a generic document
func (w *WorkflowVectorStore) Store(doc WorkflowDocument) error {
	metadata := doc.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["agent_id"] = doc.AgentID
	metadata["doc_type"] = doc.DocType
	metadata["run_id"] = w.runID
	metadata["timestamp"] = doc.Timestamp.Format(time.RFC3339)

	chromaDoc := chromem.Document{
		ID:       doc.ID,
		Content:  doc.Content,
		Metadata: metadata,
	}
	return w.collection.AddDocument(w.ctx, chromaDoc)
}

// Query finds similar documents
func (w *WorkflowVectorStore) Query(query string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	results, err := w.collection.Query(w.ctx, query, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}

	var searchResults []SearchResult
	for _, r := range results {
		searchResults = append(searchResults, SearchResult{
			ID:       r.ID,
			Content:  r.Content,
			Score:    r.Similarity,
			Metadata: r.Metadata,
		})
	}

	return searchResults, nil
}

// QueryRelevantContext retrieves context relevant to an agent's goal
func (w *WorkflowVectorStore) QueryRelevantContext(agentGoal string, topK int) ([]SearchResult, error) {
	return w.Query(agentGoal, topK)
}

// GetAgentHistory retrieves documents created by a specific agent
func (w *WorkflowVectorStore) GetAgentHistory(agentID string, limit int) ([]SearchResult, error) {
	// Query with agent filter
	filter := map[string]string{"agent_id": agentID}
	results, err := w.collection.Query(w.ctx, "", limit, filter, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent history: %w", err)
	}

	var searchResults []SearchResult
	for _, r := range results {
		searchResults = append(searchResults, SearchResult{
			ID:       r.ID,
			Content:  r.Content,
			Score:    r.Similarity,
			Metadata: r.Metadata,
		})
	}

	return searchResults, nil
}

// Close cleans up resources (collection is kept for debugging)
func (w *WorkflowVectorStore) Close() error {
	// Collections are kept after run for debugging per user decision
	// No explicit cleanup needed for chromem-go
	return nil
}

// GetRunID returns the unique run ID for this workflow
func (w *WorkflowVectorStore) GetRunID() string {
	return w.runID
}

// GetCollectionName returns the collection name
func (w *WorkflowVectorStore) GetCollectionName() string {
	return w.collectionName
}

// Count returns the number of documents in the collection
func (w *WorkflowVectorStore) Count() int {
	return w.collection.Count()
}
