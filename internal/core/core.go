package core

import (
	"sync"
	"time"

	"github.com/usememos/memogram/internal/channel"
	"github.com/usememos/memogram/internal/memos"
	"github.com/usememos/memogram/internal/store"
)

const mediaGroupTTL = 5 * time.Minute

type Core struct {
	store     *store.Store
	backend   Client
	publicURL string
	adapters  map[string]channel.Adapter
	groups    *groupCache

	instanceURL string

	mu     sync.Mutex
	browse map[string]*browseState
	edits  map[string]*pendingEdit
}

func New(st *store.Store, mc *memos.Client, publicURL string) *Core {
	return NewWithClient(st, wrapMemos(mc), publicURL)
}

func NewWithClient(st *store.Store, backend Client, publicURL string) *Core {
	return &Core{
		store:     st,
		backend:   backend,
		publicURL: publicURL,
		adapters:  make(map[string]channel.Adapter),
		groups:    newGroupCache(mediaGroupTTL),
		browse:    make(map[string]*browseState),
		edits:     make(map[string]*pendingEdit),
	}
}

func (c *Core) Register(a channel.Adapter) {
	c.adapters[a.Name()] = a
}
