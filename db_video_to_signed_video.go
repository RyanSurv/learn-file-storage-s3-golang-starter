package main

import (
	"errors"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}

	parts := strings.Split(*video.VideoURL, ",")
	if len(parts) != 2 {
		return database.Video{}, errors.New("Invalid VideoURL")
	}

	bucket := parts[0]
	key := parts[1]
	expires, err := time.ParseDuration("120s")
	if err != nil {
		return database.Video{}, err
	}

	presignedUrl, err := generatePresignedURL(cfg.s3Client, bucket, key, expires)
	if err != nil {
		return database.Video{}, err
	}

	video.VideoURL = &presignedUrl

	return video, nil
}
