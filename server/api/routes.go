package api

import (
	"strings"
)

func pathParts(path, prefix string) ([]string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return nil, false
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return []string{}, true
	}
	return strings.Split(rest, "/"), true
}
