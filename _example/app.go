package main

import (
	"time"

	"github.com/eudore/endpoint"
	"github.com/eudore/eudore"
	"github.com/eudore/eudore/middleware"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
)

func main() {
	app := endpoint.NewApp("eudore-endpoint", eudore.Renderer(eudore.RenderJSON))

	// 设置数据库类型为postgres
	app.Config.Gorm.Type = "postgres"
	app.Config.Gorm.Dialector = postgres.Open
	app.Config.Gorm.LoggerLevel = eudore.LogDebug

	err := app.Parse()
	if err != nil {
		app.Options(err)
		return
	}
	app.GetFunc("/health", eudore.HandlerEmpty)
	app.GetFunc("/metrics", app.NewPrometheusMetrics())
	app.AddMiddleware(
		app.NewOpentracingHandler(),
		app.NewPrometheusHandler(),
		middleware.NewRequestIDFunc(func(eudore.Context) string { return uuid.New().String() }),
		middleware.NewLoggerFunc(app.App, "route", "action", "Policy", "Resource", "Userid"),
		middleware.NewGzipFunc(5),
		middleware.NewRecoverFunc(),
		app.NewPolicysHandler(),
	)

	{
		// 初始化权限数据
		app.Policys.AddPolicy(&endpoint.Policy{
			PolicyID:  1,
			Statement: []byte(`[{"effect":true, "data": [
				{"kind":"value", "name":"id", "value": ["value:param:id"]},
				{"kind":"range", "name":"id", "min":1, "max":3 }
			]}]`),
		})
		app.Policys.AddMember(&endpoint.Member{UserID: 0, PolicyID: 1})
	}

	app.AddController(app.NewPolicysController())
	app.AddController(app.NewGormController(new(User)))
	app.AddController(app.NewGormController(new(Group)))
	app.AnyFunc("/sp action=test:Jaeger:Span", handlerSpan)
	app.AnyFunc("/sp/child action=test:Jaeger:SpanChild", eudore.HandlerEmpty)
	app.AnyFunc("/data action=test:Policy:Data", handlerData)
	app.Listen(":8089")
	app.Run()
}

func handlerSpan(ctx *endpoint.Context) {
	req, err := ctx.NewRequest("GET", "http://127.0.0.1:8089/sp/child", nil)
	if err == nil {
		ctx.App.HTTP.Do(req)
	}

	req, err = ctx.NewRequest("GET", "http://eudore-endpoint/sp/child", nil)
	if err == nil {
		ctx.App.HTTP.Do(req)
	}

	user := &User{}
	ctx.WithDB().Find(user)

	ctx.Info("trace info 1")
	ctx.WithField("x-h", 6666).Info("trace info 2")
	ctx.WithField("key", "hello").Info("trace info 3")
	ctx.WriteString("hello sp")
}

func handlerData(ctx *endpoint.Context) {
	user := &User{}
	ctx.WithDB().Joins("JOIN groups on users.group_id=groups.id").Find(user)

	ctx.WithDB().Where("id=?", 1).Where("id=?", 2).Find(user)
	ctx.WriteString("hello policy data")
}

type User struct {
	ID        int       `json:"id"`
	GroupID   int       `json:"group_id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Group struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
