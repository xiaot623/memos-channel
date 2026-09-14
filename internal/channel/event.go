package channel

import "context"

const (
	Telegram = "telegram"
)

type Kind int

const (
	KindMessage Kind = iota
	KindCommand
	KindAction
)

type Origin struct {
	ChatID    string
	MessageID string
	AckID     string
}

type Command struct {
	Name string
	Args string
}

type Action struct {
	Name     string
	Resource string
}

type Attachment struct {
	Filename    string
	ContentType string
	Bytes       []byte
}

type AttachmentRef interface {
	Filename() string
	Download(ctx context.Context) (*Attachment, error)
}

type InboundEvent struct {
	Channel        string
	PlatformUserID string
	Origin         Origin
	Kind           Kind
	Command        Command
	Action         Action
	TextMarkdown   string
	Attachments    []AttachmentRef
	GroupID        string
}

type OutboundKind int

const (
	OutboundSaved OutboundKind = iota
	OutboundBound
	OutboundSearchList
	OutboundError
	OutboundPromptBind
	OutboundPromptUsage
)

const (
	ActionPublic    = "public"
	ActionPrivate   = "private"
	ActionProtected = "protected"
	ActionPin       = "pin"

	CommandStart  = "start"
	CommandSearch = "search"
)

type MemoInfo struct {
	Name       string
	UID        string
	Visibility string
	Pinned     bool
	URL        string
}

type MemoSummary struct {
	Name    string
	Content string
}

type ActionHint struct {
	Name string
}

type OutboundMessage struct {
	Kind    OutboundKind
	Memo    *MemoInfo
	Results []MemoSummary
	Actions []ActionHint
	Error   string
	User    string
	Prompt  string
}

func DefaultMemoActions() []ActionHint {
	return []ActionHint{
		{Name: ActionPublic},
		{Name: ActionPrivate},
		{Name: ActionPin},
	}
}
