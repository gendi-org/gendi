package srcloc

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Renderer struct {
}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) getFileLines(path string, start, end int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	i := 0
	for scanner.Scan() {
		i++
		if i >= start && i <= end {
			lines = append(lines, scanner.Text())
		}

		if i >= end {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// If we didn't get any lines in the requested range, return an error
	if len(lines) == 0 {
		return nil, fmt.Errorf("no lines found in range %d-%d", start, end)
	}

	return lines, nil
}

func (r *Renderer) RenderLocation(loc *Location, contextLines int) (string, error) {
	if loc == nil || loc.Line < 1 {
		return "", nil
	}

	start := max(loc.Line-contextLines, 1)
	end := loc.Line + contextLines

	lines, err := r.getFileLines(loc.File, start, end)
	if err != nil {
		return "", fmt.Errorf("get file lines: %w", err)
	}

	// Build snippet
	var b strings.Builder
	lineNumWidth := len(fmt.Sprintf("%d", end))

	// Adjust end if we got fewer lines than expected (e.g., at EOF)
	actualEnd := start + len(lines) - 1

	for i := start; i <= actualEnd; i++ {
		lineIndex := i - start // Convert to 0-based index in lines slice
		lineContent := lines[lineIndex]
		if _, err := fmt.Fprintf(&b, "%*d | %s\n", lineNumWidth, i, lineContent); err != nil {
			return "", err
		}

		// Add caret line if this is the error line
		if i == loc.Line {
			// Calculate caret position
			caretPos := max(lineNumWidth+3+loc.Column-1, lineNumWidth+3)

			if _, err := fmt.Fprintf(&b, "%s^\n", strings.Repeat(" ", caretPos)); err != nil {
				return "", err
			}
		}
	}

	return b.String(), nil
}

func (r *Renderer) RenderError(srcErr error, contextLines int) string {
	var locErr *Error
	ok := errors.As(srcErr, &locErr)
	if !ok || locErr.Loc == nil {
		return srcErr.Error()
	}

	snippet, err := r.RenderLocation(locErr.Loc, contextLines)
	if snippet == "" || err != nil {
		return srcErr.Error()
	}

	return fmt.Sprintf("%s\n%s", srcErr.Error(), snippet)
}
