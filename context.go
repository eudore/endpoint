package endpoint

import (
	"context"
	"io"
	"net/http"

	"github.com/eudore/endpoint/gorm"
	"github.com/eudore/eudore"
	"github.com/opentracing/opentracing-go"
)

// Context 定义endpoint请求上下文。
type Context struct {
	eudore.Context
	App *App
}

// NewExtendContext 函数定义endpoint请求上下文闭包函数。
func NewExtendContext(app *App) func(func(*Context)) eudore.HandlerFunc {
	return func(fn func(*Context)) eudore.HandlerFunc {
		return func(ctx eudore.Context) {
			fn(&Context{
				Context: ctx,
				App:     app,
			})
		}
	}
}

// NewRequest 方法创建http请求。
func (ctx *Context) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		ctx.Error(err)
		return nil, err
	}
	return req.WithContext(ctx.GetContext()), nil
}

// NewSpan 方法创建opentracing Span。
func (ctx *Context) NewSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	span := opentracing.SpanFromContext(ctx.GetContext())
	return span.Tracer().StartSpan(operationName, append(opts, opentracing.ChildOf(span.Context()))...)
}

// WithDB 方法返回请求上下文的Database。
func (ctx *Context) WithDB() *gorm.Database {
	db := ctx.App.Database.WithContext(context.WithValue(ctx.GetContext(), gorm.ContextItemGormLogger, ctx.Logger()))
	return db
}
