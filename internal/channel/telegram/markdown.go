package telegram

import (
	"strings"

	tgmd "github.com/eekstunt/telegramify-markdown-go"
	"github.com/go-telegram/bot/models"
	"github.com/usememos/memogram/internal/channel"
)

var noHeadingSymbols = tgmd.WithHeadingSymbols([6]string{})

type formattedText struct {
	Text     string
	Entities []models.MessageEntity
}

func formatBrowse(b *channel.BrowsePayload) formattedText {
	if b != nil && b.View == channel.BrowseDetail {
		return formatMemoContent(b.Content)
	}
	return formattedText{Text: browseText(b)}
}

func formatMemoContent(content string) formattedText {
	content = strings.TrimSpace(content)
	if content == "" {
		return formattedText{Text: "(empty memo)"}
	}
	msg := tgmd.Convert(content, noHeadingSymbols)
	if tgmd.UTF16Len(msg.Text) <= telegramMaxRunes {
		return formattedText{Text: msg.Text, Entities: toEntities(msg.Entities)}
	}
	parts := tgmd.ConvertAndSplit(content, noHeadingSymbols, tgmd.WithMaxMessageLen(telegramMaxRunes-3))
	if len(parts) == 0 {
		return formattedText{Text: "(empty memo)"}
	}
	text := parts[0].Text
	if len(parts) > 1 {
		text += "..."
	}
	return formattedText{Text: text, Entities: toEntities(parts[0].Entities)}
}

func toEntities(ents []tgmd.Entity) []models.MessageEntity {
	if len(ents) == 0 {
		return nil
	}
	out := make([]models.MessageEntity, len(ents))
	for i, e := range ents {
		out[i] = models.MessageEntity{
			Type:     models.MessageEntityType(e.Type),
			Offset:   e.Offset,
			Length:   e.Length,
			URL:      e.URL,
			Language: e.Language,
		}
	}
	return out
}
