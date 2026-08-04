package generator

import (
	"fmt"
	"strings"
)

// countFormatSpecifiers counts the number of format specifiers in a format string.
// It correctly handles %% (escaped percent) which is not a format specifier.
func countFormatSpecifiers(format string) int {
	count := 0
	i := 0
	for i < len(format) {
		if format[i] == '%' {
			if i+1 < len(format) {
				next := format[i+1]
				if next == '%' {
					// %% is escaped percent, skip both
					i += 2
					continue
				}
				// Any other %X is a format specifier
				count++
				i += 2
			} else {
				// Trailing %, count it as specifier (will cause runtime error anyway)
				count++
				i++
			}
		} else {
			i++
		}
	}
	return count
}

// ErrorSnippetBuilder generates consistent error handling code snippets.
type ErrorSnippetBuilder struct {
	serviceID   string
	context     []string
	contextVars []any
	multiline   bool
	customCheck string // For non-standard conditions like nil checks
}

// NewErrorSnippet creates a new error snippet builder for a service.
func NewErrorSnippet(serviceID string) *ErrorSnippetBuilder {
	return &ErrorSnippetBuilder{
		serviceID:   serviceID,
		context:     []string{},
		contextVars: []any{},
		multiline:   false,
	}
}

// WithContext adds a context component to the error message.
func (b *ErrorSnippetBuilder) WithContext(format string, args ...any) *ErrorSnippetBuilder {
	b.context = append(b.context, format)
	b.contextVars = append(b.contextVars, args...)
	return b
}

// Multiline sets whether to use multiline formatting.
func (b *ErrorSnippetBuilder) Multiline() *ErrorSnippetBuilder {
	b.multiline = true
	return b
}

// CustomCondition sets a custom condition instead of "err != nil".
func (b *ErrorSnippetBuilder) CustomCondition(condition string) *ErrorSnippetBuilder {
	b.customCheck = condition
	return b
}

// Build generates the error handling code snippet.
func (b *ErrorSnippetBuilder) Build() string {
	// Build the error message format
	msgParts := []string{"service %q"}
	msgArgs := []any{b.serviceID}

	varIndex := 0
	for _, ctx := range b.context {
		msgParts = append(msgParts, ctx)
		// Calculate how many args this format string needs
		// Use proper counting that handles %% escape sequences
		argCount := countFormatSpecifiers(ctx)
		for range argCount {
			if varIndex < len(b.contextVars) {
				msgArgs = append(msgArgs, b.contextVars[varIndex])
				varIndex++
			}
		}
	}

	// Determine the condition
	condition := "err != nil"
	wrappedErr := true
	if b.customCheck != "" {
		condition = b.customCheck
		wrappedErr = false
	}

	// Build the error format string
	errorMsg := strings.Join(msgParts, " ")
	if wrappedErr {
		errorMsg += ": %w"
	}

	// Build the fmt.Errorf call
	fmtArgs := make([]string, 0, len(msgArgs)+1)
	for _, arg := range msgArgs {
		// %q renders ints as rune literals ('\x00'), so format by type.
		switch v := arg.(type) {
		case string:
			fmtArgs = append(fmtArgs, fmt.Sprintf("%q", v))
		default:
			fmtArgs = append(fmtArgs, fmt.Sprintf("%v", v))
		}
	}
	if wrappedErr {
		fmtArgs = append(fmtArgs, "err")
	}

	errCall := fmt.Sprintf("fmt.Errorf(%q, %s)", errorMsg, strings.Join(fmtArgs, ", "))

	// Format the final snippet
	if b.multiline {
		return fmt.Sprintf("if %s {\n\t\treturn zero, %s\n\t}", condition, errCall)
	}
	return fmt.Sprintf("if %s { return zero, %s }", condition, errCall)
}

// Convenience functions for common error patterns. Each takes the import
// manager because the emitted snippets rely on the fmt package.

func serviceConstructorError(im *ImportManager, serviceID string) string {
	im.Require("fmt")
	return NewErrorSnippet(serviceID).
		WithContext("constructor").
		Multiline().
		Build()
}

func serviceArgError(im *ImportManager, serviceID string, argIndex int) string {
	im.Require("fmt")
	return NewErrorSnippet(serviceID).
		WithContext("arg[%d]", argIndex).
		Build()
}

func serviceArgEntryError(im *ImportManager, serviceID string, argIndex int, key string) string {
	im.Require("fmt")
	return NewErrorSnippet(serviceID).
		WithContext("arg[%d]", argIndex).
		WithContext("key %s", key).
		Build()
}

func serviceParamError(im *ImportManager, serviceID string, argIndex int, paramName string) string {
	im.Require("fmt")
	return NewErrorSnippet(serviceID).
		WithContext("arg[%d]", argIndex).
		WithContext("param %q", paramName).
		Build()
}

func serviceReceiverError(im *ImportManager, serviceID, receiverID string) string {
	im.Require("fmt")
	return NewErrorSnippet(serviceID).
		WithContext("receiver %q", receiverID).
		Build()
}

// serviceArgErrorIndented returns an indented error snippet for use in nested blocks
func serviceArgErrorIndented(im *ImportManager, serviceID string, argIndex int) string {
	im.Require("fmt")
	return fmt.Sprintf("\tif err != nil { return zero, fmt.Errorf(\"service %%q arg[%%d]: %%w\", %q, %d, err) }",
		serviceID, argIndex)
}
