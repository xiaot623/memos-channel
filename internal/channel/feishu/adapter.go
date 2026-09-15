package feishu

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/usememos/memogram/internal/channel"
)

type Options struct {
	AppID          string
	AppSecret      string
	BaseURL        string
	AllowedOpenIDs string
}

type pendingCard struct {
	resp *callback.CardActionTriggerResponse
}

type Adapter struct {
	opts           Options
	allowedOpenIDs map[string]struct{}
	client         *lark.Client
	handle         channel.HandleFunc
	pending        sync.Map
}

func New(opts Options) *Adapter {
	return &Adapter{
		opts:           opts,
		allowedOpenIDs: parseAllowedOpenIDs(opts.AllowedOpenIDs),
	}
}

func (a *Adapter) Name() string {
	return channel.Feishu
}

func (a *Adapter) Start(ctx context.Context, handle channel.HandleFunc) error {
	a.handle = handle

	base := strings.TrimSpace(a.opts.BaseURL)
	var clientOpts []lark.ClientOptionFunc
	if base != "" {
		clientOpts = append(clientOpts, lark.WithOpenBaseUrl(base))
	}
	a.client = lark.NewClient(a.opts.AppID, a.opts.AppSecret, clientOpts...)

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(a.onMessage).
		OnP2CardActionTrigger(a.onCardAction)

	wsOpts := []larkws.ClientOption{larkws.WithEventHandler(eventHandler)}
	if base != "" {
		wsOpts = append(wsOpts, larkws.WithDomain(base))
	}
	return larkws.NewClient(a.opts.AppID, a.opts.AppSecret, wsOpts...).Start(ctx)
}

func (a *Adapter) onMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event != nil && event.Event != nil && event.Event.Sender != nil {
		senderType := deref(event.Event.Sender.SenderType)
		if senderType == "app" || senderType == "bot" {
			return nil
		}
	}
	if event != nil && event.Event != nil && event.Event.Message != nil && deref(event.Event.Message.ChatType) != "p2p" {
		return nil
	}

	var openID, chatID string
	if event != nil && event.Event != nil {
		if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
			openID = deref(event.Event.Sender.SenderId.OpenId)
		}
		if event.Event.Message != nil {
			chatID = deref(event.Event.Message.ChatId)
		}
	}
	if !a.isUserAllowed(openID) {
		_ = a.sendText(ctx, chatID, "你的账号无权使用此机器人")
		return nil
	}

	ev, err := a.messageEvent(event)
	if err != nil {
		slog.Error("feishu inbound", slog.Any("err", err))
		if errors.Is(err, errUnsupportedType) {
			_ = a.sendText(ctx, chatID, "暂不支持该消息类型")
		}
		return nil
	}
	if a.handle == nil {
		return nil
	}
	if err := a.handle(ctx, ev); err != nil {
		slog.Error("handle inbound event", slog.Any("err", err))
	}
	return nil
}

func (a *Adapter) onCardAction(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if event == nil || event.Event == nil {
		return toastOnly("error", "Invalid command"), nil
	}

	token := event.Event.Token
	slot := &pendingCard{}
	a.pending.Store(token, slot)
	defer a.pending.Delete(token)

	openID := ""
	if event.Event.Operator != nil {
		openID = event.Event.Operator.OpenID
	}
	origin := channel.Origin{AckID: token}
	if event.Event.Context != nil {
		origin.ChatID = event.Event.Context.OpenChatID
		origin.MessageID = event.Event.Context.OpenMessageID
	}

	if !a.isUserAllowed(openID) {
		return toastOnly("error", "你的账号无权使用此机器人"), nil
	}

	actionValue := map[string]interface{}{}
	if event.Event.Action != nil {
		actionValue = event.Event.Action.Value
	}
	action, ok := actionFromValue(actionValue)
	if !ok {
		return toastOnly("error", "Invalid command"), nil
	}

	if a.handle == nil {
		return toastOnly("error", "Invalid command"), nil
	}
	ev := channel.InboundEvent{
		Channel:        channel.Feishu,
		PlatformUserID: openID,
		Origin:         origin,
		Kind:           channel.KindAction,
		Action:         action,
	}
	if err := a.handle(ctx, ev); err != nil {
		slog.Error("handle inbound action", slog.Any("err", err))
	}
	if slot.resp != nil {
		return slot.resp, nil
	}
	return &callback.CardActionTriggerResponse{}, nil
}
