package handler

import (
	"net/http"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/dto/response"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/media"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/usecase"
	"github.com/gofiber/fiber/v2"
)

type VideoHandler struct {
	videoUsecase usecase.VideoUsecase
}

func NewVideoHandler(videoUsecase usecase.VideoUsecase) *VideoHandler {
	return &VideoHandler{videoUsecase: videoUsecase}
}

// UploadVideo assigns a server-generated video id, then streams the request
// body straight to disk (tracking progress under that id) before persisting
// it to object storage and removing the local copy.
func (h *VideoHandler) UploadVideo(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	total := c.Request().Header.ContentLength()
	if total <= 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Content-Length header is required"})
	}

	uploadID, err := h.videoUsecase.InitiateUpload(userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.videoUsecase.StoreUpload(c.Context(), uploadID, int64(total), c.Context().RequestBodyStream()); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusCreated).JSON(response.UploadProgressResponse{
		ID:       uploadID,
		Status:   "completed",
		Progress: 100,
	})
}

// UploadProgress reports the percent complete and estimated minutes left
// for an in-flight upload started via UploadVideo.
func (h *VideoHandler) UploadProgress(c *fiber.Ctx) error {
	snap, ok := h.videoUsecase.UploadProgress(c.Params("id"))
	if !ok {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "upload not found"})
	}

	return c.Status(http.StatusOK).JSON(response.UploadProgressResponse{
		ID:                   snap.ID,
		Status:               snap.Status,
		Progress:             snap.Progress,
		EstimatedMinutesLeft: snap.EstimatedMinutesLeft,
		Error:                snap.Err,
	})
}

// VideoRenditions reports the HLS rendition rows recorded for a video so
// far. Renditions are generated in the background after upload, so an empty
// list means processing hasn't started (or finished) yet.
func (h *VideoHandler) VideoRenditions(c *fiber.Ctx) error {
	videoID := c.Params("id")

	renditions, err := h.videoUsecase.ListRenditions(videoID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	items := make([]response.VideoRenditionResponse, 0, len(renditions))
	for _, r := range renditions {
		progress := 0
		switch r.Status {
		case "completed":
			progress = 100
		case "processing":
			if job, ok := media.GetJob(r.ID); ok {
				progress = job.Snapshot().Progress
			}
		}

		items = append(items, response.VideoRenditionResponse{
			Height:      r.Height,
			Width:       r.Width,
			PlaylistKey: r.PlaylistKey,
			Status:      r.Status,
			Progress:    progress,
		})
	}

	return c.Status(http.StatusOK).JSON(response.VideoRenditionsResponse{VideoID: videoID, Renditions: items})
}
