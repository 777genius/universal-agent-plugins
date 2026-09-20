package pluginmodel

func renderPortableMCPStdioTransport(target string, stdio *PortableMCPStdio) map[string]any {
	if stdio == nil {
		return nil
	}
	target = NormalizeTarget(target)
	switch target {
	case "opencode":
		command := make([]any, 0, 1+len(stdio.Args))
		command = append(command, stdio.Command)
		for _, arg := range stdio.Args {
			command = append(command, arg)
		}
		out := map[string]any{
			"type":    "local",
			"command": command,
		}
		if len(stdio.Env) > 0 {
			out["environment"] = stringMapToAny(stdio.Env)
		}
		return out
	default:
		out := map[string]any{
			"command": stdio.Command,
		}
		if len(stdio.Args) > 0 {
			args := make([]any, 0, len(stdio.Args))
			for _, arg := range stdio.Args {
				args = append(args, arg)
			}
			out["args"] = args
		}
		if len(stdio.Env) > 0 {
			out["env"] = stringMapToAny(stdio.Env)
		}
		return out
	}
}

func renderPortableMCPRemoteTransport(target string, remote *PortableMCPRemote) map[string]any {
	if remote == nil {
		return nil
	}
	target = NormalizeTarget(target)
	switch target {
	case "gemini":
		out := map[string]any{}
		if remote.Protocol == "sse" {
			out["url"] = remote.URL
		} else {
			out["httpUrl"] = remote.URL
		}
		if len(remote.Headers) > 0 {
			out["headers"] = stringMapToAny(remote.Headers)
		}
		return out
	case "opencode":
		out := map[string]any{
			"type": "remote",
			"url":  remote.URL,
		}
		if len(remote.Headers) > 0 {
			out["headers"] = stringMapToAny(remote.Headers)
		}
		return out
	default:
		kind := "http"
		if remote.Protocol == "sse" {
			kind = "sse"
		}
		out := map[string]any{
			"type": kind,
			"url":  remote.URL,
		}
		if len(remote.Headers) > 0 {
			out["headers"] = stringMapToAny(remote.Headers)
		}
		return out
	}
}
