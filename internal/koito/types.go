package koito

import (
	"bytes"
	"encoding/json"
	"time"
)

type Image struct {
	XS, Small, Medium, Large, XL string
}

type Size int

const (
	SizeXS Size = iota
	SizeSmall
	SizeMedium
)

// UnmarshalJSON treats anything but an object as no image, so an odd image field can't fail the response.
func (image *Image) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		*image = Image{}
		return nil
	}
	var paths struct {
		XS     string `json:"xs"`
		Small  string `json:"small"`
		Medium string `json:"medium"`
		Large  string `json:"large"`
		XL     string `json:"xl"`
	}
	if err := json.Unmarshal(data, &paths); err != nil {
		return err
	}
	*image = Image(paths)
	return nil
}

// Path returns the path for size, or the nearest size Koito has.
func (image Image) Path(size Size) string {
	pathsBySize := []string{image.XS, image.Small, image.Medium, image.Large, image.XL}
	for larger := int(size); larger < len(pathsBySize); larger++ {
		if pathsBySize[larger] != "" {
			return pathsBySize[larger]
		}
	}
	for smaller := int(size) - 1; smaller >= 0; smaller-- {
		if pathsBySize[smaller] != "" {
			return pathsBySize[smaller]
		}
	}
	return ""
}

type Artist struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Track struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Artists     []Artist `json:"artists"`
	Image       Image    `json:"image"`
	ListenCount int64    `json:"listen_count"`
}

// Listen.Time is zero for the track playing now.
type Listen struct {
	Time  time.Time `json:"time"`
	Track Track     `json:"track"`
}

// Entity is a top-list artist (Name) or album/track (Title, Artists).
type Entity struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Artists     []Artist `json:"artists"`
	Image       Image    `json:"image"`
	ListenCount int64    `json:"listen_count"`
}

type Ranked struct {
	Item Entity `json:"item"`
	Rank int    `json:"rank"`
}

type Stats struct {
	ListenCount     int64 `json:"listen_count"`
	TrackCount      int64 `json:"track_count"`
	AlbumCount      int64 `json:"album_count"`
	ArtistCount     int64 `json:"artist_count"`
	MinutesListened int64 `json:"minutes_listened"`
}

type Bucket struct {
	Start   time.Time `json:"start_time"`
	Listens int64     `json:"listens"`
}

type Activity struct {
	Buckets    []Bucket `json:"activity"`
	StreakDays int      `json:"streak"`
}

type pageResponse[T any] struct {
	Items []T `json:"items"`
}

type nowPlayingResponse struct {
	CurrentlyPlaying bool   `json:"currently_playing"`
	Track            *Track `json:"track"`
}
