package telegram

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/go-telegram/bot/models"
)

func formatContent(content string, contentEntities []models.MessageEntity) string {
	sort.Slice(contentEntities, func(i, j int) bool {
		if contentEntities[i].Offset == contentEntities[j].Offset {
			return contentEntities[i].Length < contentEntities[j].Length
		}
		return contentEntities[i].Offset < contentEntities[j].Offset
	})

	contentRunes := utf16.Encode([]rune(content))

	var sb strings.Builder
	cursor := 0
	for _, entity := range contentEntities {
		if !isSupportedEntity(entity.Type) {
			continue
		}
		start := entity.Offset
		end := entity.Offset + entity.Length
		if start < cursor {
			continue
		}
		if start >= len(contentRunes) {
			break
		}
		if end > len(contentRunes) {
			end = len(contentRunes)
		}

		sb.WriteString(string(utf16.Decode(contentRunes[cursor:start])))
		segment := string(utf16.Decode(contentRunes[start:end]))
		sb.WriteString(applyEntityFormatting(segment, entity))
		cursor = end
	}
	sb.WriteString(string(utf16.Decode(contentRunes[cursor:])))
	return sb.String()
}

func isSupportedEntity(entityType models.MessageEntityType) bool {
	switch entityType {
	case models.MessageEntityTypeURL,
		models.MessageEntityTypeTextLink,
		models.MessageEntityTypeBold,
		models.MessageEntityTypeItalic:
		return true
	default:
		return false
	}
}

func applyEntityFormatting(segment string, entity models.MessageEntity) string {
	if strings.TrimSpace(segment) == "" {
		return segment
	}
	re := regexp.MustCompile(`^(\s*)(.*?)(\s*)$`)
	matches := re.FindStringSubmatch(segment)
	if len(matches) != 4 {
		return segment
	}
	prefix, core, suffix := matches[1], matches[2], matches[3]
	switch entity.Type {
	case models.MessageEntityTypeURL:
		return fmt.Sprintf("%s[%s](%s)%s", prefix, core, core, suffix)
	case models.MessageEntityTypeTextLink:
		return fmt.Sprintf("%s[%s](%s)%s", prefix, core, entity.URL, suffix)
	case models.MessageEntityTypeBold:
		return fmt.Sprintf("%s**%s**%s", prefix, core, suffix)
	case models.MessageEntityTypeItalic:
		return fmt.Sprintf("%s*%s*%s", prefix, core, suffix)
	default:
		return segment
	}
}
