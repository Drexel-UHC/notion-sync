package sync

import (
	"fmt"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// mockNotionClient is a test double that returns pre-configured data by ID.
type mockNotionClient struct {
	databases   map[string]*notion.Database
	dataSources map[string]*notion.DataSourceDetail
	entries     map[string][]notion.Page // keyed by dataSourceID
	pages       map[string]*notion.Page
	blocks      map[string][]notion.Block // keyed by blockID
}

func newMockClient() *mockNotionClient {
	return &mockNotionClient{
		databases:   make(map[string]*notion.Database),
		dataSources: make(map[string]*notion.DataSourceDetail),
		entries:     make(map[string][]notion.Page),
		pages:       make(map[string]*notion.Page),
		blocks:      make(map[string][]notion.Block),
	}
}

func (m *mockNotionClient) GetDatabase(databaseID string) (*notion.Database, error) {
	db, ok := m.databases[databaseID]
	if !ok {
		return nil, fmt.Errorf("database %s not found", databaseID)
	}
	return db, nil
}

func (m *mockNotionClient) GetDataSource(dataSourceID string) (*notion.DataSourceDetail, error) {
	ds, ok := m.dataSources[dataSourceID]
	if !ok {
		return nil, fmt.Errorf("data source %s not found", dataSourceID)
	}
	return ds, nil
}

func (m *mockNotionClient) QueryAllEntries(dataSourceID string) ([]notion.Page, error) {
	entries, ok := m.entries[dataSourceID]
	if !ok {
		return nil, fmt.Errorf("data source %s not found", dataSourceID)
	}
	return entries, nil
}

func (m *mockNotionClient) GetPage(pageID string) (*notion.Page, error) {
	page, ok := m.pages[pageID]
	if !ok {
		return nil, fmt.Errorf("page %s not found", pageID)
	}
	return page, nil
}

func (m *mockNotionClient) FetchAllBlocks(blockID string) ([]notion.Block, error) {
	blocks, ok := m.blocks[blockID]
	if !ok {
		return []notion.Block{}, nil // empty blocks is valid
	}
	return blocks, nil
}

func (m *mockNotionClient) FetchBlockTree(pageID string, progress func(fetched, found int)) (*notion.BlockTree, error) {
	tree := &notion.BlockTree{Children: make(map[string][]notion.Block)}
	var fetchRecursive func(id string) error
	fetchRecursive = func(id string) error {
		blocks, err := m.FetchAllBlocks(id)
		if err != nil {
			return err
		}
		tree.Children[id] = blocks
		for _, b := range blocks {
			if b.HasChildren {
				if err := fetchRecursive(b.ID); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := fetchRecursive(pageID); err != nil {
		return nil, err
	}
	return tree, nil
}

// Helper to create a simple test page with a title.
func testPage(id, title, lastEdited string) notion.Page {
	return notion.Page{
		Object:         "page",
		ID:             id,
		LastEditedTime: lastEdited,
		URL:            "https://notion.so/" + id,
		Properties: map[string]notion.Property{
			"Name": {
				Type: "title",
				Title: []notion.RichText{
					{Type: "text", PlainText: title, Text: &notion.TextContent{Content: title}},
				},
			},
		},
	}
}
