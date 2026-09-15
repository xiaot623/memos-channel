package feishu

import "strings"

func parseAllowedOpenIDs(raw string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, entry := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}
	return allowed
}

func (a *Adapter) isUserAllowed(openID string) bool {
	if len(a.allowedOpenIDs) == 0 {
		return true
	}
	if openID == "" {
		return false
	}
	_, ok := a.allowedOpenIDs[strings.TrimSpace(openID)]
	return ok
}
