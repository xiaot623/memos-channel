package feishu

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/usememos/memogram/internal/channel"
)

func (a *Adapter) Reply(ctx context.Context, origin channel.Origin, msg channel.OutboundMessage) error {
	if origin.AckID != "" {
		return a.replyCallback(origin, msg)
	}
	if a.client == nil {
		return fmt.Errorf("feishu client is not started")
	}
	if origin.Edit && msg.Kind == channel.OutboundBrowse {
		return a.patchCard(ctx, origin.MessageID, browseCard(msg.Browse))
	}
	return a.replyChat(ctx, origin, msg)
}

func (a *Adapter) replyCallback(origin channel.Origin, msg channel.OutboundMessage) error {
	resp, err := callbackResponse(msg)
	if err != nil {
		return err
	}
	if slot, ok := a.pending.Load(origin.AckID); ok {
		slot.(*pendingCard).resp = resp
		return nil
	}
	return fmt.Errorf("missing card callback slot")
}

func callbackResponse(msg channel.OutboundMessage) (*callback.CardActionTriggerResponse, error) {
	switch msg.Kind {
	case channel.OutboundPromptBind:
		return toastOnly("error", promptBindText()), nil
	case channel.OutboundError:
		return toastOnly("error", msg.Error), nil
	case channel.OutboundSaved:
		if msg.Memo == nil {
			return nil, fmt.Errorf("missing memo for saved reply")
		}
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "info", Content: "Memo updated"},
			Card:  rawCard(savedCard("Memo updated", msg.Memo, msg.Actions)),
		}, nil
	case channel.OutboundBrowse:
		if msg.Browse == nil {
			return nil, fmt.Errorf("missing browse payload")
		}
		return &callback.CardActionTriggerResponse{
			Card: rawCard(browseCard(msg.Browse)),
		}, nil
	default:
		return toastOnly("info", "Memo updated"), nil
	}
}

func toastOnly(kind, content string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{Type: kind, Content: content},
	}
}

func rawCard(data map[string]any) *callback.Card {
	return &callback.Card{Type: "raw", Data: data}
}

func (a *Adapter) replyChat(ctx context.Context, origin channel.Origin, msg channel.OutboundMessage) error {
	switch msg.Kind {
	case channel.OutboundPromptBind:
		return a.sendText(ctx, origin.ChatID, promptBindText())
	case channel.OutboundPromptUsage:
		return a.sendText(ctx, origin.ChatID, usageText(msg.Prompt))
	case channel.OutboundBound:
		return a.sendText(ctx, origin.ChatID, fmt.Sprintf("Hello %s!", msg.User))
	case channel.OutboundError:
		return a.sendText(ctx, origin.ChatID, msg.Error)
	case channel.OutboundBrowse:
		if msg.Browse == nil {
			return fmt.Errorf("missing browse payload")
		}
		return a.sendCard(ctx, origin.ChatID, browseCard(msg.Browse))
	case channel.OutboundSaved:
		if msg.Memo == nil {
			return fmt.Errorf("missing memo for saved reply")
		}
		card := savedCard("Content saved", msg.Memo, msg.Actions)
		if origin.MessageID != "" {
			if err := a.replyCard(ctx, origin.MessageID, card); err == nil {
				return nil
			}
		}
		return a.sendCard(ctx, origin.ChatID, card)
	default:
		return nil
	}
}

func promptBindText() string {
	return "Please start the bot with /start <access_token>"
}

func usageText(prompt string) string {
	switch prompt {
	case channel.CommandSearch:
		return "Usage: /search <words>"
	default:
		return "Usage: /start <access_token>"
	}
}

func marshalContent(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *Adapter) sendText(ctx context.Context, chatID, text string) error {
	return a.createMessage(ctx, chatID, larkim.MsgTypeText, map[string]string{"text": text})
}

func (a *Adapter) sendCard(ctx context.Context, chatID string, card map[string]any) error {
	return a.createMessage(ctx, chatID, larkim.MsgTypeInteractive, card)
}

func (a *Adapter) createMessage(ctx context.Context, chatID, msgType string, payload any) error {
	content, err := marshalContent(payload)
	if err != nil {
		return err
	}
	resp, err := a.client.Im.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("send %s: %s", msgType, resp.Msg)
	}
	return nil
}

func (a *Adapter) replyCard(ctx context.Context, messageID string, card map[string]any) error {
	content, err := marshalContent(card)
	if err != nil {
		return err
	}
	resp, err := a.client.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Content(content).
			Build()).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("reply card: %s", resp.Msg)
	}
	return nil
}

func (a *Adapter) patchCard(ctx context.Context, messageID string, card map[string]any) error {
	content, err := marshalContent(card)
	if err != nil {
		return err
	}
	resp, err := a.client.Im.Message.Patch(ctx, larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(content).
			Build()).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("patch card: %s", resp.Msg)
	}
	return nil
}
