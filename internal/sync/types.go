package sync

// Version is set by main.go at startup from the build-time version string.
var Version string

// FrozenDatabase represents metadata stored in _database.json.
type FrozenDatabase struct {
	DatabaseID   string `json:"databaseId"`
	DataSourceID string `json:"dataSourceId,omitempty"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	FolderPath   string `json:"folderPath"`
	LastSyncedAt string `json:"lastSyncedAt"`
	EntryCount   int    `json:"entryCount"`
	SyncVersion  string `json:"syncVersion,omitempty"`
}

// FrozenPage represents metadata stored in _page.json for standalone pages.
type FrozenPage struct {
	PageID       string `json:"pageId"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	FolderPath   string `json:"folderPath"`
	LastSyncedAt string `json:"lastSyncedAt"`
	SyncVersion  string `json:"syncVersion,omitempty"`
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
	Status     string // "created", "updated", or "skipped"
	FilePath   string
	Title      string
	FolderPath string // set by FreezeStandalonePage for standalone pages
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
	PhasePushScanning  = "push-scanning"
	PhasePushing       = "pushing"
)

// PushOptions contains options for pushing local frontmatter changes to Notion.
type PushOptions struct {
	Client          NotionClient
	FolderPath      string
	Force           bool // skip conflict check
	DryRun          bool // report planned changes without writing
	AllowNewOptions bool // let unknown select/multi_select values auto-create options in Notion (status still validated)
}

// PushPreview is what the phase-1 confirmation gate displays to the user
// before any Notion write. Queue is the .md paths that would be pushed if
// validation passes; LocalHalts enumerates halts detectable without a
// Notion API call (stray .md, malformed YAML). Network-dependent halts
// (conflicts, unreachable) only surface later at the validation gate.
type PushPreview struct {
	Queue      []string
	LocalHalts []FileClassification
}

// PushResult contains the result of a push operation.
type PushResult struct {
	Title      string
	FolderPath string
	Total      int
	Pushed     int
	Skipped    int
	// SkippedNoOp counts rows whose local frontmatter already matched Notion's
	// stored values (DAG n32a) — nothing to push, no UpdatePage call made.
	// Distinct from Skipped, which counts rows with no pushable fields at all.
	SkippedNoOp   int
	Conflicts     int
	Failed        int
	Errors        []string
	ConflictFiles []string
	// Halted is true iff the validation gate (DAG n22a) aborted the run before
	// any Notion write. When true, Halts enumerates every halt-class file from
	// the validation pass so the caller can surface them all at once.
	Halted bool
	Halts  []FileClassification

	// AuthHalted is true iff a push write returned a run-wide auth failure
	// (401/403, DAG n34h). Unlike a validation halt this fires mid-run, so rows
	// pushed before it are real and already counted in Pushed. AuthError carries
	// the one-line reason+fix surfaced once, instead of once per remaining row.
	AuthHalted bool
	AuthError  string
}
