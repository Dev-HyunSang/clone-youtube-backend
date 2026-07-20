package redis

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const TypeVideoRenditions = "video:renditions"

type VideoRenditionsPayload struct {
	VideoID string
}

func NewVideoRenditionsTask(videoID string) (*asynq.Task, error) {
	payload, err := json.Marshal(VideoRenditionsPayload{VideoID: videoID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeVideoRenditions, payload), nil
}
