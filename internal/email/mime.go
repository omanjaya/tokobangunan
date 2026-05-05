package email

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// buildMIME bangun raw RFC822 multipart message.
// Struktur: multipart/mixed -> [multipart/alternative {text, html}, attachments...].
func buildMIME(from string, msg Message) ([]byte, error) {
	mixedBoundary := randomBoundary("mx")
	altBoundary := randomBoundary("alt")

	var buf bytes.Buffer
	// Headers.
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", encodeHeader(msg.Subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n", mixedBoundary)
	fmt.Fprintf(&buf, "\r\n")

	// Bagian alternative (text + html).
	fmt.Fprintf(&buf, "--%s\r\n", mixedBoundary)
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", altBoundary)

	textBody := msg.BodyText
	if strings.TrimSpace(textBody) == "" {
		textBody = stripHTML(msg.BodyHTML)
	}
	fmt.Fprintf(&buf, "--%s\r\n", altBoundary)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	buf.WriteString(textBody)
	buf.WriteString("\r\n")

	if strings.TrimSpace(msg.BodyHTML) != "" {
		fmt.Fprintf(&buf, "--%s\r\n", altBoundary)
		fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
		buf.WriteString(msg.BodyHTML)
		buf.WriteString("\r\n")
	}
	fmt.Fprintf(&buf, "--%s--\r\n", altBoundary)

	// Attachments.
	for _, a := range msg.Attachments {
		ct := a.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		fmt.Fprintf(&buf, "--%s\r\n", mixedBoundary)
		fmt.Fprintf(&buf, "Content-Type: %s; name=%q\r\n", ct, a.Filename)
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&buf, "Content-Disposition: attachment; filename=%q\r\n\r\n", a.Filename)
		enc := base64.StdEncoding.EncodeToString(a.Content)
		writeWrapped(&buf, enc, 76)
		buf.WriteString("\r\n")
	}
	fmt.Fprintf(&buf, "--%s--\r\n", mixedBoundary)
	return buf.Bytes(), nil
}

func writeWrapped(buf *bytes.Buffer, s string, width int) {
	for i := 0; i < len(s); i += width {
		end := i + width
		if end > len(s) {
			end = len(s)
		}
		buf.WriteString(s[i:end])
		buf.WriteString("\r\n")
	}
}

func encodeHeader(s string) string {
	// Sederhana: kalau ASCII pure, kirim apa adanya. Kalau ada non-ASCII, RFC2047.
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7F {
			return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
		}
	}
	return s
}

// randomBoundary buat boundary unik berbasis nanosecond + prefix.
func randomBoundary(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// stripHTML hapus tag HTML kasar untuk fallback text.
func stripHTML(s string) string {
	var out strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}
