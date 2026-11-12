package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// --- Create rows from `notion-create: true` files (issue #117) ---

// setupCreateFolder writes a data-source-backed _database.json and returns a
// mock whose schema has a title, a select with fixed options, and a number.
func setupCreateFolder(t *testing.T) (string, *mockNotionClient) {
	t.Helper()
	dir := t.TempDir()
	writeDatabaseMetaWithDataSource(t, dir, "db-001", "ds-001")
	client := newMockClient()
	client.dataSources["ds-001"] = &notion.DataSourceDetail{
		ID: "ds-001",
		Properties: map[string]notion.DatabaseProperty{
			"Name":   {Type: "title"},
			"Status": {Type: "select", Select: &notion.SelectConfig{Options: []notion.SelectValue{{Name: "Todo"}, {Name: "Done"}}}},
			"Score":  {Type: "number"},
		},
	}
	return dir, client
}

func writeMD(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readMD(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const createMarkerMD = "---\nnotion-create: true\nName: New row\nStatus: Todo\nScore: 3\n---\n"

// (b) A file with neither notion-id nor the marker is still a stray halt.
func TestPushDatabase_Create_StrayWithoutMarkerStillHalts(t *testing.T) {
	dir, client := setupCreateFolder(t)
	writeMD(t, dir, "stray.md", "---\nName: Stray\n---\n")

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Halted || len(result.Halts) != 1 || result.Halts[0].Class != ClassHaltUnexpected {
		t.Fatalf("expected one ClassHaltUnexpected halt, got halted=%v halts=%+v", result.Halted, result.Halts)
	}
	if len(client.createRequests) != 0 {
		t.Errorf("expected no CreatePage calls, got %d", len(client.createRequests))
	}
}

// (c) A `pending` file halts the whole run before any write — a previous
// create may have reached Notion, so re-creating would duplicate the row.
func TestPushDatabase_CreatePending_HaltsRunBeforeAnyWrite(t *testing.T) {
	dir, client := setupCreateFolder(t)
	writeMD(t, dir, "pending.md", "---\nnotion-create: pending\nName: Maybe created\n---\n")
	writeMD(t, dir, "fresh.md", createMarkerMD)

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Halted {
		t.Fatal("expected the run to halt on a pending file")
	}
	var found bool
	for _, h := range result.Halts {
		if h.Class == ClassHaltCreatePending && filepath.Base(h.Path) == "pending.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ClassHaltCreatePending for pending.md, got %+v", result.Halts)
	}
	if len(client.createRequests) != 0 {
		t.Errorf("expected no CreatePage calls, got %d", len(client.createRequests))
	}
	if got := result.Summary().Halted[0].Fix; !strings.Contains(got, "refresh") {
		t.Errorf("expected the pending fix to point at refresh, got %q", got)
	}
}

// (g) The marker on a file that already has a notion-id is contradictory.
func TestPushDatabase_Create_MarkerWithNotionIDHalts(t *testing.T) {
	dir, client := setupCreateFolder(t)
	writeMD(t, dir, "both.md", "---\nnotion-id: page-001\nnotion-create: true\nName: Both\n---\n")

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Halted || len(result.Halts) != 1 || result.Halts[0].Class != ClassHaltMalformed {
		t.Fatalf("expected one ClassHaltMalformed halt, got halted=%v halts=%+v", result.Halted, result.Halts)
	}
	if !strings.Contains(result.Halts[0].Reason, "notion-create") {
		t.Errorf("expected reason to name notion-create, got %q", result.Halts[0].Reason)
	}
	if len(client.createRequests) != 0 || len(client.updateRequests) != 0 {
		t.Error("expected no writes")
	}
}

// (i) Option validation runs on creates too — an unknown select value would
// otherwise auto-create a junk option in the shared schema.
func TestPushDatabase_Create_UnknownOptionHalts(t *testing.T) {
	dir, client := setupCreateFolder(t)
	writeMD(t, dir, "new.md", "---\nnotion-create: true\nName: New\nStatus: Bogus\n---\n")

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Halted || len(result.Halts) != 1 || result.Halts[0].Class != ClassHaltInvalidOption {
		t.Fatalf("expected one ClassHaltInvalidOption halt, got halted=%v halts=%+v", result.Halted, result.Halts)
	}
	if len(client.createRequests) != 0 {
		t.Error("expected no CreatePage calls")
	}
}

// (a) A marker file becomes one new row; the file is linked in place.
func TestPushDatabase_Create_MarkerFileCreatesRowAndLinksFile(t *testing.T) {
	dir, client := setupCreateFolder(t)
	p := writeMD(t, dir, "new.md", createMarkerMD)

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Halted {
		t.Fatalf("unexpected halt: %+v", result.Halts)
	}
	if len(client.createRequests) != 1 {
		t.Fatalf("expected 1 CreatePage call, got %d", len(client.createRequests))
	}
	req := client.createRequests[0]
	if req.DataSourceID != "ds-001" {
		t.Errorf("expected data source parent ds-001, got %q", req.DataSourceID)
	}
	for _, k := range []string{"Name", "Status", "Score"} {
		if _, ok := req.Properties[k]; !ok {
			t.Errorf("expected %s in create payload", k)
		}
	}
	if _, ok := req.Properties["notion-create"]; ok {
		t.Error("marker must never be sent as a property")
	}
	if len(client.updateRequests) != 0 {
		t.Errorf("expected no UpdatePage calls, got %d", len(client.updateRequests))
	}

	got := readMD(t, p)
	for _, want := range []string{
		"notion-id: created-1\n",
		"notion-url: \"https://www.notion.so/created-1\"\n",
		"notion-last-edited: \"2025-01-01T00:00:00.000Z\"\n",
		"notion-database-id: db-001\n",
		"notion-last-pushed: ",
		"Name: New row\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in written file, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "notion-create") {
		t.Errorf("marker should be removed after a confirmed create, got:\n%s", got)
	}
	if !strings.HasPrefix(got, "---\nnotion-id: created-1\n") {
		t.Errorf("expected notion-id first in frontmatter, got:\n%s", got)
	}

	// Linked file is an ordinary row now: a second push is a clean no-op.
	result2, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result2.Halted || len(client.createRequests) != 1 {
		t.Errorf("second push should not halt or re-create (halted=%v creates=%d)", result2.Halted, len(client.createRequests))
	}
}

// (a) Legacy metadata without a data source falls back to the database parent.
func TestPushDatabase_Create_LegacyMetadataUsesDatabaseParent(t *testing.T) {
	dir := t.TempDir()
	writeDatabaseMeta(t, dir, "db-001")
	client := newMockClient()
	client.databases["db-001"] = &notion.Database{
		ID:         "db-001",
		Properties: map[string]notion.DatabaseProperty{"Name": {Type: "title"}},
	}
	writeMD(t, dir, "new.md", "---\nnotion-create: true\nName: Legacy\n---\n")

	if _, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil); err != nil {
		t.Fatal(err)
	}
	if len(client.createRequests) != 1 {
		t.Fatalf("expected 1 CreatePage call, got %d", len(client.createRequests))
	}
	if r := client.createRequests[0]; r.DataSourceID != "" || r.DatabaseID != "db-001" {
		t.Errorf("expected database parent db-001, got %+v", r)
	}
}

// (d) A failed POST leaves the `pending` stamp; the run continues and the
// next run halts instead of re-creating.
func TestPushDatabase_CreatePending_PostErrorLeavesStampAndRerunHalts(t *testing.T) {
	dir, client := setupCreateFolder(t)
	client.createErrors[1] = &notion.ErrorResponse{Status: 500, Message: "boom"}
	p := writeMD(t, dir, "a.md", "---\nnotion-create: true\nName: A\n---\n")
	writeMD(t, dir, "b.md", "---\nnotion-create: true\nName: B\n---\n")

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created != 1 || result.Failed != 1 {
		t.Errorf("expected 1 created + 1 failed, got created=%d failed=%d", result.Created, result.Failed)
	}
	if len(client.createRequests) != 2 {
		t.Errorf("expected the run to continue to the second create, got %d calls", len(client.createRequests))
	}
	if got := readMD(t, p); !strings.Contains(got, "notion-create: pending\n") {
		t.Errorf("expected pending stamp to remain, got:\n%s", got)
	}
	s := result.Summary()
	if len(s.Failed) != 1 || s.Failed[0].File != "a.md" {
		t.Errorf("expected summary failed to name a.md, got %+v", s.Failed)
	}

	before := len(client.createRequests)
	result2, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result2.Halted || len(client.createRequests) != before {
		t.Errorf("re-run must halt without creating (halted=%v, new creates=%d)", result2.Halted, len(client.createRequests)-before)
	}
}

// (e) A failed write-back after a successful POST leaves the `pending` stamp.
func TestPushDatabase_CreatePending_WriteBackErrorLeavesStamp(t *testing.T) {
	dir, client := setupCreateFolder(t)
	p := writeMD(t, dir, "new.md", createMarkerMD)

	orig := finalizeCreateFn
	finalizeCreateFn = func(string, string, string, string, string, string) error {
		return os.ErrPermission
	}
	defer func() { finalizeCreateFn = orig }()

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.createRequests) != 1 {
		t.Fatalf("expected the POST to fire, got %d", len(client.createRequests))
	}
	if result.Failed != 1 || result.Created != 0 {
		t.Errorf("expected write-back failure to count as failed, got created=%d failed=%d", result.Created, result.Failed)
	}
	if got := readMD(t, p); !strings.Contains(got, "notion-create: pending\n") {
		t.Errorf("expected pending stamp to remain, got:\n%s", got)
	}
}

// (f) --dry-run reports the create but sends nothing and stamps nothing.
func TestPushDatabase_DryRun_CreateWritesNothing(t *testing.T) {
	dir, client := setupCreateFolder(t)
	p := writeMD(t, dir, "new.md", createMarkerMD)

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir, DryRun: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.createRequests) != 0 {
		t.Errorf("expected no CreatePage calls in dry-run, got %d", len(client.createRequests))
	}
	if got := readMD(t, p); got != createMarkerMD {
		t.Errorf("dry-run must not touch the file, got:\n%s", got)
	}
	if result.Created != 1 || len(result.CreatedRows) != 1 || result.CreatedRows[0].File != "new.md" {
		t.Errorf("expected dry-run to report 1 create for new.md, got created=%d rows=%+v", result.Created, result.CreatedRows)
	}
}

// (h) A create without the title property is a per-row failure, not a POST.
func TestPushDatabase_Create_MissingTitleFailsRow(t *testing.T) {
	dir, client := setupCreateFolder(t)
	p := writeMD(t, dir, "untitled.md", "---\nnotion-create: true\nScore: 1\n---\n")

	result, err := PushDatabase(PushOptions{Client: client, FolderPath: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.createRequests) != 0 {
		t.Errorf("expected no CreatePage calls, got %d", len(client.createRequests))
	}
	if result.Failed != 1 || len(result.FailedRows) != 1 || !strings.Contains(result.FailedRows[0].Fix, "Name") {
		t.Errorf("expected a failed row whose fix names the title property, got %+v", result.FailedRows)
	}
	if got := readMD(t, p); !strings.Contains(got, "notion-create: true\n") {
		t.Errorf("a pre-POST failure must not stamp pending, got:\n%s", got)
	}
}

// (j) The network-free preview lists creates apart from the update queue and
// surfaces a pending file as a local halt at consent time.
func TestBuildPushQueue_Create_ListsCreatesAndPendingHalt(t *testing.T) {
	dir, _ := setupCreateFolder(t)
	writeMD(t, dir, "linked.md", "---\nnotion-id: page-001\nName: Linked\n---\n")
	writeMD(t, dir, "new.md", createMarkerMD)
	writeMD(t, dir, "pending.md", "---\nnotion-create: pending\nName: P\n---\n")

	preview, err := BuildPushQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Queue) != 1 || filepath.Base(preview.Queue[0]) != "linked.md" {
		t.Errorf("queue = %v, want [linked.md]", preview.Queue)
	}
	if len(preview.Creates) != 1 || filepath.Base(preview.Creates[0]) != "new.md" {
		t.Errorf("creates = %v, want [new.md]", preview.Creates)
	}
	if len(preview.LocalHalts) != 1 || preview.LocalHalts[0].Class != ClassHaltCreatePending {
		t.Errorf("local halts = %+v, want one ClassHaltCreatePending", preview.LocalHalts)
	}
}
