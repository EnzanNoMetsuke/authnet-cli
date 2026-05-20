package cli

import (
	"os"
	"reflect"
	"regexp"
	"strings"
)

const redactedValue = "[REDACTED]"

var sentinelPattern = regexp.MustCompile(`SENTINEL_[A-Za-z0-9_-]+`)

func sanitizeForOutput(value any) any {
	reflected := sanitizeReflect(reflect.ValueOf(value))
	if !reflected.IsValid() {
		return nil
	}
	return reflected.Interface()
}

func sanitizeReflect(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			return value
		}
		sanitized := sanitizeReflect(value.Elem())
		if value.Kind() == reflect.Interface {
			return sanitized
		}
		pointer := reflect.New(value.Type().Elem())
		pointer.Elem().Set(sanitized)
		return pointer
	case reflect.String:
		return reflect.ValueOf(sanitizeString(value.String()))
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		slice := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			slice.Index(index).Set(sanitizeReflect(value.Index(index)))
		}
		return slice
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			result.SetMapIndex(iter.Key(), sanitizeReflect(iter.Value()))
		}
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		for index := 0; index < value.NumField(); index++ {
			if result.Field(index).CanSet() {
				result.Field(index).Set(sanitizeReflect(value.Field(index)))
			}
		}
		return result
	default:
		return value
	}
}

func sanitizeString(text string) string {
	sanitized := sentinelPattern.ReplaceAllString(text, redactedValue)
	for _, secret := range sensitiveEnvironmentValues() {
		sanitized = strings.ReplaceAll(sanitized, secret, redactedValue)
	}
	return sanitized
}

func sensitiveEnvironmentValues() []string {
	values := []string{}
	seen := map[string]struct{}{}
	for _, item := range os.Environ() {
		name, value, ok := strings.Cut(item, "=")
		if !ok || len(value) < 4 {
			continue
		}
		upperName := strings.ToUpper(name)
		if strings.Contains(upperName, "LOGIN") ||
			strings.Contains(upperName, "KEY") ||
			strings.Contains(upperName, "SECRET") ||
			strings.Contains(upperName, "TOKEN") {
			for _, candidate := range []string{value, strings.TrimSpace(value)} {
				if len(candidate) < 4 {
					continue
				}
				if _, exists := seen[candidate]; exists {
					continue
				}
				seen[candidate] = struct{}{}
				values = append(values, candidate)
			}
		}
	}
	return values
}

func newSafetyDeniedError(message string) error {
	return cliError{
		exitCode: exitSafetyDenied,
		code:     "safety_policy_denied",
		message:  message,
	}
}
