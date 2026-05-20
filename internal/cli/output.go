package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/spf13/cobra"
)

const (
	exitSuccess ExitCode = iota
	exitGeneralFailure
	exitUsageOrConfig
	exitAuthFailure
	exitGatewayFailure
	exitSafetyDenied
	exitNotFound
	exitUnavailable
)

const redactedByDefault = true

// ExitCode is the process status taxonomy used by automation.
type ExitCode int

type cliError struct {
	exitCode ExitCode
	code     string
	message  string
}

func (err cliError) Error() string {
	return err.message
}

type exitingError struct {
	cliError
}

func (err exitingError) Unwrap() error {
	return err.cliError
}

type renderedError struct {
	exitCode ExitCode
	message  string
}

func (err renderedError) Error() string {
	return err.message
}

type warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type structuredError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	SchemaVersion             string            `json:"schema_version"`
	Command                   string            `json:"command"`
	ProfileName               string            `json:"profile_name,omitempty"`
	EnvironmentClassification string            `json:"environment_classification,omitempty"`
	Redacted                  bool              `json:"redacted"`
	Warnings                  []warning         `json:"warnings"`
	Errors                    []structuredError `json:"errors"`
	Data                      any               `json:"data,omitempty"`
}

type commandResult struct {
	Data     any
	Warnings []warning
	Errors   []structuredError
	Human    func(io.Writer) error
	Redacted *bool
}

func renderResult(cmd *cobra.Command, result commandResult) error {
	result = sanitizeCommandResult(result)
	options := optionsFromCommand(cmd)
	if options.JSON {
		return writeOutputJSON(cmd.OutOrStdout(), newEnvelope(cmd, result.Data, result.Warnings, result.Errors, resultRedacted(result)), colorEnabled(options))
	}
	if result.Human == nil {
		return nil
	}
	if err := result.Human(cmd.OutOrStdout()); err != nil {
		return err
	}
	return renderHumanWarnings(cmd.ErrOrStderr(), result.Warnings, colorEnabled(options))
}

func renderHumanWarnings(writer io.Writer, warnings []warning, color bool) error {
	for _, item := range warnings {
		label := "warning"
		if color {
			label = "\x1b[33mwarning\x1b[0m"
		}
		if _, err := fmt.Fprintf(writer, "%s: %s\n", label, item.Message); err != nil {
			return err
		}
	}
	return nil
}

func renderHumanTable(headers []string, rows [][]string, color bool) string {
	if len(rows) == 0 {
		return ""
	}
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	headerStyle := cellStyle
	alternateRowStyle := cellStyle
	borderStyle := lipgloss.NewStyle()
	if color {
		headerStyle = headerStyle.Bold(true).Foreground(lipgloss.Color("39"))
		alternateRowStyle = alternateRowStyle.Foreground(lipgloss.Color("250"))
		borderStyle = borderStyle.Foreground(lipgloss.Color("240"))
	}
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row int, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if color && row%2 == 1 {
				return alternateRowStyle
			}
			return cellStyle
		}).
		String()
}

func writeHumanTable(writer io.Writer, headers []string, rows [][]string, color bool) error {
	rendered := renderHumanTable(headers, rows, color)
	if rendered == "" {
		return nil
	}
	_, err := fmt.Fprintln(writer, rendered)
	return err
}

func newEnvelope(cmd *cobra.Command, data any, warnings []warning, errs []structuredError, redacted bool) envelope {
	options := optionsFromCommand(cmd)
	if warnings == nil {
		warnings = []warning{}
	}
	if errs == nil {
		errs = []structuredError{}
	}
	return envelope{
		SchemaVersion:             buildFromCommand(cmd).SchemaVersion,
		Command:                   cmd.CommandPath(),
		ProfileName:               options.Profile,
		EnvironmentClassification: options.Environment,
		Redacted:                  redacted,
		Warnings:                  warnings,
		Errors:                    errs,
		Data:                      data,
	}
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeOutputJSON(writer io.Writer, value any, color bool) error {
	if !color {
		return writeJSON(writer, value)
	}

	var buffer bytes.Buffer
	if err := writeJSON(&buffer, value); err != nil {
		return err
	}
	return quick.Highlight(writer, buffer.String(), "json", "terminal256", "nordic")
}

func structuredFailure(cmd *cobra.Command, err error) (envelope, ExitCode) {
	var appErr cliError
	if errors.As(err, &appErr) {
		return newEnvelope(cmd, nil, nil, []structuredError{{
			Code:    appErr.code,
			Message: sanitizeString(appErr.message),
		}}, redactedByDefault), appErr.exitCode
	}
	return newEnvelope(cmd, nil, nil, []structuredError{{
		Code:    "general_failure",
		Message: sanitizeString(err.Error()),
	}}, redactedByDefault), exitGeneralFailure
}

func exitCodeForError(err error) ExitCode {
	var rendered renderedError
	if errors.As(err, &rendered) {
		return rendered.exitCode
	}
	var appErr cliError
	if errors.As(err, &appErr) {
		return appErr.exitCode
	}
	return exitGeneralFailure
}

func newUsageError(format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	return cliError{
		exitCode: exitUsageOrConfig,
		code:     "usage_or_config_error",
		message:  message,
	}
}

func newExitingUsageError(format string, args ...any) error {
	appErr := newUsageError(format, args...).(cliError)
	return exitingError{cliError: appErr}
}

func colorEnabled(options *globalOptions) bool {
	return options.Color == "always" && !options.NoColor && !options.Automation
}

func sanitizeCommandResult(result commandResult) commandResult {
	if resultRedacted(result) {
		result.Data = sanitizeForOutput(result.Data)
	}
	if result.Warnings != nil {
		result.Warnings = sanitizeForOutput(result.Warnings).([]warning)
	}
	if result.Errors != nil {
		result.Errors = sanitizeForOutput(result.Errors).([]structuredError)
	}
	return result
}

func resultRedacted(result commandResult) bool {
	if result.Redacted == nil {
		return redactedByDefault
	}
	return *result.Redacted
}

func boolPointer(value bool) *bool {
	return &value
}
