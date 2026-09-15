package feishu

import (
	"encoding/json"
	"strings"
)

type postMessage struct {
	Title   string           `json:"title"`
	Content [][]postElement  `json:"content"`
	ZhCn    *postLocaleBlock `json:"zh_cn"`
	EnUs    *postLocaleBlock `json:"en_us"`
}

type postLocaleBlock struct {
	Title   string          `json:"title"`
	Content [][]postElement `json:"content"`
}

type postElement struct {
	Tag      string   `json:"tag"`
	Text     string   `json:"text"`
	Href     string   `json:"href"`
	ImageKey string   `json:"image_key"`
	UserName string   `json:"user_name"`
	Style    []string `json:"style"`
}

func textFromContentJSON(raw string) string {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return strings.TrimSpace(raw)
	}
	return payload.Text
}

func postToMarkdown(raw string) (string, []string) {
	var post postMessage
	if err := json.Unmarshal([]byte(raw), &post); err != nil {
		return strings.TrimSpace(raw), nil
	}

	title, blocks := post.Title, post.Content
	if len(blocks) == 0 && post.ZhCn != nil {
		if title == "" {
			title = post.ZhCn.Title
		}
		blocks = post.ZhCn.Content
	}
	if len(blocks) == 0 && post.EnUs != nil {
		if title == "" {
			title = post.EnUs.Title
		}
		blocks = post.EnUs.Content
	}

	var images []string
	var lines []string
	if strings.TrimSpace(title) != "" {
		lines = append(lines, strings.TrimSpace(title))
	}
	for _, row := range blocks {
		var sb strings.Builder
		for _, el := range row {
			switch el.Tag {
			case "img":
				if el.ImageKey != "" {
					images = append(images, el.ImageKey)
				}
			case "a":
				label := el.Text
				if label == "" {
					label = el.Href
				}
				if el.Href != "" {
					sb.WriteString("[" + label + "](" + el.Href + ")")
				} else {
					sb.WriteString(applyPostStyle(label, el.Style))
				}
			case "at":
				name := el.UserName
				if name == "" {
					name = el.Text
				}
				if name != "" {
					sb.WriteString("@" + name)
				}
			default:
				sb.WriteString(applyPostStyle(el.Text, el.Style))
			}
		}
		if s := strings.TrimRight(sb.String(), " "); s != "" {
			lines = append(lines, s)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), images
}

func applyPostStyle(text string, styles []string) string {
	if text == "" {
		return ""
	}
	bold, italic := false, false
	for _, s := range styles {
		switch s {
		case "bold":
			bold = true
		case "italic":
			italic = true
		}
	}
	switch {
	case bold && italic:
		return "***" + text + "***"
	case bold:
		return "**" + text + "**"
	case italic:
		return "*" + text + "*"
	default:
		return text
	}
}
