package platformexec

func mergeOpenCodeObject(dst, src map[string]any) {
	if len(src) == 0 {
		return
	}
	for key, value := range src {
		existing, hasExisting := dst[key].(map[string]any)
		incoming, incomingIsMap := value.(map[string]any)
		if hasExisting && incomingIsMap {
			mergeOpenCodeObject(existing, incoming)
			dst[key] = existing
			continue
		}
		dst[key] = value
	}
}
