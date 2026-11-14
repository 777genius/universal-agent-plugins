//go:build linux

package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"time"
)

func validateTrustedProbeEvents(body []byte, facts map[string]any, started, ended time.Time) error {
	lines := bytes.Split(bytes.TrimSpace(body), []byte{'\n'})
	if len(lines) < 3 || len(lines) > 32 {
		return fmt.Errorf("fresh probe event inventory differs")
	}
	progress := trustedProbeProgress{}
	for _, line := range lines {
		event, err := trustedProbeEvent(line, facts, started, ended)
		if err != nil {
			return err
		}
		if err := progress.observe(event, facts); err != nil {
			return err
		}
	}
	if progress.initCount != 1 || progress.listCount < 1 || progress.callCount != 1 {
		return fmt.Errorf("native probe initialize/list/call incomplete")
	}
	return nil
}

func trustedProbeEvent(line []byte, facts map[string]any, started, ended time.Time) (map[string]any, error) {
	value, err := uniqueJSON(line)
	if err != nil {
		return nil, err
	}
	event, err := objectKeys(value, strings.Fields("method pid session nonce timestamp facts"), nil)
	if err != nil || event["pid"] != facts["pid"] || event["session"] != facts["session"] || event["nonce"] != facts["nonce"] {
		return nil, fmt.Errorf("probe process/session/nonce does not join native result")
	}
	timestamp, ok := event["timestamp"].(string)
	when, err := time.Parse(time.RFC3339Nano, timestamp)
	if !ok || err != nil || when.Before(started) || when.After(ended) {
		return nil, fmt.Errorf("probe event is outside actual native stage")
	}
	return event, nil
}

type trustedProbeProgress struct {
	initialized, listed, called     bool
	initCount, listCount, callCount int
}

func (p *trustedProbeProgress) observe(event, facts map[string]any) error {
	switch event["method"] {
	case "initialize":
		if p.initialized || p.listed || p.called || event["facts"] != nil {
			return fmt.Errorf("probe initialize ordering differs")
		}
		p.initialized = true
		p.initCount++
	case "tools/list":
		if !p.initialized || p.called || event["facts"] != nil {
			return fmt.Errorf("probe catalog ordering differs")
		}
		p.listed = true
		p.listCount++
	case "tools/call":
		if !p.initialized || !p.listed || p.called || !reflect.DeepEqual(event["facts"], facts) {
			return fmt.Errorf("probe call does not join exact returned facts")
		}
		p.called = true
		p.callCount++
	default:
		return fmt.Errorf("unknown probe event")
	}
	return nil
}
