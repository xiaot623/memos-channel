package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/usememos/memogram/internal/channel"
)

type Options struct {
	Token            string
	ProxyAddr        string
	AllowedUsernames string
}

type Adapter struct {
	opts             Options
	allowedUsernames map[string]struct{}
	httpClient       *http.Client
	bot              *bot.Bot
	handle           channel.HandleFunc
}

func New(opts Options) *Adapter {
	return &Adapter{
		opts:             opts,
		allowedUsernames: parseAllowedUsernames(opts.AllowedUsernames),
		httpClient:       http.DefaultClient,
	}
}

func (a *Adapter) Name() string {
	return channel.Telegram
}

func (a *Adapter) Start(ctx context.Context, handle channel.HandleFunc) error {
	a.handle = handle
	opts := []bot.Option{
		bot.WithDefaultHandler(a.onMessage),
		bot.WithCallbackQueryDataHandler("", bot.MatchTypePrefix, a.onCallback),
	}
	if a.opts.ProxyAddr != "" {
		opts = append(opts, bot.WithServerURL(a.opts.ProxyAddr))
	}

	b, err := bot.New(a.opts.Token, opts...)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}
	a.bot = b

	commands := []models.BotCommand{
		{Command: "start", Description: "Start the bot with access token"},
		{Command: "list", Description: "List latest memos"},
		{Command: "tags", Description: "Browse memos by tag"},
		{Command: "search", Description: "Search for the memos"},
	}
	if _, err := a.bot.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: commands}); err != nil {
		slog.Error("failed to set bot commands", slog.Any("err", err))
	}

	a.bot.Start(ctx)
	return nil
}

func (a *Adapter) onMessage(ctx context.Context, _ *bot.Bot, m *models.Update) {
	if m == nil || m.Message == nil || m.Message.From == nil {
		a.sendError(ctx, 0, errors.New("invalid message structure: missing required fields"))
		return
	}
	if m.Message.Chat.ID == 0 {
		a.sendError(ctx, 0, errors.New("invalid chat: missing chat ID"))
		return
	}

	username := m.Message.From.Username
	if !a.isUserAllowed(username) {
		if username == "" {
			a.sendError(ctx, m.Message.Chat.ID, errors.New("your account must have a username to use this bot"))
			return
		}
		a.sendError(ctx, m.Message.Chat.ID, fmt.Errorf("your account %s is not allowed to use this bot", username))
		return
	}

	if a.handle == nil {
		return
	}
	if err := a.handle(ctx, a.messageEvent(m)); err != nil {
		slog.Error("handle inbound event", slog.Any("err", err))
	}
}

func (a *Adapter) onCallback(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}

	origin := channel.Origin{
		AckID: update.CallbackQuery.ID,
	}
	if msg := update.CallbackQuery.Message.Message; msg != nil {
		origin.ChatID = strconv.FormatInt(msg.Chat.ID, 10)
		origin.MessageID = strconv.Itoa(msg.ID)
	}

	parts := strings.SplitN(update.CallbackQuery.Data, " ", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		_ = a.Reply(ctx, origin, channel.OutboundMessage{
			Kind:  channel.OutboundError,
			Error: "Invalid command",
		})
		return
	}

	if a.handle == nil {
		return
	}
	ev := channel.InboundEvent{
		Channel:        channel.Telegram,
		PlatformUserID: strconv.FormatInt(update.CallbackQuery.From.ID, 10),
		Origin:         origin,
		Kind:           channel.KindAction,
		Action: channel.Action{
			Name:     parts[0],
			Resource: parts[1],
		},
	}
	if err := a.handle(ctx, ev); err != nil {
		slog.Error("handle inbound action", slog.Any("err", err))
	}
}

func (a *Adapter) messageEvent(m *models.Update) channel.InboundEvent {
	message := m.Message
	ev := channel.InboundEvent{
		Channel:        channel.Telegram,
		PlatformUserID: strconv.FormatInt(message.From.ID, 10),
		Origin: channel.Origin{
			ChatID:    strconv.FormatInt(message.Chat.ID, 10),
			MessageID: strconv.Itoa(message.ID),
		},
		Kind:    channel.KindMessage,
		GroupID: message.MediaGroupID,
	}

	text := message.Text
	switch {
	case text == "/start" || strings.HasPrefix(text, "/start "):
		ev.Kind = channel.KindCommand
		ev.Command = channel.Command{
			Name: channel.CommandStart,
			Args: strings.TrimSpace(strings.TrimPrefix(text, "/start")),
		}
		return ev
	case text == "/search" || strings.HasPrefix(text, "/search "):
		ev.Kind = channel.KindCommand
		ev.Command = channel.Command{
			Name: channel.CommandSearch,
			Args: strings.TrimSpace(strings.TrimPrefix(text, "/search")),
		}
		return ev
	case text == "/list" || strings.HasPrefix(text, "/list "):
		ev.Kind = channel.KindCommand
		ev.Command = channel.Command{
			Name: channel.CommandList,
			Args: strings.TrimSpace(strings.TrimPrefix(text, "/list")),
		}
		return ev
	case text == "/tags" || strings.HasPrefix(text, "/tags "):
		ev.Kind = channel.KindCommand
		ev.Command = channel.Command{
			Name: channel.CommandTags,
			Args: strings.TrimSpace(strings.TrimPrefix(text, "/tags")),
		}
		return ev
	case text == "/cancel" || strings.HasPrefix(text, "/cancel "):
		ev.Kind = channel.KindCommand
		ev.Command = channel.Command{
			Name: channel.CommandCancel,
		}
		return ev
	}

	content := message.Text
	contentEntities := message.Entities
	if message.Caption != "" {
		content = message.Caption
		contentEntities = message.CaptionEntities
	}
	if len(contentEntities) > 0 {
		content = formatContent(content, contentEntities)
	}
	ev.TextMarkdown = prependForwardOrigin(content, message)

	if message.Document != nil {
		ev.Attachments = append(ev.Attachments, a.fileRef(message.Document.FileID))
	}
	if message.Voice != nil {
		ev.Attachments = append(ev.Attachments, a.fileRef(message.Voice.FileID))
	}
	if message.Video != nil {
		ev.Attachments = append(ev.Attachments, a.fileRef(message.Video.FileID))
	}
	if len(message.Photo) > 0 {
		photo := message.Photo[len(message.Photo)-1]
		ev.Attachments = append(ev.Attachments, a.fileRef(photo.FileID))
	}
	return ev
}

func (a *Adapter) sendError(ctx context.Context, chatID int64, err error) {
	slog.Error("error", slog.Any("err", err))
	if a.bot == nil {
		return
	}
	_, _ = a.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("Error: %s", err.Error()),
	})
}
