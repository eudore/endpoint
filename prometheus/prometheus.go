package prometheus

import (
	"strconv"
	"time"

	"github.com/eudore/eudore"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

var (
	// PrometheusInflightName 定义当前请求的监控项名称
	PrometheusInflightName = "prometheus_http_requests_in_flight"
	// PrometheusInflightHelp 定义当前请求的监控项名称
	PrometheusInflightHelp = "Current number of scrapes being served."
	// PrometheusCountName 定义总请求的监控项名称
	PrometheusCountName = "prometheus_http_requests_total"
	// PrometheusCountHelp 定义总请求的监控项名称
	PrometheusCountHelp = "Total number of scrapes by HTTP status code."
	// PrometheusDurationName 定义响应状态码的监控项名称
	PrometheusDurationName = "prometheus_http_request_duration_seconds"
	// PrometheusDurationHelp 定义响应状态码的监控项名称
	PrometheusDurationHelp = "Histogram of latencies for HTTP requests."
	// PrometheusResponseSizeName 定义响应body大小的监控项名称
	PrometheusResponseSizeName = "prometheus_http_response_size_bytes"
	// PrometheusResponseSizeHelp 定义响应body大小的监控项名称
	PrometheusResponseSizeHelp = "Histogram of response size for HTTP requests."
)

// Prometheus 定义prometheus使用的对象。
type Prometheus interface {
	prometheus.Registerer
	prometheus.Gatherer
}

// NewPrometheus 函数初始化prometheus。
func NewPrometheus() Prometheus {
	prom := prometheus.NewRegistry()
	prom.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}), prometheus.NewGoCollector())
	return prom
}

// NewPrometheusHandler 函数创建一个prometheus http请求记录函数。
func NewPrometheusHandler(name string, reg prometheus.Registerer) eudore.HandlerFunc {
	service := prometheus.Labels{"service": name}
	labels := []string{"code", "method", "path", "handler"}
	httpInflight := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: PrometheusInflightName, Help: PrometheusInflightHelp, ConstLabels: service},
		[]string{"method", "path", "handler"},
	)
	httpCount := prometheus.NewCounterVec(prometheus.CounterOpts{Name: PrometheusCountName, Help: PrometheusCountHelp, ConstLabels: service}, labels)
	httpDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: PrometheusDurationName, Help: PrometheusDurationHelp, ConstLabels: service}, labels)
	httpResponseSize := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: PrometheusResponseSizeName, Help: PrometheusResponseSizeHelp, ConstLabels: service}, labels)
	reg.MustRegister(httpCount, httpInflight, httpDuration, httpResponseSize)

	return func(ctx eudore.Context) {
		labels := prometheus.Labels{
			"method":  ctx.Method(),
			"path":    ctx.Path(),
			"handler": ctx.GetParam(eudore.ParamRoute),
		}
		inflight := httpInflight.With(labels)
		inflight.Inc()
		defer inflight.Dec()
		now := time.Now()
		ctx.Next()
		labels["code"] = strconv.Itoa(ctx.Response().Status())
		httpCount.With(labels).Inc()
		httpDuration.With(labels).Observe(time.Since(now).Seconds())
		httpResponseSize.With(labels).Observe(float64(ctx.Response().Size()))
	}
}

// NewPrometheusMetrics 函数创建一个prometheus metrics响应处理函数。
func NewPrometheusMetrics(gatherer prometheus.Gatherer) eudore.HandlerFunc {
	return func(ctx eudore.Context) {
		mfs, err := gatherer.Gather()
		if err != nil {
			ctx.Error("error gathering metrics:", err)
			return
		}

		contentType := expfmt.Negotiate(ctx.Request().Header)
		ctx.SetHeader(eudore.HeaderContentType, string(contentType))
		enc := expfmt.NewEncoder(ctx, contentType)

		for _, mf := range mfs {
			err := enc.Encode(mf)
			if err != nil {
				ctx.Error("error encoding and sending metric family:", err)
				return
			}
		}

		if closer, ok := enc.(expfmt.Closer); ok {
			err := closer.Close()
			if err != nil {
				ctx.Error("error encoding and sending metric family:", err)
				return
			}
		}
	}
}
