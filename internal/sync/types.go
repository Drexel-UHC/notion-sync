package sync

// FrozenDatabase represents metadata stored in _database.json.
type FrozenDatabase struct {
	DatabaseID   string `json:"databaseId"`
	DataSourceID string `json:"dataSourceId,omitempty"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	FolderPath   string `json:"folderPath"`
	LastSyncedAt string `json:"lastSyncedAt"`
	EntryCount   int    `json:"entryCount"`
}

// DatabaseFreezeResult represents the result of a database sync operation.
type DatabaseFreezeResult struct {
	Title      string
	FolderPath string
	Total      int
	Created    int
	Updated    int
	Skipped    int
	Deleted    int
	Failed     int
	Errors     []string
}

// PageFreezeResult represents the result of freezing a single page.
type PageFreezeResult struct {
	Status   string // "created", "updated", or "skipped"
	FilePath string
	Title    string
}

// ProgressPhase represents the current phase of a sync operation.
type ProgressPhase struct {
	Phase   string // "querying", "diffing", "stale-detected", "importing", "complete"
	Total   int
	Stale   int
	Current int
	Title   string
}

// ProgressCallback is called during sync to report progress.
type ProgressCallback func(ProgressPhase)

const (
	PhaseQuerying      = "querying"
	PhaseDiffing       = "diffing"
	PhaseStaleDetected = "stale-detected"
	PhaseImporting     = "importing"
	PhaseComplete      = "complete"
)

// OutputMode controls what outputs are produced during sync.
type OutputMode string

const (
	OutputBoth     OutputMode = "both"     // Write markdown files and SQLite (default)
	OutputMarkdown OutputMode = "markdown" // Write markdown files only
	OutputSQLite   OutputMode = "sqlite"   // Write SQLite only
)
