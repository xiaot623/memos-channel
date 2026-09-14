package memos

import (
	"net/http"

	"github.com/usememos/memos/proto/gen/api/v1/apiv1connect"
)

type Client struct {
	baseURL string

	InstanceService   apiv1connect.InstanceServiceClient
	AuthService       apiv1connect.AuthServiceClient
	UserService       apiv1connect.UserServiceClient
	MemoService       apiv1connect.MemoServiceClient
	AttachmentService apiv1connect.AttachmentServiceClient
}

func NewClient(baseURL string) *Client {
	httpClient := http.DefaultClient

	return &Client{
		baseURL:           baseURL,
		InstanceService:   apiv1connect.NewInstanceServiceClient(httpClient, baseURL),
		AuthService:       apiv1connect.NewAuthServiceClient(httpClient, baseURL),
		UserService:       apiv1connect.NewUserServiceClient(httpClient, baseURL),
		MemoService:       apiv1connect.NewMemoServiceClient(httpClient, baseURL),
		AttachmentService: apiv1connect.NewAttachmentServiceClient(httpClient, baseURL),
	}
}

func (c *Client) NewAuthenticatedClient(accessToken string) *Client {
	httpClient := &http.Client{
		Transport: &authTransport{
			token:     accessToken,
			transport: http.DefaultTransport,
		},
	}

	return &Client{
		baseURL:           c.baseURL,
		InstanceService:   apiv1connect.NewInstanceServiceClient(httpClient, c.baseURL),
		AuthService:       apiv1connect.NewAuthServiceClient(httpClient, c.baseURL),
		UserService:       apiv1connect.NewUserServiceClient(httpClient, c.baseURL),
		MemoService:       apiv1connect.NewMemoServiceClient(httpClient, c.baseURL),
		AttachmentService: apiv1connect.NewAttachmentServiceClient(httpClient, c.baseURL),
	}
}

type authTransport struct {
	token     string
	transport http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.transport.RoundTrip(req)
}
