package metadata

import (
	"io"
	"net/http"
	"time"

	"github.com/bogem/id3v2/v2"
)

// WriteMP3Tags writes ID3v2 tags to a downloaded MP3 file
func WriteMP3Tags(path, title, artist, album, thumbnailURL string) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return err
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
		if img, err := fetchImage(thumbnailURL); err == nil {
			pic := id3v2.PictureFrame{
				Encoding:    id3v2.EncodingUTF8,
				MimeType:    "image/jpeg",
				PictureType: id3v2.PTFrontCover,
				Description: "Cover",
				Picture:     img,
			}
			tag.AddAttachedPicture(pic)
		}
	}

	return tag.Save()
}

func fetchImage(url string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
