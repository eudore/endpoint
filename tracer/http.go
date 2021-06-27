package tracer

import (
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

// NewOpentracingHTTPClient 函数设置http.Client的Transport支持opentracing日志写入。
func NewOpentracingHTTPClient(client *http.Client) *http.Client {
	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}
	client.Transport = httpTrace{client.Transport}
	return client
}

type httpTrace struct {
	next http.RoundTripper
}

func (trace httpTrace) RoundTripper() http.RoundTripper {
	return trace.next
}

func (trace httpTrace) RoundTrip(req *http.Request) (*http.Response, error) {
	spanParent := opentracing.SpanFromContext(req.Context())
	if spanParent == nil {
		return trace.next.RoundTrip(req)
	}
	span := spanParent.Tracer().StartSpan(
		"http.client",
		opentracing.ChildOf(spanParent.Context()),
		opentracing.Tags{
			"span.kind":   "client",
			"http.method": req.Method,
			"http.scheme": req.URL.Scheme,
			"http.host":   req.URL.Host,
			"http.path":   req.URL.Path,
		},
	)
	defer span.Finish()

	if req.URL.RawQuery != "" {
		span.SetTag("http.row", req.URL.RawQuery)
	}
	serviceId := req.Header.Get("X-Service-Id")
	if serviceId != "" {
		span.SetTag("http.service", serviceId)
	}
	spanParent.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))

	resp, err := trace.next.RoundTrip(req)
	if resp != nil {
		span.SetTag("http.status", resp.StatusCode)
	}
	if err != nil {
		span.LogFields(log.String("error", err.Error()))
	}

	return resp, err
}
