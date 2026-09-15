package feishu

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/usememos/memogram/internal/channel"
)

var errUnsupportedType = errors.New("unsupported message type")

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func parseCommand(text string) (channel.Command, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return channel.Command{}, false
	}
	name, args, _ := strings.Cut(text, " ")
	name = strings.ToLower(name)
	if i := strings.Index(name, "@"); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimPrefix(name, "/")
	switch name {
	case channel.CommandStart, channel.CommandSearch, channel.CommandList, channel.CommandTags, channel.CommandCancel, channel.CommandHelp:
		return channel.Command{Name: name, Args: strings.TrimSpace(args)}, true
	default:
		return channel.Command{}, false
	}
}

func actionFromValue(value map[string]interface{}) (channel.Action, bool) {
	if len(value) == 0 {
		return channel.Action{}, false
	}
	name := stringValue(value["name"])
	resource := stringValue(value["resource"])
	if name == "" || resource == "" {
		return channel.Action{}, false
	}
	return channel.Action{Name: name, Resource: resource}, true
}

func stringValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func (a *Adapter) messageEvent(event *larkim.P2MessageReceiveV1) (channel.InboundEvent, error) {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Sender == nil {
		return channel.InboundEvent{}, fmt.Errorf("invalid message structure: missing required fields")
	}

	msg := event.Event.Message
	openID := ""
	if event.Event.Sender.SenderId != nil {
		openID = deref(event.Event.Sender.SenderId.OpenId)
	}

	ev := channel.InboundEvent{
		Channel:        channel.Feishu,
		PlatformUserID: openID,
		Origin: channel.Origin{
			ChatID:    deref(msg.ChatId),
			MessageID: deref(msg.MessageId),
		},
		Kind: channel.KindMessage,
	}

	msgType := deref(msg.MessageType)
	content := deref(msg.Content)

	switch msgType {
	case larkim.MsgTypeText:
		text := textFromContentJSON(content)
		if cmd, ok := parseCommand(text); ok {
			ev.Kind = channel.KindCommand
			ev.Command = cmd
			return ev, nil
		}
		ev.TextMarkdown = text
	case larkim.MsgTypePost:
		md, imageKeys := postToMarkdown(content)
		if cmd, ok := parseCommand(md); ok {
			ev.Kind = channel.KindCommand
			ev.Command = cmd
			return ev, nil
		}
		ev.TextMarkdown = md
		for _, key := range imageKeys {
			ev.Attachments = append(ev.Attachments, a.fileRef(ev.Origin.MessageID, key, "image", key+".png"))
		}
	case larkim.MsgTypeImage:
		if key := contentField(content, "image_key"); key != "" {
			ev.Attachments = append(ev.Attachments, a.fileRef(ev.Origin.MessageID, key, "image", key+".png"))
		}
	case larkim.MsgTypeFile:
		if key := contentField(content, "file_key"); key != "" {
			ev.Attachments = append(ev.Attachments, a.fileRef(ev.Origin.MessageID, key, "file", contentField(content, "file_name")))
		}
	case larkim.MsgTypeAudio:
		if key := contentField(content, "file_key"); key != "" {
			ev.Attachments = append(ev.Attachments, a.fileRef(ev.Origin.MessageID, key, "file", "audio.ogg"))
		}
	case larkim.MsgTypeMedia:
		if key := contentField(content, "file_key"); key != "" {
			ev.Attachments = append(ev.Attachments, a.fileRef(ev.Origin.MessageID, key, "file", "video.mp4"))
		}
	default:
		return ev, fmt.Errorf("%w: %s", errUnsupportedType, msgType)
	}

	return ev, nil
}

func contentField(raw, key string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	s, _ := payload[key].(string)
	return s
}
