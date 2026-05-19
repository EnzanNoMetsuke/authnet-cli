package cli

import (
	"strings"
	"testing"
)

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
