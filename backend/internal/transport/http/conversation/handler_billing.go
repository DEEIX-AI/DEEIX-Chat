package conversation

import (
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

type billingRequestInput struct {
	UserID            uint
	Conversation      *model.Conversation
	PlatformModelName string
	ClientRunID       string
}

// buildBillingInput 构造消息与媒体请求共用的计费上下文。
func buildBillingInput(request billingRequestInput) appconversation.SendMessageBillingInput {
	input := appconversation.SendMessageBillingInput{
		UserID:            request.UserID,
		PlatformModelName: strings.TrimSpace(request.PlatformModelName),
		ClientRunID:       strings.TrimSpace(request.ClientRunID),
	}
	if request.Conversation != nil {
		input.ConversationID = request.Conversation.ID
		input.ConversationModel = request.Conversation.Model
		input.Conversation = request.Conversation
	}
	return input
}
