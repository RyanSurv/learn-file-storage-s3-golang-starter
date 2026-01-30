package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func processVideoForFastStart(filePath string) (string, error) {
	parts := strings.Split(filePath, ".")

	if len(parts) != 2 {
		return "", errors.New("Weird file path, correct yourself")
	}

	// foo.mp4 => ["foo", "mp4"] => "foo.processing.mp4"
	outputFilePath := parts[0] + ".processing." + parts[1]

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		fmt.Println("ffprobe stderr:", errBuf.String())
		return "", err
	}

	return outputFilePath, nil
}
