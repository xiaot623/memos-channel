package core

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/usememos/memogram/internal/memos"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

type Client interface {
	GetInstanceProfile(ctx context.Context) (*v1pb.InstanceProfile, error)
	Authenticated(token string) AuthedClient
}

type ListMemosPage struct {
	Memos         []*v1pb.Memo
	NextPageToken string
}

type AuthedClient interface {
	GetCurrentUser(ctx context.Context) (*v1pb.User, error)
	CreateMemo(ctx context.Context, content string) (*v1pb.Memo, error)
	GetMemo(ctx context.Context, name string) (*v1pb.Memo, error)
	UpdateMemo(ctx context.Context, memo *v1pb.Memo, paths []string) error
	DeleteMemo(ctx context.Context, name string) error
	ListMemos(ctx context.Context, pageSize int32, pageToken, orderBy, filter string) (ListMemosPage, error)
	GetUserStats(ctx context.Context, userName string) (*v1pb.UserStats, error)
	CreateAttachment(ctx context.Context, filename, contentType string, data []byte, memoName string) error
}

type memosBackend struct {
	c *memos.Client
}

func wrapMemos(c *memos.Client) Client {
	return &memosBackend{c: c}
}

func (m *memosBackend) GetInstanceProfile(ctx context.Context) (*v1pb.InstanceProfile, error) {
	resp, err := m.c.InstanceService.GetInstanceProfile(ctx, connect.NewRequest(&v1pb.GetInstanceProfileRequest{}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (m *memosBackend) Authenticated(token string) AuthedClient {
	return &authedBackend{c: m.c.NewAuthenticatedClient(token)}
}

type authedBackend struct {
	c *memos.Client
}

func (a *authedBackend) GetCurrentUser(ctx context.Context) (*v1pb.User, error) {
	resp, err := a.c.AuthService.GetCurrentUser(ctx, connect.NewRequest(&v1pb.GetCurrentUserRequest{}))
	if err != nil {
		return nil, err
	}
	if resp.Msg == nil || resp.Msg.User == nil {
		return nil, fmt.Errorf("empty current user")
	}
	return resp.Msg.User, nil
}

func (a *authedBackend) CreateMemo(ctx context.Context, content string) (*v1pb.Memo, error) {
	resp, err := a.c.MemoService.CreateMemo(ctx, connect.NewRequest(&v1pb.CreateMemoRequest{
		Memo: &v1pb.Memo{Content: content},
	}))
	if err != nil {
		slog.Error("failed to create memo", slog.Any("err", err))
		return nil, fmt.Errorf("create memo: %w", err)
	}
	return resp.Msg, nil
}

func (a *authedBackend) GetMemo(ctx context.Context, name string) (*v1pb.Memo, error) {
	resp, err := a.c.MemoService.GetMemo(ctx, connect.NewRequest(&v1pb.GetMemoRequest{Name: name}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (a *authedBackend) UpdateMemo(ctx context.Context, memo *v1pb.Memo, paths []string) error {
	_, err := a.c.MemoService.UpdateMemo(ctx, connect.NewRequest(&v1pb.UpdateMemoRequest{
		Memo: memo,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: paths,
		},
	}))
	return err
}

func (a *authedBackend) ListMemos(ctx context.Context, pageSize int32, pageToken, orderBy, filter string) (ListMemosPage, error) {
	resp, err := a.c.MemoService.ListMemos(ctx, connect.NewRequest(&v1pb.ListMemosRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		OrderBy:   orderBy,
		Filter:    filter,
	}))
	if err != nil {
		return ListMemosPage{}, err
	}
	return ListMemosPage{
		Memos:         resp.Msg.GetMemos(),
		NextPageToken: resp.Msg.GetNextPageToken(),
	}, nil
}

func (a *authedBackend) GetUserStats(ctx context.Context, userName string) (*v1pb.UserStats, error) {
	resp, err := a.c.UserService.GetUserStats(ctx, connect.NewRequest(&v1pb.GetUserStatsRequest{
		Name: userName,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func (a *authedBackend) DeleteMemo(ctx context.Context, name string) error {
	_, err := a.c.MemoService.DeleteMemo(ctx, connect.NewRequest(&v1pb.DeleteMemoRequest{
		Name: name,
	}))
	return err
}

func (a *authedBackend) CreateAttachment(ctx context.Context, filename, contentType string, data []byte, memoName string) error {
	_, err := a.c.AttachmentService.CreateAttachment(ctx, connect.NewRequest(&v1pb.CreateAttachmentRequest{
		Attachment: &v1pb.Attachment{
			Filename: filename,
			Type:     contentType,
			Size:     int64(len(data)),
			Content:  data,
			Memo:     &memoName,
		},
	}))
	return err
}
