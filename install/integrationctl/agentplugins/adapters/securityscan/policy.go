package securityscan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// blockingRuleCodes is intentionally narrow and versioned. It contains
// high-confidence executable behavior that can persist, damage the host, or
// exfiltrate sensitive data. Context-sensitive prose findings remain visible
// warnings so that install confirmation is reserved for direct evidence.
var blockingRuleCodes = map[string]struct{}{
	"SEC103": {}, "SEC330": {}, "SEC344": {},
	"SEC637": {}, "SEC640": {}, "SEC645": {}, "SEC648": {},
	"SEC652": {}, "SEC653": {}, "SEC654": {}, "SEC658": {}, "SEC659": {}, "SEC660": {},
	"SEC665": {}, "SEC666": {}, "SEC671": {}, "SEC672": {},
	"SEC674": {}, "SEC675": {}, "SEC676": {}, "SEC680": {}, "SEC681": {}, "SEC682": {},
	"SEC684": {}, "SEC686": {}, "SEC697": {}, "SEC698": {}, "SEC701": {}, "SEC702": {},
	"SEC706": {}, "SEC710": {}, "SEC717": {}, "SEC718": {}, "SEC725": {}, "SEC726": {},
	"SEC729": {}, "SEC730": {}, "SEC733": {}, "SEC734": {}, "SEC738": {}, "SEC742": {},
}

func DefaultRequirement() domain.SecurityRequirement {
	return domain.SecurityRequirement{
		Scanner: domain.SecurityScanner{ID: domain.SecurityScannerID, Version: domain.SecurityScannerVersion},
		Policy:  domain.SecurityPolicy{ID: domain.SecurityPolicyID, Version: domain.SecurityPolicyVersion, Digest: policyDigest()},
	}
}

func policyDigest() string {
	codes := make([]string, 0, len(blockingRuleCodes))
	for code := range blockingRuleCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	document := map[string]any{
		"id": domain.SecurityPolicyID, "version": domain.SecurityPolicyVersion,
		"blocking_confidence": "high", "blocking_rule_codes": codes, "deny_is_blocking": true,
	}
	body, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	// The signed Security Index uses the registry's canonical JSON profile,
	// where a trailing LF is part of the bytes covered by every digest.
	body = append(body, '\n')
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func disposition(code, severity, confidence string) string {
	if severity == "deny" {
		return "blocking"
	}
	if confidence == "high" {
		if _, ok := blockingRuleCodes[code]; ok {
			return "blocking"
		}
	}
	return "warning"
}
