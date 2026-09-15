package feishu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/usememos/memogram/internal/channel"
)

type fileRef struct {
	adapter      *Adapter
	messageID    string
	fileKey      string
	resourceType string
	filename     string
}

func (a *Adapter) fileRef(messageID, fileKey, resourceType, filename string) channel.AttachmentRef {
	return &fileRef{
		adapter:      a,
		messageID:    messageID,
		fileKey:      fileKey,
		resourceType: resourceType,
		filename:     filename,
	}
}

func (f *fileRef) Filename() string {
	if f.filename != "" {
		return f.filename
	}
	return f.fileKey
}

func (f *fileRef) Download(ctx context.Context) (*channel.Attachment, error) {
	if f.adapter == nil || f.adapter.client == nil {
		return nil, fmt.Errorf("feishu client is not started")
	}

	resp, err := f.adapter.client.Im.MessageResource.Get(ctx, larkim.NewGetMessageResourceReqBuilder().
		MessageId(f.messageID).
		FileKey(f.fileKey).
		Type(f.resourceType).
		Build())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("download resource: %s", resp.Msg)
	}

	reader := resp.File
	if rc, ok := reader.(io.ReadCloser); ok {
		defer rc.Close()
	}
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	filename := f.filename
	if filename == "" && resp.FileName != "" {
		filename = filepath.Base(resp.FileName)
	}
	if filename == "" {
		filename = f.fileKey
	}

	return &channel.Attachment{
		Filename:    filename,
		ContentType: http.DetectContentType(bytes),
		Bytes:       bytes,
	}, nil
}
