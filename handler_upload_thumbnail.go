package main

import (
	"crypto/rand"
	"encoding/base64"
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

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	multipartFile, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer multipartFile.Close()

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unable to find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User unauthorized to modify this video", err)
		return
	}

	mediaType := header.Header.Get("Content-Type")
	mediaType, _, _ = mime.ParseMediaType(mediaType)
	if mediaType != "image/png" && mediaType != "image/jpeg" {
		respondWithError(w, http.StatusBadRequest, "Only image files are allowed: [png, jpeg]", err)
		return
	}

	parts := strings.Split(mediaType, "/")
	ext := parts[1]
	key := make([]byte, 32)
	rand.Read(key)
	randomString := base64.RawURLEncoding.EncodeToString(key)
	thumbnailFilePath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s.%s", randomString, ext))
	file, err := os.Create(thumbnailFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create image file", err)
		return
	}
	io.Copy(file, multipartFile)

	newURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, randomString, ext)
	video.ThumbnailURL = &newURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
