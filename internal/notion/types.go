package notion

import "time"

// Database represents a Notion database response.
type Database struct {
	Object      string                      `json:"object"`
	ID          string                      `json:"id"`
	CreatedTime time.Time                   `json:"created_time"`
	LastEdited  time.Time                   `json:"last_edited_time"`
	Title       []RichText                  `json:"title"`
	URL         string                      `json:"url"`
	DataSources []DataSource                `json:"data_sources,omitempty"`
	Properties  map[string]DatabaseProperty `json:"properties,omitempty"`
}

// DatabaseProperty represents a property schema entry in a database.
type DatabaseProperty struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// DataSource represents a data source reference within a database.
type DataSource struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// DataSourceDetail represents the full data source response from GET /data_sources/{id}.
type DataSourceDetail struct {
	ID    string     `json:"id"`
	Type  string     `json:"type"`
	Title []RichText `json:"title,omitempty"`
}

// Page represents a Notion page response.
type Page struct {
	Object         string              `json:"object"`
	ID             string              `json:"id"`
	CreatedTime    time.Time           `json:"created_time"`
	LastEditedTime string              `json:"last_edited_time"`
	URL            string              `json:"url"`
	Properties     map[string]Property `json:"properties"`
	Parent         Parent              `json:"parent"`
}

// Parent represents the parent of a page.
type Parent struct {
	Type       string `json:"type"`
	DatabaseID string `json:"database_id,omitempty"`
	PageID     string `json:"page_id,omitempty"`
}

// Property represents a page property with its type and value.
type Property struct {
	ID             string       `json:"id"`
	Type           string       `json:"type"`
	Title          []RichText   `json:"title,omitempty"`
	RichText       []RichText   `json:"rich_text,omitempty"`
	Number         *float64     `json:"number,omitempty"`
	Select         *SelectValue `json:"select,omitempty"`
	MultiSelect    []SelectValue `json:"multi_select,omitempty"`
	Status         *SelectValue `json:"status,omitempty"`
	Date           *DateValue   `json:"date,omitempty"`
	Checkbox       bool         `json:"checkbox,omitempty"`
	URL            *string      `json:"url,omitempty"`
	Email          *string      `json:"email,omitempty"`
	PhoneNumber    *string      `json:"phone_number,omitempty"`
	Relation       []Relation   `json:"relation,omitempty"`
	People         []Person     `json:"people,omitempty"`
	Files          []File       `json:"files,omitempty"`
	CreatedTime    string         `json:"created_time,omitempty"`
	LastEditedTime string         `json:"last_edited_time,omitempty"`
	UniqueID       *UniqueIDValue `json:"unique_id,omitempty"`
	CreatedBy      *Person        `json:"created_by,omitempty"`
	LastEditedBy   *Person        `json:"last_edited_by,omitempty"`
}

// UniqueIDValue represents a unique_id property value.
type UniqueIDValue struct {
	Prefix string `json:"prefix"`
	Number int    `json:"number"`
}

// SelectValue represents a select or status option.
type SelectValue struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// DateValue represents a date property value.
type DateValue struct {
	Start    string  `json:"start"`
	End      *string `json:"end,omitempty"`
	TimeZone *string `json:"time_zone,omitempty"`
}

// Relation represents a relation property item.
type Relation struct {
	ID string `json:"id"`
}

// Person represents a user mention.
type Person struct {
	ID   string  `json:"id"`
	Name *string `json:"name,omitempty"`
}

// File represents a file property item.
type File struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	File     *FileURL      `json:"file,omitempty"`
	External *ExternalURL  `json:"external,omitempty"`
}

// FileURL represents an internal Notion file URL.
type FileURL struct {
	URL        string    `json:"url"`
	ExpiryTime time.Time `json:"expiry_time,omitempty"`
}

// ExternalURL represents an external file URL.
type ExternalURL struct {
	URL string `json:"url"`
}

// Block represents a Notion block response.
type Block struct {
	Object         string `json:"object"`
	ID             string `json:"id"`
	Type           string `json:"type"`
	HasChildren    bool   `json:"has_children"`

	// Block type-specific content
	Paragraph        *ParagraphBlock    `json:"paragraph,omitempty"`
	Heading1         *HeadingBlock      `json:"heading_1,omitempty"`
	Heading2         *HeadingBlock      `json:"heading_2,omitempty"`
	Heading3         *HeadingBlock      `json:"heading_3,omitempty"`
	BulletedListItem *ListItemBlock     `json:"bulleted_list_item,omitempty"`
	NumberedListItem *ListItemBlock     `json:"numbered_list_item,omitempty"`
	ToDo             *ToDoBlock         `json:"to_do,omitempty"`
	Toggle           *ToggleBlock       `json:"toggle,omitempty"`
	Code             *CodeBlock         `json:"code,omitempty"`
	Quote            *QuoteBlock        `json:"quote,omitempty"`
	Callout          *CalloutBlock      `json:"callout,omitempty"`
	Equation         *EquationBlock     `json:"equation,omitempty"`
	Divider          *struct{}          `json:"divider,omitempty"`
	TableOfContents  *struct{}          `json:"table_of_contents,omitempty"`
	Breadcrumb       *struct{}          `json:"breadcrumb,omitempty"`
	ChildPage        *ChildPageBlock    `json:"child_page,omitempty"`
	ChildDatabase    *ChildDatabaseBlock `json:"child_database,omitempty"`
	Image            *MediaBlock        `json:"image,omitempty"`
	Video            *MediaBlock        `json:"video,omitempty"`
	Audio            *MediaBlock        `json:"audio,omitempty"`
	File             *MediaBlock        `json:"file,omitempty"`
	PDF              *MediaBlock        `json:"pdf,omitempty"`
	Bookmark         *BookmarkBlock     `json:"bookmark,omitempty"`
	Embed            *EmbedBlock        `json:"embed,omitempty"`
	LinkToPage       *LinkToPageBlock   `json:"link_to_page,omitempty"`
	SyncedBlock      *SyncedBlockContent `json:"synced_block,omitempty"`
	Table            *TableBlock        `json:"table,omitempty"`
	TableRow         *TableRowBlock     `json:"table_row,omitempty"`
	ColumnList       *struct{}          `json:"column_list,omitempty"`
	Column           *struct{}          `json:"column,omitempty"`
}

// ParagraphBlock represents paragraph content.
type ParagraphBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
}

// HeadingBlock represents heading content.
type HeadingBlock struct {
	RichText     []RichText `json:"rich_text"`
	Color        string     `json:"color,omitempty"`
	IsToggleable bool       `json:"is_toggleable"`
}

// ListItemBlock represents list item content.
type ListItemBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
}

// ToDoBlock represents a to-do item.
type ToDoBlock struct {
	RichText []RichText `json:"rich_text"`
	Checked  bool       `json:"checked"`
	Color    string     `json:"color,omitempty"`
}

// ToggleBlock represents a toggle block.
type ToggleBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
}

// CodeBlock represents a code block.
type CodeBlock struct {
	RichText []RichText `json:"rich_text"`
	Caption  []RichText `json:"caption,omitempty"`
	Language string     `json:"language"`
}

// QuoteBlock represents a quote block.
type QuoteBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
}

// CalloutBlock represents a callout block.
type CalloutBlock struct {
	RichText []RichText `json:"rich_text"`
	Icon     *Icon      `json:"icon,omitempty"`
	Color    string     `json:"color,omitempty"`
}

// Icon represents a block icon (emoji or external).
type Icon struct {
	Type     string       `json:"type"`
	Emoji    string       `json:"emoji,omitempty"`
	External *ExternalURL `json:"external,omitempty"`
}

// EquationBlock represents an equation block.
type EquationBlock struct {
	Expression string `json:"expression"`
}

// ChildPageBlock represents a child page reference.
type ChildPageBlock struct {
	Title string `json:"title"`
}

// ChildDatabaseBlock represents a child database reference.
type ChildDatabaseBlock struct {
	Title string `json:"title"`
}

// MediaBlock represents image, video, audio, file, or PDF blocks.
type MediaBlock struct {
	Type     string       `json:"type"`
	Caption  []RichText   `json:"caption,omitempty"`
	File     *FileURL     `json:"file,omitempty"`
	External *ExternalURL `json:"external,omitempty"`
}

// BookmarkBlock represents a bookmark.
type BookmarkBlock struct {
	URL     string     `json:"url"`
	Caption []RichText `json:"caption,omitempty"`
}

// EmbedBlock represents an embed.
type EmbedBlock struct {
	URL     string     `json:"url"`
	Caption []RichText `json:"caption,omitempty"`
}

// LinkToPageBlock represents a link to another page or database.
type LinkToPageBlock struct {
	Type       string `json:"type"`
	PageID     string `json:"page_id,omitempty"`
	DatabaseID string `json:"database_id,omitempty"`
}

// SyncedBlockContent represents a synced block.
type SyncedBlockContent struct {
	SyncedFrom *SyncedFrom `json:"synced_from,omitempty"`
}

// SyncedFrom represents the original synced block.
type SyncedFrom struct {
	Type    string `json:"type"`
	BlockID string `json:"block_id"`
}

// TableBlock represents a table.
type TableBlock struct {
	TableWidth      int  `json:"table_width"`
	HasColumnHeader bool `json:"has_column_header"`
	HasRowHeader    bool `json:"has_row_header"`
}

// TableRowBlock represents a table row.
type TableRowBlock struct {
	Cells [][]RichText `json:"cells"`
}

// RichText represents a rich text item.
type RichText struct {
	Type        string       `json:"type"`
	PlainText   string       `json:"plain_text"`
	Annotations Annotations  `json:"annotations"`
	Href        *string      `json:"href,omitempty"`
	Text        *TextContent `json:"text,omitempty"`
	Mention     *Mention     `json:"mention,omitempty"`
	Equation    *Equation    `json:"equation,omitempty"`
}

// TextContent represents plain text content.
type TextContent struct {
	Content string `json:"content"`
	Link    *Link  `json:"link,omitempty"`
}

// Link represents a link in text.
type Link struct {
	URL string `json:"url"`
}

// Mention represents a mention in rich text.
type Mention struct {
	Type        string       `json:"type"`
	Page        *PageMention `json:"page,omitempty"`
	Database    *DatabaseMention `json:"database,omitempty"`
	Date        *DateValue   `json:"date,omitempty"`
	User        *Person      `json:"user,omitempty"`
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
}

// PageMention represents a page mention.
type PageMention struct {
	ID string `json:"id"`
}

// DatabaseMention represents a database mention.
type DatabaseMention struct {
	ID string `json:"id"`
}

// LinkPreview represents a link preview mention.
type LinkPreview struct {
	URL string `json:"url"`
}

// Equation represents an inline equation.
type Equation struct {
	Expression string `json:"expression"`
}

// Annotations represents text formatting.
type Annotations struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

// ListResponse represents a paginated list response.
type ListResponse[T any] struct {
	Object     string  `json:"object"`
	Results    []T     `json:"results"`
	NextCursor *string `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
}

// BlockListResponse is a list response for blocks.
type BlockListResponse = ListResponse[Block]

// PageListResponse is a list response for pages (from dataSources.query).
type PageListResponse = ListResponse[Page]

// ErrorResponse represents a Notion API error.
type ErrorResponse struct {
	Object  string `json:"object"`
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ErrorResponse) Error() string {
	return e.Message
}
