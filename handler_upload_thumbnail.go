package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const maxMemory = 10 << 20

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing multipart form", err)
		return
	}

	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error during FormFile", err)
		return
	}

	mediaType := fileHeader.Header.Get("Content-Type")
	mediaTypeParts := strings.Split(mediaType, "/")
	if len(mediaTypeParts) != 2 {
		respondWithError(w, http.StatusBadRequest, "Error getting file type", err)
		return
	}

	// Only allow jpeg and png
	fileType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing media type", err)
		return
	}
	if fileType != "image/jpeg" && fileType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Unsupported file type", err)
		return
	}

	imagePath := filepath.Join(fmt.Sprintf("%s/%s.%s", cfg.assetsRoot, videoID, mediaTypeParts[1]))

	createdFile, err := os.Create(imagePath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error creating image file on disk", err)
		return
	}

	_, err = io.Copy(createdFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error creating image file on disk", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error reading image data", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	fullImagePath := fmt.Sprintf("http://localhost:8091/%s", imagePath)
	video.ThumbnailURL = &fullImagePath

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusBadRequest, "Error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
