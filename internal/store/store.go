package store

import (
	"fmt"
	"sync"
)

type Store struct {
	Data string

	userAccessTokenCache sync.Map // map[cacheKey]string
}

func New(data string) *Store {
	return &Store{
		Data:                 data,
		userAccessTokenCache: sync.Map{},
	}
}

func (s *Store) Init() error {
	if err := s.loadUserAccessTokenMapFromFile(); err != nil {
		return fmt.Errorf("failed to load user access token map from file: %w", err)
	}
	return nil
}

func cacheKey(channel, userID string) string {
	return channel + "\x1f" + userID
}
