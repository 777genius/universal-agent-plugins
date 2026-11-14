package skillscatalog

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

func invalidRequest(message string) error { return errors.New(message) }
func requestValues(raw string) (url.Values, error) {
	if len(raw) > 2048 {
		return nil, invalidRequest("Query string is too long")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, invalidRequest("Invalid query string")
	}
	for key, items := range values {
		if len(items) != 1 {
			return nil, invalidRequest("Duplicate query parameters are not allowed")
		}
		if key != "view" && key != "page" && key != "q" && key != "limit" {
			return nil, invalidRequest("Unknown query parameter")
		}
	}
	return values, nil
}
func parseRequest(raw string) (Request, error) {
	q, err := requestValues(raw)
	if err != nil {
		return Request{}, err
	}
	r := Request{View: Popular}
	if q.Has("view") {
		r.View = View(q.Get("view"))
		if !r.View.Valid() {
			return Request{}, invalidRequest("view must be popular, trending or hot")
		}
	}
	if q.Has("page") {
		r.Page, err = strconv.Atoi(q.Get("page"))
		if err != nil || r.Page < 0 || r.Page > 10000 {
			return Request{}, invalidRequest("page must be between 0 and 10000")
		}
	}
	if q.Has("q") {
		return searchRequest(r, q)
	}
	if q.Has("limit") {
		return Request{}, invalidRequest("limit is only supported with q")
	}
	return r, nil
}
func searchRequest(r Request, q url.Values) (Request, error) {
	if !safeText(q.Get("q"), 200) {
		return Request{}, invalidRequest("q contains unsupported characters")
	}
	r.Query = strings.TrimSpace(q.Get("q"))
	if !utf8.ValidString(r.Query) || utf8.RuneCountInString(r.Query) < 2 || !safeText(r.Query, 200) {
		return Request{}, invalidRequest("q must contain 2 or more characters, up to 200 bytes")
	}
	if r.Page != 0 {
		return Request{}, invalidRequest("Search supports only page 0")
	}
	r.View = Popular
	r.Limit = 100
	if q.Has("limit") {
		var err error
		r.Limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || r.Limit < 1 || r.Limit > 100 {
			return Request{}, invalidRequest("limit must be between 1 and 100")
		}
	}
	return r, nil
}
