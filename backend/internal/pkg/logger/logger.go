package logger

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 创建 JSON 日志实例。
// 业务上下文统一写入 msg，避免额外业务字段散落在 JSON 外层。
//
// 日志字段规范：
//
//	time         - ISO8601 时间戳
//	level        - 日志级别（小写）
//	msg          - 日志消息
//	caller       - 调用源文件:行号
//	stacktrace   - 仅 error/fatal 级别自动携带
func New(env string, _ string) (*zap.Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	level := zap.InfoLevel
	if env != "prod" {
		level = zap.DebugLevel
	}

	baseCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.Lock(os.Stdout),
		level,
	)
	core := newMessageOnlyCore(baseCore)

	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
	}

	return zap.New(core, opts...), nil
}

type messageOnlyCore struct {
	zapcore.Core
	fields []zap.Field
}

func newMessageOnlyCore(core zapcore.Core) zapcore.Core {
	return &messageOnlyCore{Core: core}
}

func (c *messageOnlyCore) With(fields []zap.Field) zapcore.Core {
	next := *c
	next.fields = append(append([]zap.Field(nil), c.fields...), fields...)
	next.Core = c.Core.With(nil)
	return &next
}

func (c *messageOnlyCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *messageOnlyCore) Write(entry zapcore.Entry, fields []zap.Field) error {
	allFields := make([]zap.Field, 0, len(c.fields)+len(fields))
	allFields = append(allFields, c.fields...)
	allFields = append(allFields, fields...)
	entry.Message = appendLogFieldsToMessage(entry.Message, allFields)
	return c.Core.Write(entry, nil)
}

func appendLogFieldsToMessage(message string, fields []zap.Field) string {
	if len(fields) == 0 {
		return message
	}
	encoded := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(encoded)
	}
	if len(encoded.Fields) == 0 {
		return message
	}

	keys := make([]string, 0, len(encoded.Fields))
	for key := range encoded.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := fmt.Sprint(encoded.Fields[key])
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", normalizeLogFieldName(key), value))
	}
	if len(parts) == 0 {
		return message
	}
	if strings.TrimSpace(message) == "" {
		return strings.Join(parts, " ")
	}
	return message + " " + strings.Join(parts, " ")
}

func normalizeLogFieldName(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "field"
	}
	if index := strings.LastIndexByte(key, '.'); index >= 0 && index < len(key)-1 {
		key = key[index+1:]
	}
	return key
}
