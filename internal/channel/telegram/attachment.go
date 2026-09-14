package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/go-telegram/bot"
	"github.com/usememos/memogram/internal/channel"
)

type fileRef struct {
	adapter *Adapter
	fileID  string
}

func (a *Adapter) fileRef(fileID string) channel.AttachmentRef {
	return &fileRef{adapter: a, fileID: fileID}
}

func (f *fileRef) Filename() string {
	return f.fileID
}

func (f *fileRef) Download(ctx context.Context) (*channel.Attachment, error) {
	if f.adapter == nil || f.adapter.bot == nil {
		return nil, fmt.Errorf("telegram bot is not started")
	}

	file, err := f.adapter.bot.GetFile(ctx, &bot.GetFileParams{FileID: f.fileID})
	if err != nil {
		return nil, err
	}

	fileLink := f.adapter.bot.FileDownloadLink(file)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileLink, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	response, err := f.adapter.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("download failed with status %s", response.Status)
	}

	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	contentType := response.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(bytes)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &channel.Attachment{
		Filename:    filepath.Base(file.FilePath),
		ContentType: contentType,
		Bytes:       bytes,
	}, nil
}
