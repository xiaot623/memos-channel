package telegram

import (
	"fmt"

	"github.com/go-telegram/bot/models"
)

func prependForwardOrigin(content string, message *models.Message) string {
	if message == nil || message.ForwardOrigin == nil {
		return content
	}

	var originName, originUsername string
	switch origin := message.ForwardOrigin; {
	case origin.MessageOriginUser != nil:
		user := origin.MessageOriginUser.SenderUser
		if user.LastName != "" {
			originName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		} else {
			originName = user.FirstName
		}
		originUsername = user.Username
	case origin.MessageOriginHiddenUser != nil:
		hiddenUserName := origin.MessageOriginHiddenUser.SenderUserName
		if hiddenUserName != "" {
			originName = hiddenUserName
		} else {
			originName = "Hidden User"
		}
	case origin.MessageOriginChat != nil:
		chat := origin.MessageOriginChat.SenderChat
		originName = chat.Title
		originUsername = chat.Username
	case origin.MessageOriginChannel != nil:
		channel := origin.MessageOriginChannel.Chat
		originName = channel.Title
		originUsername = channel.Username
	}

	if originUsername != "" {
		return fmt.Sprintf("Forwarded from [%s](https://t.me/%s)\n%s", originName, originUsername, content)
	}
	return fmt.Sprintf("Forwarded from %s\n%s", originName, content)
}
