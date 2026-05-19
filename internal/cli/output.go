package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
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
}

func renderResult(cmd *cobra.Command, result commandResult) error {
	result = sanitizeCommandResult(result)
	options := optionsFromCommand(cmd)
	if options.JSON {
		return writeOutputJSON(cmd.OutOrStdout(), newEnvelope(cmd, result.Data, result.Warnings, result.Errors), colorEnabled(options))
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
	headerStyle := lipgloss.NewStyle()
	borderStyle := lipgloss.NewStyle()
	if color {
		headerStyle = headerStyle.Bold(true).Foreground(lipgloss.Color("39"))
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
			return lipgloss.NewStyle()
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

func newEnvelope(cmd *cobra.Command, data any, warnings []warning, errs []structuredError) envelope {
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
		Redacted:                  redactedByDefault,
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
	if _, err := writer.Write([]byte("\x1b[36m")); err != nil {
		return err
	}
	if err := writeJSON(writer, value); err != nil {
		return err
	}
	_, err := writer.Write([]byte("\x1b[0m"))
	return err
}

func structuredFailure(cmd *cobra.Command, err error) (envelope, ExitCode) {
	var appErr cliError
	if errors.As(err, &appErr) {
		return newEnvelope(cmd, nil, nil, []structuredError{{
			Code:    appErr.code,
			Message: sanitizeString(appErr.message),
		}}), appErr.exitCode
	}
	return newEnvelope(cmd, nil, nil, []structuredError{{
		Code:    "general_failure",
		Message: sanitizeString(err.Error()),
	}}), exitGeneralFailure
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

func colorEnabled(options *globalOptions) bool {
	return options.Color == "always" && !options.NoColor && !options.Automation
}

func sanitizeCommandResult(result commandResult) commandResult {
	result.Data = sanitizeForOutput(result.Data)
	if result.Warnings != nil {
		result.Warnings = sanitizeForOutput(result.Warnings).([]warning)
	}
	if result.Errors != nil {
		result.Errors = sanitizeForOutput(result.Errors).([]structuredError)
	}
	return result
}
