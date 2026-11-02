//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
)

type geminiCycleAssessmentFrame struct {
	Before, After map[string]geminiCycleRecord
	Ledger        map[string]geminiCycleAssessment
}

func geminiCycleAssessmentLeaves(ledger map[string]geminiCycleAssessment, current *geminiCycleAssessment) (map[string]geminiCycleAssessment, map[string]geminiCycleAssessment, error) {
	next, leaves := map[string]geminiCycleAssessment{}, map[string]geminiCycleAssessment{}
	for slot, assessment := range ledger {
		if assessment.Slot != slot {
			return nil, nil, fmt.Errorf("Gemini held cache ledger identity")
		}
		next[slot] = assessment
	}
	if current != nil {
		if previous, had := next[current.Slot]; had && !reflect.DeepEqual(previous, *current) {
			return nil, nil, fmt.Errorf("Gemini source revision rebound held cache identity")
		}
		next[current.Slot] = *current
	}
	if len(next) > 4 {
		return nil, nil, fmt.Errorf("Gemini finite assessment ledger bound")
	}
	for slot, assessment := range next {
		if !map[string]bool{"user/U-one": true, "a/A-one": true, "b/B-one": true, "a/A-two": true}[slot] || !geminiCycleDigest("sha256:"+assessment.Key) || len(assessment.Body) == 0 || len(assessment.Body) > 4096 {
			return nil, nil, fmt.Errorf("Gemini independently held cache descriptor")
		}
		name := "security/assessments/" + assessment.Key + ".json"
		if _, duplicate := leaves[name]; duplicate {
			return nil, nil, fmt.Errorf("Gemini scopes share assessment identity")
		}
		assessment.Body = bytes.Clone(assessment.Body)
		next[slot], leaves[name] = assessment, assessment
	}
	return next, leaves, nil
}
func geminiCycleAssessmentClosure(ctx context.Context, before, after map[string]geminiCycleRecord, ledger map[string]geminiCycleAssessment, current *geminiCycleAssessment) (geminiCycleAssessmentFrame, error) {
	refuse := func() (geminiCycleAssessmentFrame, error) {
		return geminiCycleAssessmentFrame{}, fmt.Errorf("Gemini exact source-bound assessment cache closure")
	}
	if ctx == nil || ctx.Err() != nil || before == nil || after == nil {
		return refuse()
	}
	next, leaves, err := geminiCycleAssessmentLeaves(ledger, current)
	if err != nil {
		return geminiCycleAssessmentFrame{}, err
	}
	for _, name := range []string{"security", "security/assessments"} {
		a, had := before[name]
		b, has := after[name]
		if had || has {
			if !geminiCycleParent(a, b, had, has) || !had && current == nil || current == nil && a != b {
				return refuse()
			}
		}
		if len(next) > 0 && !has || name == "security/assessments" && has && len(next) == 0 {
			return refuse()
		}
	}
	for name, expected := range leaves {
		a, had := before[name]
		b, has := after[name]
		_, retained := ledger[expected.Slot]
		if !has || b.Mode != 0100000|0600 || b.Links != 1 || b.Size != int64(len(expected.Body)) || b.SHA256 != geminiCycleSHA(expected.Body) || retained && (!had || a != b) || !retained && had {
			return refuse()
		}
	}
	frame := geminiCycleAssessmentFrame{Before: map[string]geminiCycleRecord{}, After: map[string]geminiCycleRecord{}, Ledger: next}
	for name := range geminiCycleKeys(before, after) {
		if ctx.Err() != nil {
			return refuse()
		}
		if strings.HasPrefix(name, "security/assessments/") {
			if _, known := leaves[name]; !known {
				return refuse()
			}
			continue
		}
		if name == "security" || name == "security/assessments" {
			continue
		}
		if record, had := before[name]; had {
			frame.Before[name] = record
		}
		if record, has := after[name]; has {
			frame.After[name] = record
		}
	}
	return frame, nil
}
func (frame geminiCycleAssessmentFrame) globalUnchanged(sourceOperation bool) error {
	before, after := frame.Before, frame.After
	if sourceOperation {
		a, had := before["."]
		b, has := after["."]
		if !geminiCycleParent(a, b, had, has) {
			return fmt.Errorf("Gemini global source-cache root identity")
		}
		// Projection maps are independent copies. Only this exact owned root's
		// bookkeeping may change while source snapshots are created/removed.
		delete(before, ".")
		delete(after, ".")
	}
	if !reflect.DeepEqual(before, after) {
		return fmt.Errorf("Gemini assessment operation changed unrelated global authority")
	}
	return nil
}
