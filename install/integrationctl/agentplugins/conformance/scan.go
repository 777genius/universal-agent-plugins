package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type duplicateMode uint8

const (
	duplicateRecord duplicateMode = iota
	duplicateRoot
	duplicateAll
)

type duplicate struct{ keys []string }
type parseFailure struct {
	Code  string
	cause error
}

func (e *parseFailure) Error() string { return e.Code }
func (e *parseFailure) Unwrap() error { return e.cause }

// scanJSON is the single structural token walk. Compatibility callers select
// their old duplicate boundary and no new limits; author callers record bounded
// duplicate locations before any map materialization.
func scanJSON(ctx context.Context, body []byte, limits Limits, mode duplicateMode) ([]duplicate, error) {
	// encoding/json map decoding already rejects nesting beyond 10000. Keep
	// that existing ceiling in the compatibility walk before recursive descent.
	if limits.Depth == 0 {
		limits.Depth = 10000
	}
	s := jsonScanner{ctx: ctx, decoder: json.NewDecoder(bytes.NewReader(body)), limits: limits, mode: mode}
	s.decoder.UseNumber()
	token, err := s.token()
	if err != nil {
		return nil, err
	}
	if mode == duplicateRoot && token != json.Delim('{') {
		return nil, fmt.Errorf("document must be a JSON object")
	}
	if err = s.value(token, nil, 0); err != nil {
		return nil, err
	}
	if _, err = s.decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("document contains multiple JSON values")
		}
		return nil, err
	}
	return s.duplicates, nil
}

type jsonScanner struct {
	ctx             context.Context
	decoder         *json.Decoder
	limits          Limits
	mode            duplicateMode
	tokens, members int
	duplicates      []duplicate
}

func (s *jsonScanner) token() (json.Token, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	s.tokens++
	if s.limits.Tokens > 0 && s.tokens > s.limits.Tokens {
		return nil, &parseFailure{Code: "document_token_limit"}
	}
	return s.decoder.Token()
}
func (s *jsonScanner) value(token json.Token, keys []string, depth int) error {
	delim, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	depth++
	if s.limits.Depth > 0 && depth > s.limits.Depth {
		return &parseFailure{Code: "document_depth_limit"}
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for s.decoder.More() {
			s.members++
			if s.limits.Members > 0 && s.members > s.limits.Members {
				return &parseFailure{Code: "document_member_limit"}
			}
			t, err := s.token()
			if err != nil {
				return err
			}
			key, ok := t.(string)
			if !ok {
				return fmt.Errorf("JSON object key must be a string")
			}
			if seen[key] {
				if s.mode == duplicateAll || (s.mode == duplicateRoot && depth == 1) {
					return fmt.Errorf("JSON object contains duplicate field %q", key)
				}
				if s.mode == duplicateRecord {
					if len(s.duplicates) >= s.limits.Diagnostics {
						return &parseFailure{Code: "document_duplicate_limit"}
					}
					s.duplicates = append(s.duplicates, duplicate{keys: append(append([]string(nil), keys...), key)})
				}
			}
			seen[key] = true
			t, err = s.token()
			if err != nil {
				return err
			}
			if err = s.value(t, append(keys, key), depth); err != nil {
				return err
			}
		}
		closing, err := s.token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not terminated")
		}
	case '[':
		for s.decoder.More() {
			t, err := s.token()
			if err != nil {
				return err
			}
			if err = s.value(t, keys, depth); err != nil {
				return err
			}
		}
		closing, err := s.token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return fmt.Errorf("JSON array is not terminated")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}
