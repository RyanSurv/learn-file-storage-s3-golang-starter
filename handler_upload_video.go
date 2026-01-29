package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// upload limit of 1 GB
	http.MaxBytesReader(w, r.Body, 1<<30)

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

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video from DB", err)
		return
	}

	if userID != dbVideo.UserID {
		respondWithError(w, http.StatusUnauthorized, "You do not own this video", err)
		return
	}

	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error during FormFile", err)
		return
	}
	defer file.Close()

	mediaType := fileHeader.Header.Get("Content-Type")
	mediaTypeParts := strings.Split(mediaType, "/")
	if len(mediaTypeParts) != 2 {
		respondWithError(w, http.StatusBadRequest, "Error getting file type", err)
		return
	}

	// Only allow mp4
	fileType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing media type", err)
		return
	}
	if fileType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Unsupported file type", err)
		return
	}

	// Create temp file on disk
	diskFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error creating temp file", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer diskFile.Close()

	_, err = io.Copy(diskFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error copying contents into temp file", err)
		return
	}

	// this will allow us to read the file again from the beginning
	diskFile.Seek(0, io.SeekStart)

	// get aspect ratio to appropriately prefix object
	aspectRatio, err := getVideoAspectRatio(diskFile.Name())
	if err != nil {
		fmt.Println(err)
		respondWithError(w, http.StatusBadRequest, "Error getting video aspect ratio", err)
		return
	}

	var prefix string
	if aspectRatio == "9:16" {
		prefix = "portrait"
	} else if aspectRatio == "16:9" {
		prefix = "landscape"
	} else {
		prefix = "other"
	}

	key := make([]byte, 32)
	rand.Read(key)
	encoded := base64.RawURLEncoding.EncodeToString(key)
	finalKey := fmt.Sprintf("%s/%s.mp4", prefix, encoded)
	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &finalKey,
		Body:        diskFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error adding video to bucket", err)
		return
	}

	s3VideoUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, finalKey)
	dbVideo.VideoURL = &s3VideoUrl
	if err := cfg.db.UpdateVideo(dbVideo); err != nil {
		respondWithError(w, http.StatusBadRequest, "Error updating video in DB", err)
		return
	}

	respondWithJSON(w, http.StatusOK, finalKey)
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var buf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		fmt.Println("ffprobe stderr:", errBuf.String())
		return "", err
	}

	type ffprobeOutput struct {
		Streams []struct {
			// Width int `json:"width"`
			// Height int `json:"height"`
			Ratio string `json:"display_aspect_ratio"`
		}
	}
	var output ffprobeOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		return "", err
	}

	stream := output.Streams[0]
	if stream.Ratio == "" {
		return "", errors.New("Error populating json fields")
	}

	if stream.Ratio == "9:16" {
		return "9:16", nil
	} else if stream.Ratio == "16:9" {
		return "16:9", nil
	}

	return "other", nil
}
