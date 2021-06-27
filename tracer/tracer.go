package tracer

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eudore/eudore"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/uber/jaeger-client-go"
	jaegerconfig "github.com/uber/jaeger-client-go/config"
	jaegerprometheus "github.com/uber/jaeger-lib/metrics/prometheus"
)

/*
	docker run -d -e ES_JAVA_OPTS="-Xms256m -Xmx256m" -p 9200:9200  elasticsearch:5.6
	docker run -d -e ELASTICSEARCH_URL=http://172.17.0.1:9200 -p 5601:5601 kibana:5.6
	docker run -d -p 6831:6831/udp -p 16686:16686 -e SPAN_STORAGE_TYPE=elasticsearch -e ES_SERVER_URLS=http://172.17.0.1:9200 jaegertracing/all-in-one:latest

	docker run -d -p 6831:6831/udp -p 16686:16686 jaegertracing/all-in-one:latest
*/

var (
	// OpentracingContextHeaderName 定义span保存header名称。
	OpentracingContextHeaderName = "x-trace-span"
	// OpentracingBaggageHeaderPrefix 定义分布式数据保存header前缀。
	OpentracingBaggageHeaderPrefix = "x-context-"
	// OpentracingBaggageHeader 定义分布式数据保存header。
	OpentracingBaggageHeader = "x-baggage"
	// OpentracingDebugHeader 定义debug-id。
	OpentracingDebugHeader = "x-debug-id"
)

// Tracer 定义别名 opentracing.Tracer
type Tracer = opentracing.Tracer

// Config 定义配置。
type Config struct {
	ServiceName string                `json:"servicename" alias:"servicename"`
	Agent       string                `json:"agent" alias:"agent"`
	Logger      eudore.Logger         `json:"-" alias:"-"`
	Registerer  prometheus.Registerer `json:"-" alias:"-"`
}

// NewOpentracing 函数使用配置创建Tracer。
func NewOpentracing(config *Config) (opentracing.Tracer, error) {
	if config.ServiceName == "" {
		return nil, errors.New("Opentracing ServiceName muest no-nil")
	}

	config.Agent = eudore.GetString(config.Agent, "127.0.0.1:6831")
	cfg := jaegerconfig.Configuration{
		ServiceName: config.ServiceName,
		Headers: &jaeger.HeadersConfig{
			TraceContextHeaderName:   OpentracingContextHeaderName,
			TraceBaggageHeaderPrefix: OpentracingBaggageHeaderPrefix,
			JaegerBaggageHeader:      OpentracingBaggageHeader,
			JaegerDebugHeader:        OpentracingDebugHeader,
		},
		Sampler: &jaegerconfig.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegerconfig.ReporterConfig{
			LocalAgentHostPort: config.Agent,
			// LogSpans:           true,
		},
	}

	var options []jaegerconfig.Option
	if config.Logger != nil {
		options = append(options, jaegerconfig.Logger(&traceLogger{config.Logger}))
	}
	if config.Registerer != nil {
		options = append(options, jaegerconfig.Metrics(jaegerprometheus.New(jaegerprometheus.WithRegisterer(config.Registerer))))
	}

	tracer, _, err := cfg.NewTracer(options...)
	if err != nil {
		return nil, err
	}
	//	app.Infof("init opentracing to jaeger agent '%s' success.", app.Config.JaegerAgent)
	return tracer, err
}

// NewOpentracingHandler 函数创建eudore http请求处理中间件函数，创建span相关对象。
func NewOpentracingHandler(tracer opentracing.Tracer) eudore.HandlerFunc {
	return func(ctx eudore.Context) {
		spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(ctx.Request().Header))
		span := tracer.StartSpan(
			eudore.GetString(ctx.GetParam(eudore.ParamAction), "ServeHTTP"),
			opentracing.ChildOf(spanCtx),
			opentracing.StartTime(time.Now()),
			opentracing.Tags{
				"span.kind":       "server",
				"http.user-agent": ctx.GetHeader("User-Agent"),
				"http.method":     ctx.Method(),
				"http.url":        ctx.Request().RequestURI,
				"http.readip":     ctx.RealIP(),
			},
		)
		traceID := fmt.Sprint(span.Context())
		pos := strings.IndexByte(traceID, ':')
		if pos != -1 {
			ctx.SetHeader(eudore.HeaderXTraceID, traceID[:pos])
		}
		ctx.WithContext(opentracing.ContextWithSpan(ctx.GetContext(), span))
		ctx.SetLogger(ctx.Logger().WithField("context", ctx.GetContext()).WithField("x-trace-id", traceID[:pos]).WithFields(nil, nil))

		ctx.Next()
		span.SetTag("http.status", ctx.Response().Status())
		span.SetTag("http.request-id", ctx.RequestID())
		span.SetTag("http.route", ctx.GetParam("route"))
		span.Finish()
	}
}
