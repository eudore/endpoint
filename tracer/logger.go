package tracer

import (
	"context"
	"strings"

	"github.com/eudore/eudore"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

type traceLogger struct {
	eudore.Logger
}

func (log traceLogger) Error(msg string) {
	log.Logger.Error(msg)
}

func (log traceLogger) Infof(msg string, args ...interface{}) {
	log.Logger.Infof(msg, args...)
}

type loggerStdDataTrace struct {
	eudore.LoggerStdData
}

// NewOpentracingLogger 函数创建同时写入opentraing的eudore.Logger。
func NewOpentracingLogger() eudore.Logger {
	return eudore.NewLoggerStd(NewOpentracingLoggerStdData(nil))
}

// NewOpentracingLoggerStdData 函数指定eudore.LoggerStdData创建同时写入opentraing的eudore.Logger。
func NewOpentracingLoggerStdData(data eudore.LoggerStdData) eudore.LoggerStdData {
	if data == nil {
		data = eudore.NewLoggerStdDataJSON(nil)
	}
	return loggerStdDataTrace{data}
}

func (data loggerStdDataTrace) GetLogger() *eudore.LoggerStd {
	log := data.LoggerStdData.GetLogger()
	_, ok := log.LoggerStdData.(loggerStdDataTrace)
	if !ok {
		log.LoggerStdData = loggerStdDataTrace{log.LoggerStdData}
	}
	return log
}

func (data loggerStdDataTrace) PutLogger(l *eudore.LoggerStd) {
	for i := len(l.Keys) - 1; -1 < i; i-- {
		if l.Keys[i] == "context" {
			ctx, ok := l.Vals[i].(context.Context)
			if ok {
				l.Keys = append(l.Keys[:i], l.Keys[i+1:]...)
				l.Vals = append(l.Vals[:i], l.Vals[i+1:]...)
				fields := make([]log.Field, 0, len(l.Keys)+1)
				for i := 0; i < len(l.Keys); i++ {
					if !strings.HasPrefix(l.Keys[i], "x-") {
						fields = append(fields, log.Object(l.Keys[i], l.Vals[i]))
					}
				}
				fields = append(fields, log.String("level", l.Level.String()), log.String("message", l.Message))
				// TODO: check nil
				opentracing.SpanFromContext(ctx).LogFields(fields...)
				break
			}
		}
	}
	data.LoggerStdData.PutLogger(l)
}
