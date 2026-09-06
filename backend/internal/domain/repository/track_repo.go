package repository

import "io"

type MediaClient interface {
	Search(query string, limit int) ([]map[string]any, error)
	Resolve(url string) (map[string]any, error)
	Download(sourceURL, mediaID string) (map[string]any, error)
	Ensure(items []map[string]any) ([]string, error)
	Related(sourceURL string, limit int) ([]string, error)
	Captions(sourceURL, lang string) (map[string]any, error)
	Content(mediaID, rangeHeader string) (int64, string, []byte, error)
	UpdateReferences(mediaIDs []string) error

	// Mashups: persistent user uploads, transcoded with ffmpeg on media-service.
	UploadMashup(mediaID, filename string, body io.Reader) error
	MashupStatus(mediaID string) (map[string]any, error)
	DeleteMashup(mediaID string) error
}
