package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type record struct {
	Channel string `json:"channel"`
	UserID  string `json:"user_id"`
	Token   string `json:"token"`
}

func (s *Store) Get(channel, userID string) (string, bool) {
	accessToken, ok := s.userAccessTokenCache.Load(cacheKey(channel, userID))
	if !ok {
		return "", false
	}
	return accessToken.(string), true
}

func (s *Store) Set(channel, userID, accessToken string) {
	s.userAccessTokenCache.Store(cacheKey(channel, userID), accessToken)
	if err := s.saveUserAccessTokenMapToFile(); err != nil {
		slog.Error("failed to save user access token map to file", "error", err)
	}
}

func (s *Store) saveUserAccessTokenMapToFile() error {
	entries := s.snapshotAccessTokens()
	dataDir := filepath.Dir(s.Data)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(dataDir, "memogram-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	writer := bufio.NewWriter(tmpFile)
	for _, entry := range entries {
		line, err := json.Marshal(record{
			Channel: entry.channel,
			UserID:  entry.userID,
			Token:   entry.accessToken,
		})
		if err != nil {
			tmpFile.Close()
			return fmt.Errorf("encode data file: %w", err)
		}
		if _, err := writer.Write(line); err != nil {
			tmpFile.Close()
			return fmt.Errorf("write data file: %w", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			tmpFile.Close()
			return fmt.Errorf("write data file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("flush data file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("sync data file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close data file: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), s.Data); err != nil {
		return fmt.Errorf("replace data file: %w", err)
	}
	return nil
}

func (s *Store) loadUserAccessTokenMapFromFile() error {
	if _, err := os.Stat(s.Data); os.IsNotExist(err) {
		file, err := os.Create(s.Data)
		if err != nil {
			return err
		}
		return file.Close()
	} else if err != nil {
		return err
	}

	file, err := os.Open(s.Data)
	if err != nil {
		return err
	}
	defer file.Close()

	migrated := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		channel, userID, accessToken, legacy, ok := parseLine(scanner.Text())
		if !ok {
			continue
		}
		if legacy {
			migrated = true
		}
		s.userAccessTokenCache.Store(cacheKey(channel, userID), accessToken)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if migrated {
		return s.saveUserAccessTokenMapToFile()
	}
	return nil
}

func parseLine(line string) (channel, userID, accessToken string, legacy, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", "", false, false
	}
	if strings.HasPrefix(line, "{") {
		var rec record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return "", "", "", false, false
		}
		if rec.Channel == "" || rec.UserID == "" || rec.Token == "" {
			return "", "", "", false, false
		}
		return rec.Channel, rec.UserID, rec.Token, false, true
	}

	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", "", false, false
	}
	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		return "", "", "", false, false
	}
	if parts[1] == "" {
		return "", "", "", false, false
	}
	return "telegram", parts[0], parts[1], true, true
}

type userAccessTokenEntry struct {
	channel     string
	userID      string
	accessToken string
}

func (s *Store) snapshotAccessTokens() []userAccessTokenEntry {
	entries := make([]userAccessTokenEntry, 0)
	s.userAccessTokenCache.Range(func(key, value interface{}) bool {
		rawKey, ok := key.(string)
		if !ok {
			return true
		}
		accessToken, ok := value.(string)
		if !ok {
			return true
		}
		channel, userID, ok := splitCacheKey(rawKey)
		if !ok {
			return true
		}
		entries = append(entries, userAccessTokenEntry{
			channel:     channel,
			userID:      userID,
			accessToken: accessToken,
		})
		return true
	})

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].channel != entries[j].channel {
			return entries[i].channel < entries[j].channel
		}
		return entries[i].userID < entries[j].userID
	})

	return entries
}

func splitCacheKey(key string) (channel, userID string, ok bool) {
	parts := strings.SplitN(key, "\x1f", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
