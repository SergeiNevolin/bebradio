package repository

type MediaClient interface {
	Search(query string, limit int) ([]map[string]any, error)
	Resolve(url string) (map[string]any, error)
	Download(sourceURL, mediaID string) (map[string]any, error)
	Ensure(items []map[string]any) ([]string, error)
	Related(sourceURL string, limit int) ([]string, error)
	Captions(sourceURL, lang string) (map[string]any, error)
	Content(mediaID, rangeHeader string) (int64, string, []byte, error)
	UpdateReferences(mediaIDs []string) error
}
