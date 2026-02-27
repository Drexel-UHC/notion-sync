package sync

import "github.com/ran-codes/notion-sync/internal/notion"

// NotionClient abstracts the Notion API methods used by sync operations.
// *notion.Client satisfies this interface. Tests can provide a mock.
type NotionClient interface {
	GetDatabase(databaseID string) (*notion.Database, error)
	GetDataSource(dataSourceID string) (*notion.DataSourceDetail, error)
	QueryAllEntries(dataSourceID string) ([]notion.Page, error)
	GetPage(pageID string) (*notion.Page, error)
	FetchAllBlocks(blockID string) ([]notion.Block, error)
	FetchBlockTree(pageID string, progress func(fetched, found int)) (*notion.BlockTree, error)
}
