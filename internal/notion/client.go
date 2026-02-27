// Package notion provides a thin HTTP client for the Notion API
// with rate limiting and automatic retry logic.
package notion

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	baseURL              = "https://api.notion.com/v1"
	notionVersion        = "2025-09-03"
	maxRetries           = 5
	minRequestIntervalMs = 340 // ~3 requests per second
	maxBackoffMs         = 30000
)

// Client is a Notion API client with rate limiting and retry logic.
// Rate limiting allows multiple requests in-flight simultaneously
// while ensuring new requests are only started ~3 per second.
type Client struct {
	apiKey     string
	httpClient *http.Client
	rateCh     chan struct{} // ticker-based rate limiter
	stopRate   chan struct{} // signal to stop the rate limiter
}

// NewClient creates a new Notion API client.
func NewClient(apiKey string) *Client {
	c := &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		rateCh:     make(chan struct{}, 1),
		stopRate:   make(chan struct{}),
	}
	// Start rate limiter: emit one token every 340ms (~3 req/sec).
	// Multiple requests can be in-flight, but new ones only start at this rate.
	go func() {
		ticker := time.NewTicker(time.Duration(minRequestIntervalMs) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				select {
				case c.rateCh <- struct{}{}:
				default: // don't accumulate tokens
				}
			case <-c.stopRate:
				return
			}
		}
	}()
	return c
}

// Close stops the rate limiter goroutine.
func (c *Client) Close() {
	select {
	case <-c.stopRate:
	default:
		close(c.stopRate)
	}
}

// throttle waits for the next rate limiter token before starting a request.
// Multiple goroutines can wait concurrently; each proceeds as a token arrives.
func (c *Client) throttle() {
	<-c.rateCh
}

// request makes an HTTP request with retry logic.
func (c *Client) request(method, endpoint string, body interface{}) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		c.throttle()

		var reqBody io.Reader
		if body != nil {
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal request body: %w", err)
			}
			reqBody = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequest(method, baseURL+endpoint, reqBody)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Notion-Version", notionVersion)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		// Check if retryable
		isRetryable := resp.StatusCode == 429 ||
			resp.StatusCode == 500 ||
			resp.StatusCode == 502 ||
			resp.StatusCode == 503 ||
			resp.StatusCode == 504

		if !isRetryable || attempt == maxRetries-1 {
			var errResp ErrorResponse
			if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
				errResp.Status = resp.StatusCode
				return nil, &errResp
			}
			return nil, fmt.Errorf("notion API error: status %d, body: %s", resp.StatusCode, string(respBody))
		}

		// Calculate backoff delay
		var delay time.Duration
		if resp.StatusCode == 429 {
			// Respect Retry-After header (value is in seconds)
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.ParseFloat(retryAfter, 64); err == nil {
					delay = time.Duration(seconds * float64(time.Second))
				}
			}
		}
		if delay == 0 {
			delay = time.Duration(math.Pow(2, float64(attempt))) * time.Second
		}

		// Add jitter: ±25% randomization
		jitter := float64(delay) * 0.25 * (rand.Float64()*2 - 1)
		delay = time.Duration(float64(delay) + jitter)
		if delay > maxBackoffMs*time.Millisecond {
			delay = maxBackoffMs * time.Millisecond
		}

		fmt.Fprintf(os.Stderr, "Notion API %d. Retrying in %dms (attempt %d/%d)\n",
			resp.StatusCode, delay.Milliseconds(), attempt+1, maxRetries)

		time.Sleep(delay)
		lastErr = fmt.Errorf("retryable error: status %d", resp.StatusCode)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("exhausted retries")
}

// GetDatabase retrieves a database by ID.
func (c *Client) GetDatabase(databaseID string) (*Database, error) {
	respBody, err := c.request("GET", "/databases/"+databaseID, nil)
	if err != nil {
		return nil, err
	}

	var db Database
	if err := json.Unmarshal(respBody, &db); err != nil {
		return nil, fmt.Errorf("unmarshal database: %w", err)
	}
	return &db, nil
}

// GetDataSource retrieves a data source by ID.
func (c *Client) GetDataSource(dataSourceID string) (*DataSourceDetail, error) {
	respBody, err := c.request("GET", "/data_sources/"+dataSourceID, nil)
	if err != nil {
		return nil, err
	}

	var ds DataSourceDetail
	if err := json.Unmarshal(respBody, &ds); err != nil {
		return nil, fmt.Errorf("unmarshal data source: %w", err)
	}
	return &ds, nil
}

// QueryDataSource queries entries from a data source.
func (c *Client) QueryDataSource(dataSourceID string, startCursor *string) (*PageListResponse, error) {
	body := map[string]interface{}{
		"page_size": 100,
	}
	if startCursor != nil {
		body["start_cursor"] = *startCursor
	}

	respBody, err := c.request("POST", "/data_sources/"+dataSourceID+"/query", body)
	if err != nil {
		return nil, err
	}

	var result PageListResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal query response: %w", err)
	}
	return &result, nil
}

// GetPage retrieves a page by ID.
func (c *Client) GetPage(pageID string) (*Page, error) {
	respBody, err := c.request("GET", "/pages/"+pageID, nil)
	if err != nil {
		return nil, err
	}

	var page Page
	if err := json.Unmarshal(respBody, &page); err != nil {
		return nil, fmt.Errorf("unmarshal page: %w", err)
	}
	return &page, nil
}

// ListBlockChildren lists children of a block.
func (c *Client) ListBlockChildren(blockID string, startCursor *string) (*BlockListResponse, error) {
	endpoint := "/blocks/" + blockID + "/children?page_size=100"
	if startCursor != nil {
		endpoint += "&start_cursor=" + *startCursor
	}

	respBody, err := c.request("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result BlockListResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal blocks: %w", err)
	}
	return &result, nil
}

// FetchAllBlocks fetches all blocks for a page (handling pagination).
func (c *Client) FetchAllBlocks(blockID string) ([]Block, error) {
	var blocks []Block
	var cursor *string

	for {
		resp, err := c.ListBlockChildren(blockID, cursor)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, resp.Results...)

		if !resp.HasMore || resp.NextCursor == nil {
			break
		}
		cursor = resp.NextCursor
	}

	return blocks, nil
}

// BlockTree holds all blocks for a page, keyed by parent block ID.
type BlockTree struct {
	Children map[string][]Block // parentID → children
}

// FetchBlockTree recursively fetches the entire block tree for a page,
// using concurrent requests (up to 3 at a time) for child blocks.
// Reports progress via the optional callback: (fetchedCount, foundCount).
func (c *Client) FetchBlockTree(pageID string, progress func(fetched, found int)) (*BlockTree, error) {
	tree := &BlockTree{
		Children: make(map[string][]Block),
	}

	var mu sync.Mutex
	var fetched int
	found := 1 // start with 1 for the root

	report := func() {
		if progress != nil {
			mu.Lock()
			progress(fetched, found)
			mu.Unlock()
		}
	}

	var fetchErr error
	var errOnce sync.Once

	var wg sync.WaitGroup

	var fetchRecursive func(blockID string)
	fetchRecursive = func(blockID string) {
		defer wg.Done()

		blocks, err := c.FetchAllBlocks(blockID)

		if err != nil {
			errOnce.Do(func() { fetchErr = err })
			return
		}

		mu.Lock()
		tree.Children[blockID] = blocks
		fetched++

		// Count new children that need fetching
		childCount := 0
		for _, b := range blocks {
			if b.HasChildren {
				childCount++
			}
		}
		found += childCount
		mu.Unlock()

		report()

		// Recurse into children concurrently
		for _, b := range blocks {
			if b.HasChildren {
				wg.Add(1)
				go fetchRecursive(b.ID)
			}
		}
	}

	wg.Add(1)
	go fetchRecursive(pageID)
	wg.Wait()

	if fetchErr != nil {
		return nil, fetchErr
	}

	return tree, nil
}

// QueryAllEntries queries all entries from a data source (handling pagination).
func (c *Client) QueryAllEntries(dataSourceID string) ([]Page, error) {
	var entries []Page
	var cursor *string

	for {
		resp, err := c.QueryDataSource(dataSourceID, cursor)
		if err != nil {
			return nil, err
		}

		for _, result := range resp.Results {
			if result.Object == "page" {
				entries = append(entries, result)
			}
		}

		if !resp.HasMore || resp.NextCursor == nil {
			break
		}
		cursor = resp.NextCursor
	}

	return entries, nil
}

// IsNotFoundError returns true if the error indicates the requested
// Notion object was not found or not accessible as the expected type.
// Notion returns 404 for missing objects and 401 "API token is invalid"
// when querying an ID against the wrong object type (e.g. page ID on /databases/).
func IsNotFoundError(err error) bool {
	var apiErr *ErrorResponse
	if errors.As(err, &apiErr) {
		return apiErr.Status == 404 ||
			apiErr.Code == "object_not_found" ||
			(apiErr.Status == 401 && strings.Contains(apiErr.Message, "API token is invalid"))
	}
	return false
}

var hexIDRe = regexp.MustCompile(`[a-f0-9]{32}`)

// NormalizeNotionID accepts a 32-char hex string, UUID with dashes, or full Notion URL.
// Returns a UUID with dashes.
func NormalizeNotionID(input string) (string, error) {
	raw := strings.TrimSpace(input)

	// Handle full Notion URLs: extract the last 32 hex chars
	if strings.HasPrefix(raw, "http") {
		matches := hexIDRe.FindAllString(strings.ToLower(raw), -1)
		if len(matches) == 0 {
			return "", fmt.Errorf("could not extract Notion ID from URL: %s", raw)
		}
		raw = matches[len(matches)-1]
	}

	// Strip dashes to get pure hex
	hex := strings.ReplaceAll(raw, "-", "")
	hex = strings.ToLower(hex)

	if len(hex) != 32 || !hexIDRe.MatchString(hex) {
		return "", fmt.Errorf("invalid Notion ID: %s", input)
	}

	// Format as UUID: 8-4-4-4-12
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32]), nil
}
