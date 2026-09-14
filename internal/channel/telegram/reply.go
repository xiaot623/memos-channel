package telegram

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/usememos/memogram/internal/channel"
)

var disableLinkPreview = &models.LinkPreviewOptions{IsDisabled: boolPtr(true)}

func boolPtr(v bool) *bool { return &v }

func (a *Adapter) Reply(ctx context.Context, origin channel.Origin, msg channel.OutboundMessage) error {
	if a.bot == nil {
		return fmt.Errorf("telegram bot is not started")
	}

	if origin.AckID != "" {
		return a.replyCallback(ctx, origin, msg)
	}
	if origin.Edit && msg.Kind == channel.OutboundBrowse {
		return a.editBrowse(ctx, origin, msg)
	}
	return a.replyChat(ctx, origin, msg)
}

func (a *Adapter) replyCallback(ctx context.Context, origin channel.Origin, msg channel.OutboundMessage) error {
	switch msg.Kind {
	case channel.OutboundPromptBind:
		_, err := a.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: origin.AckID,
			Text:            "Please start the bot with /start <access_token>",
			ShowAlert:       true,
		})
		return err
	case channel.OutboundError:
		_, err := a.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: origin.AckID,
			Text:            msg.Error,
			ShowAlert:       true,
		})
		return err
	case channel.OutboundSaved:
		if msg.Memo == nil {
			return fmt.Errorf("missing memo for saved reply")
		}
		chatID, _ := strconv.ParseInt(origin.ChatID, 10, 64)
		messageID, _ := strconv.Atoi(origin.MessageID)
		pinnedMarker := ""
		if msg.Memo.Pinned {
			pinnedMarker = "📌"
		}
		_, err := a.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        fmt.Sprintf("Memo updated as %s with [%s](%s) %s", msg.Memo.Visibility, msg.Memo.Name, msg.Memo.URL, pinnedMarker),
			ParseMode:   models.ParseModeMarkdown,
			ReplyMarkup: keyboard(msg.Actions, msg.Memo.Name),
		})
		if err != nil {
			return err
		}
		_, err = a.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: origin.AckID,
			Text:            "Memo updated",
		})
		return err
	case channel.OutboundBrowse:
		return a.editBrowse(ctx, origin, msg)
	default:
		_, err := a.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: origin.AckID,
			Text:            "Memo updated",
		})
		return err
	}
}

func (a *Adapter) replyChat(ctx context.Context, origin channel.Origin, msg channel.OutboundMessage) error {
	chatID, _ := strconv.ParseInt(origin.ChatID, 10, 64)
	messageID, _ := strconv.Atoi(origin.MessageID)

	switch msg.Kind {
	case channel.OutboundPromptBind:
		_, err := a.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Please start the bot with /start <access_token>",
		})
		return err
	case channel.OutboundPromptUsage:
		text := usageText(msg.Prompt)
		_, err := a.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   text,
		})
		return err
	case channel.OutboundBound:
		_, err := a.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Hello %s!", msg.User),
		})
		return err
	case channel.OutboundError:
		_, err := a.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   msg.Error,
		})
		return err
	case channel.OutboundBrowse:
		if msg.Browse == nil {
			return fmt.Errorf("missing browse payload")
		}
		formatted := formatBrowse(msg.Browse)
		params := &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        formatted.Text,
			Entities:    formatted.Entities,
			ReplyMarkup: browseKeyboard(msg.Browse),
		}
		if msg.Browse.View == channel.BrowseDetail {
			params.LinkPreviewOptions = disableLinkPreview
		}
		_, err := a.bot.SendMessage(ctx, params)
		return err
	case channel.OutboundSaved:
		if msg.Memo == nil {
			return fmt.Errorf("missing memo for saved reply")
		}
		params := &bot.SendMessageParams{
			ChatID:              chatID,
			Text:                fmt.Sprintf("Content saved as %s with [%s](%s)", msg.Memo.Visibility, msg.Memo.Name, msg.Memo.URL),
			ParseMode:           models.ParseModeMarkdown,
			DisableNotification: true,
			ReplyMarkup:         keyboard(msg.Actions, msg.Memo.Name),
		}
		if messageID != 0 {
			params.ReplyParameters = &models.ReplyParameters{MessageID: messageID}
		}
		_, err := a.bot.SendMessage(ctx, params)
		return err
	default:
		return nil
	}
}

func (a *Adapter) editBrowse(ctx context.Context, origin channel.Origin, msg channel.OutboundMessage) error {
	if msg.Browse == nil {
		return fmt.Errorf("missing browse payload")
	}
	chatID, _ := strconv.ParseInt(origin.ChatID, 10, 64)
	messageID, _ := strconv.Atoi(origin.MessageID)
	formatted := formatBrowse(msg.Browse)
	params := &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        formatted.Text,
		Entities:    formatted.Entities,
		ReplyMarkup: browseKeyboard(msg.Browse),
	}
	if msg.Browse.View == channel.BrowseDetail {
		params.LinkPreviewOptions = disableLinkPreview
	}
	_, err := a.bot.EditMessageText(ctx, params)
	if err != nil {
		return err
	}
	if origin.AckID == "" {
		return nil
	}
	_, err = a.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: origin.AckID,
	})
	return err
}

func usageText(prompt string) string {
	switch prompt {
	case channel.CommandSearch:
		return "Usage: /search <words>"
	default:
		return "Usage: /start <access_token>"
	}
}

func keyboard(actions []channel.ActionHint, memoName string) *models.InlineKeyboardMarkup {
	if len(actions) == 0 || memoName == "" {
		return nil
	}
	row := make([]models.InlineKeyboardButton, 0, len(actions))
	for _, action := range actions {
		text := actionLabel(action.Name)
		if text == "" {
			continue
		}
		row = append(row, models.InlineKeyboardButton{
			Text:         text,
			CallbackData: fmt.Sprintf("%s %s", action.Name, memoName),
		})
	}
	if len(row) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{row},
	}
}

func actionLabel(name string) string {
	switch name {
	case channel.ActionPublic:
		return "Public"
	case channel.ActionPrivate:
		return "Private"
	case channel.ActionPin:
		return "Pin"
	default:
		return ""
	}
}
