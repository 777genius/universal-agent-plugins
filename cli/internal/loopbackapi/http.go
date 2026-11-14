// Package loopbackapi contains the transport protection shared by local UAP APIs.
// It does not own installation policy, sessions, or accepted operations.
package loopbackapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
)

type ErrorWriter func(http.ResponseWriter, int, string, string, bool)

// Guard keeps the bearer capability and exact loopback authority inseparable.
// Static picker assets remain public on this exact host; every /api/ route is private.
func Guard(host, origin, token string, fail ErrorWriter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Host != host {
			fail(w, http.StatusForbidden, "INVALID_HOST", "Unexpected loopback host", false)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && !authorized(w, r, origin, token, fail) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authorized(w http.ResponseWriter, r *http.Request, origin, token string, fail ErrorWriter) bool {
	expected := "Bearer " + token
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		fail(w, http.StatusUnauthorized, "UNAUTHORIZED", "The session capability is missing or invalid", false)
		return false
	}
	incoming := r.Header.Get("Origin")
	mutation := r.Method != http.MethodGet && r.Method != http.MethodHead
	if (mutation && incoming != origin) || (incoming != "" && incoming != origin) {
		fail(w, http.StatusForbidden, "INVALID_ORIGIN", "Unexpected request origin", false)
		return false
	}
	return true
}

// Decode accepts one bounded JSON document, including only known request fields.
// Generated compile-time types are not a substitute for this runtime boundary.
func Decode(w http.ResponseWriter, r *http.Request, value any, fail ErrorWriter) bool {
	if ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || ct != "application/json" {
		fail(w, http.StatusUnsupportedMediaType, "INVALID_REQUEST", "Content-Type must be application/json", false)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		fail(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid or oversized JSON request", false)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		fail(w, http.StatusBadRequest, "INVALID_REQUEST", "Expected one JSON document", false)
		return false
	}
	return true
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func RandomID() string {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes[:])
}
