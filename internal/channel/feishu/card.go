package feishu

import (
	"fmt"
	"strings"
	"time"

	"github.com/usememos/memogram/internal/channel"
)

const (
	buttonMaxRunes = 28
	cardMaxRunes   = 4000
)

func browseCard(b *channel.BrowsePayload) map[string]any {
	if b == nil {
		return newCard("", []any{markdownElement("没有找到备忘录。")})
	}
	switch b.View {
	case channel.BrowseTags:
		return tagsCard(b)
	case channel.BrowseDetail:
		return detailCard(b)
	case channel.BrowseConfirmDelete:
		return newCard("", []any{
			markdownElement(clipCardMarkdown(b.Content)),
			columnSet([]map[string]any{
				callbackButton("是", channel.ActionDeleteConfirm, channel.BrowsePlaceholder, "danger"),
				callbackButton("否", channel.ActionBack, channel.BrowsePlaceholder, "default"),
			}),
		})
	case channel.BrowseEditPrompt:
		return newCard("", []any{
			markdownElement(clipCardMarkdown(b.Content)),
			columnSet([]map[string]any{
				callbackButton("返回", channel.ActionBack, channel.BrowsePlaceholder, "default"),
			}),
		})
	default:
		return listCard(b)
	}
}

func listCard(b *channel.BrowsePayload) map[string]any {
	if len(b.Items) == 0 {
		return newCard(browseListTitle(b), []any{markdownElement("没有找到备忘录。")})
	}
	elements := make([]any, 0, len(b.Items)+1)
	for i, item := range b.Items {
		elements = append(elements, callbackButton(listButtonText(item), channel.ActionOpen, fmt.Sprintf("%d", i), "default"))
	}
	return newCard(browseListTitle(b), appendNav(elements, b, true))
}

func tagsCard(b *channel.BrowsePayload) map[string]any {
	title := fmt.Sprintf("标签 · %d–%d", b.Start, b.End)
	if len(b.Tags) == 0 {
		return newCard(title, []any{markdownElement("没有标签。")})
	}
	elements := make([]any, 0, len(b.Tags)+1)
	for i, tag := range b.Tags {
		label := fmt.Sprintf("#%s (%d)", tag.Name, tag.Count)
		elements = append(elements, callbackButton(truncateRunes(label, buttonMaxRunes), channel.ActionTag, fmt.Sprintf("%d", i), "default"))
	}
	return newCard(title, appendNav(elements, b, false))
}

func detailCard(b *channel.BrowsePayload) map[string]any {
	elements := []any{
		markdownElement(clipCardMarkdown(b.Content)),
		columnSet([]map[string]any{
			callbackButton("返回", channel.ActionBack, channel.BrowsePlaceholder, "default"),
			callbackButton("编辑", channel.ActionEdit, channel.BrowsePlaceholder, "default"),
			callbackButton("删除", channel.ActionDelete, channel.BrowsePlaceholder, "danger"),
		}),
	}
	if b.Memo != nil && b.Memo.URL != "" {
		elements = append(elements, urlButton("打开", b.Memo.URL))
	}
	return newCard("", elements)
}

func savedCard(prefix string, memo *channel.MemoInfo, actions []channel.ActionHint) map[string]any {
	body := prefix
	if memo.URL != "" {
		body = fmt.Sprintf("%s为 %s：[%s](%s)", prefix, memo.Visibility, memo.Name, memo.URL)
	} else {
		body = fmt.Sprintf("%s为 %s：%s", prefix, memo.Visibility, memo.Name)
	}
	if memo.Pinned {
		body += " 📌"
	}
	elements := []any{markdownElement(body)}
	if buttons := savedButtons(actions, memo.Name); len(buttons) > 0 {
		elements = append(elements, columnSet(buttons))
	}
	return newCard("", elements)
}

func savedButtons(actions []channel.ActionHint, memoName string) []map[string]any {
	if memoName == "" {
		return nil
	}
	row := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		label := actionLabel(action.Name)
		if label == "" {
			continue
		}
		row = append(row, callbackButton(label, action.Name, memoName, "default"))
	}
	return row
}

func actionLabel(name string) string {
	switch name {
	case channel.ActionPublic:
		return "公开"
	case channel.ActionPrivate:
		return "私密"
	case channel.ActionPin:
		return "置顶"
	default:
		return ""
	}
}

func browseListTitle(b *channel.BrowsePayload) string {
	rangeLabel := fmt.Sprintf("%d–%d", b.Start, b.End)
	switch {
	case b.Query != "":
		return fmt.Sprintf("搜索 · %s · %s", b.Query, rangeLabel)
	case b.Tag != "":
		return fmt.Sprintf("标签 · #%s · %s", b.Tag, rangeLabel)
	default:
		return "备忘录 · 最近更新 · " + rangeLabel
	}
}

func appendNav(elements []any, b *channel.BrowsePayload, tagsShortcut bool) []any {
	nav := make([]map[string]any, 0, 3)
	if b.HasPrev {
		nav = append(nav, callbackButton("上一页", channel.ActionPrev, channel.BrowsePlaceholder, "default"))
	}
	if tagsShortcut && b.Tag != "" {
		nav = append(nav, callbackButton("标签", channel.ActionTags, channel.BrowsePlaceholder, "default"))
	}
	if b.HasNext {
		nav = append(nav, callbackButton("下一页", channel.ActionNext, channel.BrowsePlaceholder, "default"))
	}
	if len(nav) == 0 {
		return elements
	}
	return append(elements, columnSet(nav))
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

func truncateRunes(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "无标题"
	}
	runes := []rune(s)
	if max < 1 || len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func clipCardMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "（空备忘录）"
	}
	runes := []rune(s)
	if len(runes) <= cardMaxRunes {
		return s
	}
	return string(runes[:cardMaxRunes-3]) + "..."
}

func newCard(header string, elements []any) map[string]any {
	card := map[string]any{
		"schema": "2.0",
		"body":   map[string]any{"elements": elements},
	}
	if header != "" {
		card["header"] = map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": header},
			"template": "blue",
		}
	}
	return card
}

func markdownElement(content string) map[string]any {
	return map[string]any{
		"tag":     "markdown",
		"content": content,
	}
}

func callbackButton(label, name, resource, btnType string) map[string]any {
	return map[string]any{
		"tag":   "button",
		"text":  map[string]any{"tag": "plain_text", "content": label},
		"type":  btnType,
		"width": "fill",
		"behaviors": []any{
			map[string]any{
				"type": "callback",
				"value": map[string]string{
					"name":     name,
					"resource": resource,
				},
			},
		},
	}
}

func urlButton(label, url string) map[string]any {
	return map[string]any{
		"tag":   "button",
		"text":  map[string]any{"tag": "plain_text", "content": label},
		"type":  "primary",
		"width": "fill",
		"behaviors": []any{
			map[string]any{
				"type":        "open_url",
				"default_url": url,
			},
		},
	}
}

func columnSet(buttons []map[string]any) map[string]any {
	columns := make([]any, 0, len(buttons))
	for _, btn := range buttons {
		columns = append(columns, map[string]any{
			"tag":            "column",
			"width":          "weighted",
			"weight":         1,
			"vertical_align": "center",
			"elements":       []any{btn},
		})
	}
	return map[string]any{
		"tag":                "column_set",
		"flex_mode":          "none",
		"background_style":   "default",
		"horizontal_spacing": "8px",
		"columns":            columns,
	}
}
