package response

type UploadProgressResponse struct {
	ID                   string  `json:"id"`
	Status               string  `json:"status"`
	Progress             int     `json:"progress"`
	EstimatedMinutesLeft float64 `json:"estimated_minutes_left,omitempty"`
	Error                string  `json:"error,omitempty"`
}

type VideoRenditionResponse struct {
	Height      int    `json:"height"`
	Width       int    `json:"width"`
	PlaylistKey string `json:"playlist_key,omitempty"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
}

type VideoRenditionsResponse struct {
	VideoID    string                   `json:"video_id"`
	Renditions []VideoRenditionResponse `json:"renditions"`
}
