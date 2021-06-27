package gorm

import (
	"context"
	"fmt"
	"time"

	"github.com/eudore/eudore"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	gormlogger "gorm.io/gorm/logger"
	gormutils "gorm.io/gorm/utils"
)

type contextKey struct {
	name string
}

// ContextItemGormLogger 定义gorm使用的eudore.Logger的context key。
var ContextItemGormLogger = &contextKey{"logger"}

type gormLogger struct {
	Logger        eudore.Logger
	LogLevel      eudore.LoggerLevel
	SlowThreshold time.Duration
}

// NewGromLogger 函数适配eudore.Logger实现gormlogger接口。
func NewGromLogger(logger eudore.Logger, level eudore.LoggerLevel, slow time.Duration) gormlogger.Interface {
	return &gormLogger{
		Logger:        logger,
		LogLevel:      level,
		SlowThreshold: slow,
	}
}

func (l gormLogger) getLogger(ctx context.Context) eudore.Logger {
	log, ok := ctx.Value("logger").(eudore.Logger)
	if ok {
		return log
	}
	return l.Logger
}

var levelMapping = map[gormlogger.LogLevel]eudore.LoggerLevel{
	gormlogger.Silent: eudore.LogFatal,
	gormlogger.Error:  eudore.LogError,
	gormlogger.Warn:   eudore.LogWarning,
	gormlogger.Info:   eudore.LogInfo,
}

func (l *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	el, ok := levelMapping[level]
	if !ok {
		el = eudore.LogDebug
	}
	newlogger := *l
	newlogger.LogLevel = el
	return &newlogger
}

func (l gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= eudore.LogInfo {
		l.getLogger(ctx).Infof(msg, data...)
	}
}

func (l gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= eudore.LogWarning {
		l.getLogger(ctx).Warningf(msg, data...)
	}
}

func (l gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= eudore.LogError {
		l.getLogger(ctx).Errorf(msg, data...)
	}
}

func (l gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	if l.LogLevel < eudore.LogFatal {
		elapsed := time.Since(begin)
		log := l.getLogger(ctx).WithFields([]string{"sqltime", "sql", "file"},
			[]interface{}{fmt.Sprintf("%.3fms", float64(elapsed.Nanoseconds())/1e6), sql, gormutils.FileWithLineNum()})
		if rows != -1 {
			log.WithField("rows", rows)
		}
		switch {
		case err != nil && l.LogLevel >= eudore.LogError:
			log.Error(err.Error())
		case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= eudore.LogWarning:
			log.Warningf("SLOW SQL >= %v", l.SlowThreshold)
		case l.LogLevel <= eudore.LogInfo:
			log.Info()
		}
	}

	spanParent := opentracing.SpanFromContext(ctx)
	if spanParent != nil {
		span := spanParent.Tracer().StartSpan(
			"gorm",
			opentracing.StartTime(begin),
			opentracing.ChildOf(spanParent.Context()),
			opentracing.Tags{"span.kind": "client"},
		)
		span.LogFields(log.Int64("rows", rows))
		span.LogFields(log.String("sql", sql))
		if err != nil {
			span.LogFields(log.String("error", err.Error()))
		}
		span.Finish()
	}
}
