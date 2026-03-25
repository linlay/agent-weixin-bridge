package diag

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

type level int32

const (
	levelDebug level = iota
	levelInfo
	levelWarn
	levelError
)

var currentLevel atomic.Int32

func init() {
	currentLevel.Store(int32(levelInfo))
}

func Configure(raw string) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		currentLevel.Store(int32(levelDebug))
	default:
		currentLevel.Store(int32(levelInfo))
	}
}

func Debug(event string, kv ...any) {
	logAt(levelDebug, "DEBUG", event, kv...)
}

func Info(event string, kv ...any) {
	logAt(levelInfo, "INFO", event, kv...)
}

func Warn(event string, kv ...any) {
	logAt(levelWarn, "WARN", event, kv...)
}

func Error(event string, kv ...any) {
	logAt(levelError, "ERROR", event, kv...)
}

func RedactSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if utf8.RuneCountInString(value) <= 8 {
		return "***"
	}
	runes := []rune(value)
	return fmt.Sprintf("%s...%s(len=%d)", string(runes[:4]), string(runes[len(runes)-4:]), len(runes))
}

func PreviewText(value string, maxRunes int) string {
	value = compactWhitespace(value)
	if maxRunes <= 0 {
		maxRunes = 80
	}
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes]) + "..."
}

func DurationMillis(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func logAt(msgLevel level, label, event string, kv ...any) {
	if msgLevel < level(currentLevel.Load()) {
		return
	}
	var parts []string
	parts = append(parts, "level="+label, "event="+event)
	for i := 0; i < len(kv); i += 2 {
		key := fmt.Sprintf("arg_%d", i)
		if name, ok := kv[i].(string); ok && strings.TrimSpace(name) != "" {
			key = name
		}
		var value any = ""
		if i+1 < len(kv) {
			value = kv[i+1]
		}
		parts = append(parts, key+"="+formatValue(value))
	}
	log.Print(strings.Join(parts, " "))
}

func formatValue(value any) string {
	switch v := value.(type) {
	case nil:
		return `""`
	case string:
		return strconv.Quote(compactWhitespace(v))
	case error:
		return strconv.Quote(compactWhitespace(v.Error()))
	case time.Time:
		if v.IsZero() {
			return `""`
		}
		return strconv.Quote(v.Format(time.RFC3339Nano))
	case fmt.Stringer:
		return strconv.Quote(compactWhitespace(v.String()))
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return strconv.Quote(compactWhitespace(fmt.Sprint(value)))
	}
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
