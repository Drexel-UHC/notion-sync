package sync

import (
	"fmt"

	"github.com/ran-codes/notion-sync/internal/notion"
)

// mockNotionClient is a test double that returns pre-configured data by ID.
type mockNotionClient struct {
	databases      map[string]*notion.Database
	dataSources    map[string]*notion.DataSourceDetail
	entries        map[string][]notion.Page // keyed by dataSourceID
	pages          map[string]*notion.Page
	blocks         map[string][]notion.Block // keyed by blockID
	updateRequests []updateRequest           // recorded UpdatePage calls
	// updatePageReturns, when set for a page ID, is what UpdatePage returns
	// for that ID instead of pages[id]. Simulates Notion's real behavior
	// where UpdatePage echoes a minute-quantized last_edited_time while the
	// stored value (returned by GetPage / QueryDataSource) is precise.
	updatePageReturns map[string]*notion.Page
	// postUpdatePages, when set for a page ID, replaces pages[id] after a
	// successful UpdatePage call. Simulates Notion's stored state advancing
	// to a precise post-edit timestamp that subsequent GetPage calls see.
	postUpdatePages map[string]*notion.Page
}

type updateRequest struct {
	PageID     string
	Properties map[string]interface{}
}

func newMockClient() *mockNotionClient {
	return &mockNotionClient{
		databases:         make(map[string]*notion.Database),
		dataSources:       make(map[string]*notion.DataSourceDetail),
		entries:           make(map[string][]notion.Page),
		pages:             make(map[string]*notion.Page),
		blocks:            make(map[string][]notion.Block),
		updatePageReturns: make(map[string]*notion.Page),
		postUpdatePages:   make(map[string]*notion.Page),
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

func (m *mockNotionClient) UpdatePage(pageID string, properties map[string]interface{}) (*notion.Page, error) {
	m.updateRequests = append(m.updateRequests, updateRequest{PageID: pageID, Properties: properties})

	var response *notion.Page
	if override, ok := m.updatePageReturns[pageID]; ok {
		response = override
	} else {
		page, ok := m.pages[pageID]
		if !ok {
			return nil, fmt.Errorf("page %s not found", pageID)
		}
		response = page
	}

	// Simulate Notion's stored state advancing after a successful update.
	if post, ok := m.postUpdatePages[pageID]; ok {
		m.pages[pageID] = post
	}

	return response, nil
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
