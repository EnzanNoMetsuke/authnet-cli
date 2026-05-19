package cli

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var ansiSequencePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestRenderHumanTablePadsCellsHorizontally(t *testing.T) {
	output := renderHumanTable(
		[]string{"A", "B"},
		[][]string{{"one", "two"}},
		false,
	)

	lines := strings.Split(output, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected rendered table rows, got:\n%s", output)
	}
	assertContains(t, lines[1], " A ")
	assertContains(t, lines[1], " B ")
	assertContains(t, lines[3], " one ")
	assertContains(t, lines[3], " two ")
}

func TestRenderHumanTableAlternatesBodyRowsWhenColorEnabled(t *testing.T) {
	output := renderHumanTable(
		[]string{"A"},
		[][]string{{"one"}, {"two"}, {"three"}},
		true,
	)

	assertContains(t, output, "\x1b[38;5;250mtwo")
	assertNotContains(t, output, "\x1b[38;5;250mone")
	assertNotContains(t, output, "\x1b[38;5;250mthree")

	plainOutput := renderHumanTable(
		[]string{"A"},
		[][]string{{"one"}, {"two"}, {"three"}},
		false,
	)
	assertNotContains(t, plainOutput, "\x1b[38;5;250m")
}

func TestWriteOutputJSONHighlightsSyntaxWhenColorEnabled(t *testing.T) {
	value := map[string]any{
		"bool":   true,
		"items":  []string{"one"},
		"number": 12.34,
		"string": "value",
	}

	var output bytes.Buffer
	if err := writeOutputJSON(&output, value, true); err != nil {
		t.Fatalf("expected colored JSON output to succeed: %v", err)
	}

	colored := output.String()
	assertContains(t, colored, "\x1b[")
	assertNotContains(t, colored, "\x1b[36m{")

	ansiSequences := uniqueANSISequences(colored)
	if len(ansiSequences) < 2 {
		t.Fatalf("expected multiple ANSI styles, got %d in:\n%q", len(ansiSequences), colored)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stripANSI(colored)), &decoded); err != nil {
		t.Fatalf("expected highlighted output to remain valid JSON after stripping ANSI: %v\n%s", err, colored)
	}
	if decoded["string"] != "value" {
		t.Fatalf("expected JSON content to round-trip, got %#v", decoded["string"])
	}
}

func TestWriteOutputJSONLeavesPlainJSONUncolored(t *testing.T) {
	var output bytes.Buffer
	if err := writeOutputJSON(&output, map[string]string{"string": "value"}, false); err != nil {
		t.Fatalf("expected plain JSON output to succeed: %v", err)
	}

	plain := output.String()
	assertNotContains(t, plain, "\x1b[")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(plain), &decoded); err != nil {
		t.Fatalf("expected plain output to be valid JSON: %v\n%s", err, plain)
	}
	if decoded["string"] != "value" {
		t.Fatalf("expected JSON content to round-trip, got %#v", decoded["string"])
	}
}

func stripANSI(text string) string {
	return ansiSequencePattern.ReplaceAllString(text, "")
}

func uniqueANSISequences(text string) map[string]struct{} {
	sequences := map[string]struct{}{}
	for _, sequence := range ansiSequencePattern.FindAllString(text, -1) {
		sequences[sequence] = struct{}{}
	}
	return sequences
}
