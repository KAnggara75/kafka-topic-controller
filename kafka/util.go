package kafka

import "strings"

func normalizeConfig(cfg map[string]string) map[string]string {
	out := make(map[string]string, len(cfg))

	for k, v := range cfg {
		key := strings.ToLower(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		out[key] = val
	}

	return out
}
