package metadata

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/bogem/id3v2/v2"
)

const maxThumbnailSize = 5 * 1024 * 1024 // 5MB limit

// fetchImageWithValidation downloads and validates thumbnail images
func fetchImageWithValidation(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "radii5-metadata-fetcher")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("image fetch failed with status %d", resp.StatusCode)
	}

	// Check Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || !strings.HasPrefix(mediaType, "image/") {
			return nil, fmt.Errorf("invalid content type: %s", contentType)
		}
	}

	// Limit reader to prevent memory exhaustion
	limitedReader := io.LimitReader(resp.Body, maxThumbnailSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// Check if we hit the size limit
	if len(data) == maxThumbnailSize {
		return nil, fmt.Errorf("image too large (>5MB)")
	}

	return data, nil
}

// WriteMP3Tags writes ID3v2 tags to a downloaded MP3 file
func WriteMP3Tags(path, title, artist, album, thumbnailURL string) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("failed to open MP3 file: %w", err)
	}
	defer tag.Close()

	if title != "" {
		tag.SetTitle(title)
	}
	if artist != "" {
		tag.SetArtist(artist)
	}
	if album != "" {
		tag.SetAlbum(album)
	}

	// Embed thumbnail as album art
	if thumbnailURL != "" {
		if img, err := fetchImageWithValidation(thumbnailURL); err == nil {
			pic := id3v2.PictureFrame{
				Encoding:    id3v2.EncodingUTF8,
				MimeType:    "image/jpeg",
				PictureType: id3v2.PTFrontCover,
				Description: "Cover",
				Picture:     img,
			}
			tag.AddAttachedPicture(pic)
		} else {
			// Log error but don't fail the entire operation
			fmt.Printf("Warning: failed to fetch thumbnail: %v\n", err)
		}
	}

	return tag.Save()
}

// fetchImage is kept for backward compatibility but deprecated
func fetchImage(url string) ([]byte, error) {
	return fetchImageWithValidation(url)
}
