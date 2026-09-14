package telegram

import "strings"

func parseAllowedUsernames(raw string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, entry := range strings.Split(raw, ",") {
		trimmed := strings.ToLower(strings.TrimSpace(entry))
		if trimmed == "" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}
	return allowed
}

func (a *Adapter) isUserAllowed(username string) bool {
	if len(a.allowedUsernames) == 0 {
		return true
	}
	if username == "" {
		return false
	}
	_, ok := a.allowedUsernames[strings.ToLower(strings.TrimSpace(username))]
	return ok
}
