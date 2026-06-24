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
	// updateErrors, when set for a page ID, makes UpdatePage return that error
	// for the row (the terminal outcome after the HTTP client's own retries).
	// Used to exercise the push loop's error classifier: per-row 4xx (n34c) vs
	// run-wide auth 401/403 (n34h).
	updateErrors map[string]error
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
		updateErrors:      make(map[string]error),
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

	// Injected terminal failure for this row — return before touching stored
	// state, mirroring Notion rejecting the write.
	if err, ok := m.updateErrors[pageID]; ok && err != nil {
		return nil, err
	}

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

	// Advance the stored state that subsequent GetPage calls observe. A test that
	// sets postUpdatePages controls the stored row verbatim (used to simulate a
	// read-back mismatch or a specific post-edit timestamp); otherwise the mock
	// simulates Notion storing exactly what we sent, so the read-back verify
	// (DAG n34d) sees the new values and passes.
	if post, ok := m.postUpdatePages[pageID]; ok {
		m.pages[pageID] = post
	} else if base, ok := m.pages[pageID]; ok {
		m.pages[pageID] = applyProps(clonePage(base), properties)
	}

	return response, nil
}

// clonePage deep-copies a page's Properties map so applyProps can mutate the
// stored copy without touching the fixture the test set up.
func clonePage(p *notion.Page) *notion.Page {
	cp := *p
	cp.Properties = make(map[string]notion.Property, len(p.Properties))
	for k, v := range p.Properties {
		cp.Properties[k] = v
	}
	return &cp
}

// applyProps decodes an UpdatePage payload (as built by buildPropertyValue) back
// into the page's Property structs, so a subsequent GetPage reflects what was
// sent. This is what lets the read-back verify (DAG n34d) be exercised against
// the mock instead of always trivially passing.
func applyProps(page *notion.Page, properties map[string]interface{}) *notion.Page {
	if page.Properties == nil {
		page.Properties = map[string]notion.Property{}
	}
	for key, raw := range properties {
		payload, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		prop := page.Properties[key]
		switch {
		case hasKey(payload, "title"):
			prop.Type, prop.Title = "title", decodeRichTextPayload(payload["title"])
		case hasKey(payload, "rich_text"):
			prop.Type, prop.RichText = "rich_text", decodeRichTextPayload(payload["rich_text"])
		case hasKey(payload, "number"):
			prop.Type, prop.Number = "number", decodeFloatPtr(payload["number"])
		case hasKey(payload, "select"):
			prop.Type, prop.Select = "select", decodeNamePtr(payload["select"])
		case hasKey(payload, "status"):
			prop.Type, prop.Status = "status", decodeNamePtr(payload["status"])
		case hasKey(payload, "multi_select"):
			prop.Type, prop.MultiSelect = "multi_select", decodeNameSlice(payload["multi_select"])
		case hasKey(payload, "date"):
			prop.Type, prop.Date = "date", decodeDatePayload(payload["date"])
		case hasKey(payload, "checkbox"):
			prop.Type = "checkbox"
			if b, ok := payload["checkbox"].(bool); ok {
				prop.Checkbox = b
			}
		case hasKey(payload, "url"):
			prop.Type, prop.URL = "url", decodeStrPtr(payload["url"])
		case hasKey(payload, "email"):
			prop.Type, prop.Email = "email", decodeStrPtr(payload["email"])
		case hasKey(payload, "phone_number"):
			prop.Type, prop.PhoneNumber = "phone_number", decodeStrPtr(payload["phone_number"])
		case hasKey(payload, "relation"):
			prop.Type, prop.Relation = "relation", decodeRelation(payload["relation"])
		}
		page.Properties[key] = prop
	}
	return page
}

func hasKey(m map[string]interface{}, k string) bool {
	_, ok := m[k]
	return ok
}

func decodeRichTextPayload(v interface{}) []notion.RichText {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []notion.RichText
	for _, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		text, _ := m["text"].(map[string]interface{})
		content, _ := text["content"].(string)
		out = append(out, notion.RichText{
			Type:      "text",
			PlainText: content,
			Text:      &notion.TextContent{Content: content},
		})
	}
	return out
}

func decodeFloatPtr(v interface{}) *float64 {
	if f, ok := v.(float64); ok {
		return &f
	}
	return nil
}

func decodeNamePtr(v interface{}) *notion.SelectValue {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	name, _ := m["name"].(string)
	return &notion.SelectValue{Name: name}
}

func decodeNameSlice(v interface{}) []notion.SelectValue {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []notion.SelectValue
	for _, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		out = append(out, notion.SelectValue{Name: name})
	}
	return out
}

func decodeDatePayload(v interface{}) *notion.DateValue {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	start, _ := m["start"].(string)
	dv := &notion.DateValue{Start: start}
	if end, ok := m["end"].(string); ok && end != "" {
		dv.End = &end
	}
	return dv
}

func decodeStrPtr(v interface{}) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

func decodeRelation(v interface{}) []notion.Relation {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []notion.Relation
	for _, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		out = append(out, notion.Relation{ID: id})
	}
	return out
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
