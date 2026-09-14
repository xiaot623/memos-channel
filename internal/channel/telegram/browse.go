package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/usememos/memogram/internal/channel"
)

const (
	buttonMaxRunes   = 28
	telegramMaxRunes = 4096
)

func browseText(b *channel.BrowsePayload) string {
	if b == nil {
		return "No memos found."
	}
	switch b.View {
	case channel.BrowseTags:
		if len(b.Tags) == 0 {
			return "No tags found."
		}
		return fmt.Sprintf("Tags · %d–%d", b.Start, b.End)
	case channel.BrowseDetail:
		text := strings.TrimSpace(b.Content)
		if text == "" {
			text = "(empty memo)"
		}
		return clipMessage(text)
	case channel.BrowseConfirmDelete:
		return clipMessage(b.Content)
	case channel.BrowseEditPrompt:
		return clipMessage(b.Content)
	default:
		if len(b.Items) == 0 {
			return "No memos found."
		}
		return browseListTitle(b)
	}
}

func browseListTitle(b *channel.BrowsePayload) string {
	rangeLabel := fmt.Sprintf("%d–%d", b.Start, b.End)
	switch {
	case b.Query != "":
		return fmt.Sprintf("Search · %s · %s", b.Query, rangeLabel)
	case b.Tag != "":
		return fmt.Sprintf("Tag · #%s · %s", b.Tag, rangeLabel)
	default:
		return "Memos · updated · " + rangeLabel
	}
}

func browseKeyboard(b *channel.BrowsePayload) *models.InlineKeyboardMarkup {
	if b == nil {
		return nil
	}
	switch b.View {
	case channel.BrowseTags:
		return tagsKeyboard(b)
	case channel.BrowseDetail:
		return markupRows([][]models.InlineKeyboardButton{{
			browseButton("Back", channel.ActionBack),
			browseButton("Edit", channel.ActionEdit),
			browseButton("Delete", channel.ActionDelete),
		}})
	case channel.BrowseConfirmDelete:
		return markupRows([][]models.InlineKeyboardButton{{
			browseButton("Yes", channel.ActionDeleteConfirm),
			browseButton("No", channel.ActionBack),
		}})
	case channel.BrowseEditPrompt:
		return markupRows([][]models.InlineKeyboardButton{{
			browseButton("Back", channel.ActionBack),
		}})
	default:
		return listKeyboard(b)
	}
}

func listKeyboard(b *channel.BrowsePayload) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(b.Items)+1)
	for i, item := range b.Items {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         listButtonText(item),
			CallbackData: fmt.Sprintf("%s %d", channel.ActionOpen, i),
		}})
	}
	nav := make([]models.InlineKeyboardButton, 0, 3)
	if b.HasPrev {
		nav = append(nav, browseButton("Prev", channel.ActionPrev))
	}
	if b.Tag != "" {
		nav = append(nav, browseButton("Tags", channel.ActionTags))
	}
	if b.HasNext {
		nav = append(nav, browseButton("Next", channel.ActionNext))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	return markupRows(rows)
}

func tagsKeyboard(b *channel.BrowsePayload) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(b.Tags)+1)
	for i, tag := range b.Tags {
		label := fmt.Sprintf("#%s (%d)", tag.Name, tag.Count)
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         truncateButton(label),
			CallbackData: fmt.Sprintf("%s %d", channel.ActionTag, i),
		}})
	}
	nav := make([]models.InlineKeyboardButton, 0, 2)
	if b.HasPrev {
		nav = append(nav, browseButton("Prev", channel.ActionPrev))
	}
	if b.HasNext {
		nav = append(nav, browseButton("Next", channel.ActionNext))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	return markupRows(rows)
}

func browseButton(text, action string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{
		Text:         text,
		CallbackData: action + " " + channel.BrowsePlaceholder,
	}
}

func markupRows(rows [][]models.InlineKeyboardButton) *models.InlineKeyboardMarkup {
	if len(rows) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func listButtonText(item channel.MemoSummary) string {
	prefix := listDatePrefix(item.UpdatedAt)
	body := listPreview(item)
	budget := buttonMaxRunes - len([]rune(prefix))
	if budget < 1 {
		return truncateRunes(prefix+body, buttonMaxRunes)
	}
	return prefix + truncateRunes(body, budget)
}

func listDatePrefix(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("(0102) ")
}

func listPreview(item channel.MemoSummary) string {
	if snippet := strings.TrimSpace(item.Snippet); snippet != "" {
		return snippet
	}
	return strings.TrimSpace(item.Content)
}

func truncateButton(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "Untitled"
	}
	return truncateRunes(s, buttonMaxRunes)
}

func truncateRunes(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "Untitled"
	}
	runes := []rune(s)
	if max < 1 || len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func clipMessage(s string) string {
	runes := []rune(s)
	if len(runes) <= telegramMaxRunes {
		return s
	}
	return string(runes[:telegramMaxRunes-3]) + "..."
}
