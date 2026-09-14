package channel

import (
	"context"
	"time"
)

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
	// Edit tells the adapter to change an existing bot message at MessageID
	// instead of sending a new one. Used after a pending edit when there is
	// no callback AckID.
	Edit bool
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
	OutboundError
	OutboundPromptBind
	OutboundPromptUsage
	OutboundBrowse
)

const (
	ActionPublic        = "public"
	ActionPrivate       = "private"
	ActionProtected     = "protected"
	ActionPin           = "pin"
	ActionOpen          = "open"
	ActionNext          = "next"
	ActionPrev          = "prev"
	ActionBack          = "back"
	ActionEdit          = "edit"
	ActionDelete        = "delete"
	ActionDeleteConfirm = "delete.confirm"
	ActionTags          = "tags"
	ActionTag           = "tag"

	CommandStart  = "start"
	CommandSearch = "search"
	CommandList   = "list"
	CommandTags   = "tags"
	CommandCancel = "cancel"

	BrowsePlaceholder = "_"
)

type BrowseView int

const (
	BrowseList BrowseView = iota
	BrowseDetail
	BrowseTags
	BrowseConfirmDelete
	BrowseEditPrompt
)

type MemoInfo struct {
	Name       string
	UID        string
	Visibility string
	Pinned     bool
	URL        string
}

type MemoSummary struct {
	Name      string
	Snippet   string
	Content   string
	UpdatedAt time.Time
}

type TagCount struct {
	Name  string
	Count int32
}

type BrowsePayload struct {
	View    BrowseView
	Tag     string
	Query   string
	Start   int
	End     int
	HasPrev bool
	HasNext bool
	Items   []MemoSummary
	Tags    []TagCount
	Memo    *MemoInfo
	Content string
}

type ActionHint struct {
	Name string
}

type OutboundMessage struct {
	Kind    OutboundKind
	Memo    *MemoInfo
	Browse  *BrowsePayload
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

func IsBrowseAction(name string) bool {
	switch name {
	case ActionOpen, ActionNext, ActionPrev, ActionBack,
		ActionEdit, ActionDelete, ActionDeleteConfirm,
		ActionTags, ActionTag:
		return true
	default:
		return false
	}
}
