// Fences docusaurus/ off from the notion-sync Go module. A directory with its own go.mod is a
// separate module, so `go build ./...` / `go test ./...` at the repo root never walk into
// docusaurus/ — including node_modules/, where an npm package can ship stray .go files.
module github.com/Drexel-UHC/notion-sync/docusaurus

go 1.24
