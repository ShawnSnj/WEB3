package resume

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"code.sajari.com/docconv"
)

// ExtractText pulls plain text from resume file bytes (.txt, .md, .doc, .docx, .pdf, .rtf).
func ExtractText(filename, contentType string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("file is empty")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	ct := strings.ToLower(strings.TrimSpace(contentType))

	// Plain text — also accept UTF-8-ish binary pasted as .txt
	if isPlainText(ext, ct) {
		return cleanText(string(data)), nil
	}

	// If someone renamed .docx to .doc or vice versa, sniff ZIP (docx) header
	if ext == ".doc" && looksLikeDocx(data) {
		ext = ".docx"
	}

	mime := docconvMime(ext, ct)
	if mime == "" {
		return "", fmt.Errorf("unsupported file type %q — use .doc, .docx, .pdf, .txt, or paste text", ext)
	}

	resp, err := docconv.Convert(bytes.NewReader(data), mime, true)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w (try Save As .docx or paste text)", ext, err)
	}
	text := cleanText(resp.Body)
	if text == "" {
		return "", fmt.Errorf("no text found in %s — try a different format or paste text", filename)
	}
	return text, nil
}

func isPlainText(ext, contentType string) bool {
	switch ext {
	case ".txt", ".md", ".markdown", "":
		return true
	}
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	return false
}

func looksLikeDocx(data []byte) bool {
	return len(data) >= 2 && data[0] == 'P' && data[1] == 'K'
}

func docconvMime(ext, contentType string) string {
	switch ext {
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	case ".pdf":
		return "application/pdf"
	case ".rtf":
		return "application/rtf"
	case ".odt":
		return "application/vnd.oasis.opendocument.text"
	}
	switch {
	case strings.Contains(contentType, "wordprocessingml"):
		return contentType
	case strings.Contains(contentType, "msword"):
		return "application/msword"
	case strings.Contains(contentType, "pdf"):
		return "application/pdf"
	case strings.Contains(contentType, "rtf"):
		return "application/rtf"
	}
	return ""
}

func cleanText(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, " ")
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// Collapse excessive blank lines
	lines := strings.Split(s, "\n")
	var out []string
	blank := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			blank++
			if blank > 2 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// ReadFile reads up to maxBytes from r.
func ReadFile(r io.Reader, maxBytes int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxBytes))
}
