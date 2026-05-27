package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading video ", videoID, "by user", userID)

	const maxUploadSize = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxUploadSize))

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unable to find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User unauthorized to modify this video", err)
		return
	}

	multipartFile, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer multipartFile.Close()

	mediaType := header.Header.Get("Content-Type")
	mediaType, _, _ = mime.ParseMediaType(mediaType)
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Supported video file format: [mp4]", err)
		return
	}

	tempFile, err := os.CreateTemp("/tmp", "tubely-upload-temp-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error while creating temporary video file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	io.Copy(tempFile, multipartFile)
	tempFile.Seek(0, io.SeekStart)

	// upload to s3 bucket
	key := make([]byte, 32)
	rand.Read(key)
	randomString := base64.RawURLEncoding.EncodeToString(key)
	objectKey := fmt.Sprintf("%s.mp4", randomString)
	cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &objectKey,
		Body:        tempFile,
		ContentType: &mediaType,
	})

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, objectKey)
	video.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
