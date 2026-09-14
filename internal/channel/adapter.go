package channel

import "context"

// Adapter is a long-connection channel (Telegram long poll, later Gateway/WebSocket).
// Implementations must not call the Memos API; Core must not import bot SDKs.
type Adapter interface {
	Name() string
	// Start maintains the inbound long connection until ctx is cancelled.
	Start(ctx context.Context, handle HandleFunc) error
	Reply(ctx context.Context, origin Origin, msg OutboundMessage) error
}

// HandleFunc is Core's inbound entry. Adapters call it after normalizing SDK types.
type HandleFunc func(ctx context.Context, ev InboundEvent) error
