// Package wingsv кодирует и декодирует ссылки wingsv://. Формат общий с
// приложением: base64url от байта формата и сжатого protobuf Config.
package wingsv

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/andybalholm/brotli"
	"google.golang.org/protobuf/proto"

	"github.com/WINGS-N/wingsv-dex/internal/gen/wingsvpb"
)

const (
	SchemePrefix = "wingsv://"
	// formatProtobufDeflate - старый формат, читается ради ссылок, выданных
	// раньше
	formatProtobufDeflate = 0x12
	// formatProtobufBrotli - основной формат: ссылка короче на четверть
	formatProtobufBrotli = 0x13
	// brotliQuality - пятый уровень. Одиннадцатый на этих данных даёт те же
	// байты за время в полсотни раз большее
	brotliQuality = 5
)

// linkPattern matches a wingsv:// link inside arbitrary pasted text, mirroring the
// app's LINK_PATTERN. The body allows both url-safe and standard base64 alphabets.
var (
	linkPattern     = regexp.MustCompile(`wingsv://[A-Za-z0-9_\-+/=]+`)
	whitespaceRegex = regexp.MustCompile(`\s+`)
)

// ExtractLink pulls a wingsv:// link out of arbitrary text: a
// regex scan first, then the whole trimmed string if it already is a link.
func ExtractLink(text string) string {
	text = strings.TrimSpace(text)
	if m := linkPattern.FindString(text); m != "" {
		return m
	}
	if strings.HasPrefix(text, SchemePrefix) {
		return text
	}
	return ""
}

// Decode parses a wingsv:// link (or text containing one) into its Config. It is
// lenient: it extracts the link from surrounding
// text, strips stray slashes and whitespace, fixes base64 padding, and accepts both
// the url-safe and standard base64 alphabets.
func Decode(raw string) (*wingsvpb.Config, error) {
	link := ExtractLink(raw)
	if link == "" {
		return nil, errors.New("no wingsv:// link found")
	}
	decoded, err := decodePayload(link)
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, errors.New("wingsv: empty payload")
	}
	inflated, err := decompress(decoded)
	if err != nil {
		return nil, err
	}
	config := &wingsvpb.Config{}
	if err := proto.Unmarshal(inflated, config); err != nil {
		return nil, fmt.Errorf("wingsv: protobuf: %w", err)
	}
	return config, nil
}

// decodePayload turns the base64 body of a link into bytes's
// decodePayload: trim, drop leading slashes, strip whitespace, repad, then try the
// url-safe alphabet and fall back to standard base64.
func decodePayload(link string) ([]byte, error) {
	payload := strings.TrimSpace(link[len(SchemePrefix):])
	payload = strings.TrimLeft(payload, "/")
	payload = whitespaceRegex.ReplaceAllString(payload, "")
	payload = normalizePadding(payload)
	if b, err := base64.URLEncoding.DecodeString(payload); err == nil {
		return b, nil
	}
	b, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("wingsv: base64: %w", err)
	}
	return b, nil
}

// normalizePadding pads the payload out to a multiple of four with '=', matching
// the app so both padded and stripped links decode.
func normalizePadding(payload string) string {
	if mod := len(payload) % 4; mod != 0 {
		payload += strings.Repeat("=", 4-mod)
	}
	return payload
}

// Encode собирает ссылку в основном формате: protobuf, сжатый brotli, байт
// формата, base64url.
func Encode(config *wingsvpb.Config) (string, error) {
	raw, err := proto.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("wingsv: protobuf: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteByte(formatProtobufBrotli)
	bw := brotli.NewWriterLevel(&buf, brotliQuality)
	if _, err := bw.Write(raw); err != nil {
		return "", fmt.Errorf("wingsv: brotli: %w", err)
	}
	if err := bw.Close(); err != nil {
		return "", fmt.Errorf("wingsv: brotli close: %w", err)
	}
	return SchemePrefix + base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

// EncodeDeflate собирает ссылку старым форматом. Нужен там, где на той стороне
// заведомо старая сборка
func EncodeDeflate(config *wingsvpb.Config) (string, error) {
	raw, err := proto.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("wingsv: protobuf: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteByte(formatProtobufDeflate)
	zw, err := zlib.NewWriterLevel(&buf, flate.BestCompression)
	if err != nil {
		return "", fmt.Errorf("wingsv: zlib: %w", err)
	}
	if _, err := zw.Write(raw); err != nil {
		return "", fmt.Errorf("wingsv: zlib write: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("wingsv: zlib close: %w", err)
	}
	return SchemePrefix + base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

// decompress разбирает кадр: за байтом формата идёт сжатый protobuf
func decompress(decoded []byte) ([]byte, error) {
	switch decoded[0] {
	case formatProtobufBrotli:
		return io.ReadAll(brotli.NewReader(bytes.NewReader(decoded[1:])))
	case formatProtobufDeflate:
		return inflate(decoded[1:])
	default:
		return nil, fmt.Errorf("wingsv: unsupported payload format 0x%02x", decoded[0])
	}
}

// inflate decompresses the payload. The app writes a zlib stream; fall back to a
// raw deflate stream so a truncated or alternately framed link still decodes.
func inflate(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("wingsv: empty compressed payload")
	}
	if zr, err := zlib.NewReader(bytes.NewReader(payload)); err == nil {
		defer zr.Close()
		return io.ReadAll(zr)
	}
	fr := flate.NewReader(bytes.NewReader(payload))
	defer fr.Close()
	out, err := io.ReadAll(fr)
	if err != nil {
		return nil, fmt.Errorf("wingsv: link is corrupt or truncated: %w", err)
	}
	return out, nil
}
