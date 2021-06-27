/*
Package endpoint 是eudore集成gorm、opentracing、prometheus等第三方库扩展，可以参考endpoint发挥eudore的扩展能力，自定义一套体系。
*/
package endpoint

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/eudore/endpoint/gorm"
	"github.com/eudore/endpoint/prometheus"
	"github.com/eudore/endpoint/tracer"
	"github.com/eudore/eudore"
	"github.com/eudore/eudore/policy"
)

// ApplicationServiceVersion 定义app的版本描述，可以编译时设置。
var ApplicationServiceVersion = "0.0.0"

// App 定义endpoint app 组合全部新组件。
type App struct {
	*Config
	*eudore.App
	Database   *gorm.Database
	Policys    *policy.Policys
	Tracer     tracer.Tracer
	Prometheus prometheus.Prometheus
	HTTP       *http.Client
}

//Config 定义endpoint全部组件配置。
type Config struct {
	ServiceName    string                 `json:"servicename" alias:"servicename"`
	ServicePort    int                    `json:"serviceport" alias:"serviceport"`
	ServiceVersion string                 `json:"serviceversion" alias:"serviceversion"`
	Config         string                 `json:"config" alias:"config"`
	Logger         eudore.LoggerStdConfig `json:"logger" alias:"logger"`
	Gorm           gorm.Config            `json:"gorm" alias:"gorm"`
	// Prometheus     prometheus.Config `json:"prometheus" alias:"prometheus"`
	Tracer tracer.Config `json:"tracing" alias:"tracing"`
}

// NewApp 函数创建新的endpoint App。
func NewApp(name string, options ...interface{}) *App {
	rand.Seed(time.Now().UnixNano())
	config := &Config{
		ServiceName:    name,
		ServicePort:    rand.Intn(3000) + 30000,
		ServiceVersion: ApplicationServiceVersion,
	}
	app := &App{
		Config: config,
		App: eudore.NewApp(
			eudore.NewConfigEudore(config),
			eudore.NewLoggerInit(),
			//			tracer.NewOpentracingLogger(),
		),
		HTTP: tracer.NewOpentracingHTTPClient(http.DefaultClient),
	}
	app.Options(options...)

	// 定义配置解析方法
	app.ParseOption([]eudore.ConfigParseFunc{
		eudore.ConfigParseArgs,
		eudore.ConfigParseEnvs,
		eudore.ConfigParseJSON,
		eudore.ConfigParseArgs,
		eudore.ConfigParseEnvs,
		eudore.ConfigParseMods,
		eudore.ConfigParseWorkdir,
		eudore.ConfigParseHelp,
		app.NewParseLoggerFunc(),
		app.NewParseGormFunc(),
		app.NewParsePolicysFunc(),
		app.NewParsePrometheusFunc(),
		app.NewParseTracingFunc(),
	})
	app.AddHandlerExtend(NewExtendContext(app))
	return app
}

// NewParseLoggerFunc 方法创建一个日志配置解析函数。
func (app *App) NewParseLoggerFunc() eudore.ConfigParseFunc {
	return func(eudore.Config) error {
		log := tracer.NewOpentracingLoggerStdData(eudore.NewLoggerStdDataJSON(&app.Config.Logger))
		app.Options(eudore.NewLoggerStd(log))
		return nil
	}
}

// NewParseGormFunc 方法创建一个Gorm配置解析函数。
func (app *App) NewParseGormFunc() eudore.ConfigParseFunc {
	return func(eudore.Config) error {
		config := &app.Config.Gorm
		config.Logger = app
		db, err := gorm.NewGorm(config)
		if err != nil {
			return err
		}
		app.Database = db
		app.Info(config.Success)
		return nil
	}
}

// GormController 定义别名 gorm.GormController
type GormController = gorm.GormController

// NewGormController 方法创建一个Gorm控制器，处理单model请求。
func (app *App) NewGormController(model interface{}) *gorm.GormController {
	return gorm.NewGormController(app.Database, model)
}

// NewParsePolicysFunc 方法创建一个Policys配置解析函数。
func (app *App) NewParsePolicysFunc() eudore.ConfigParseFunc {
	return func(eudore.Config) error {
		app.Policys = policy.NewPolicys()
		return nil
	}
}

// Policy 定义别名 policy.Policy
type Policy = policy.Policy

// Member 定义别名 policy.Member
type Member = policy.Member

// NewPolicysHandler 方法创建Policys默认鉴权处理中间件函数。
func (app *App) NewPolicysHandler() eudore.HandlerFunc {
	return app.Policys.HandleHTTP
}

// NewPolicysController 方法创建Policys控制器。
func (app *App) NewPolicysController() eudore.Controller {
	db, err := app.Database.DB()
	if err != nil {
		app.Error(err)
		return nil
	}
	return app.Policys.NewPolicysController(app.Config.Gorm.Type, db)
}

// NewParsePrometheusFunc 方法创建一个Prometheus配置解析函数。
func (app *App) NewParsePrometheusFunc() eudore.ConfigParseFunc {
	return func(eudore.Config) error {
		app.Prometheus = prometheus.NewPrometheus()
		return nil
	}
}

// NewPrometheusHandler 方法创建prometheus处理中间件函数。
func (app *App) NewPrometheusHandler() eudore.HandlerFunc {
	return prometheus.NewPrometheusHandler(app.ServiceName, app.Prometheus)
}

// NewPrometheusMetrics 方法创建prometheus /metrics处理函数。
func (app *App) NewPrometheusMetrics() eudore.HandlerFunc {
	return prometheus.NewPrometheusMetrics(app.Prometheus)
}

// NewParseTracingFunc 方法创建Traing配置解析函数。
func (app *App) NewParseTracingFunc() eudore.ConfigParseFunc {
	return func(eudore.Config) error {
		config := &app.Config.Tracer
		config.ServiceName = app.ServiceName
		config.Logger = app
		config.Registerer = app.Prometheus
		trace, err := tracer.NewOpentracing(config)
		if err != nil {
			return err
		}
		app.Tracer = trace
		app.Infof("init opentraceing to jaeger agent %s", config.Agent)
		return nil
	}
}

// NewOpentracingHandler 方法创建opentracing处理中间件函数。
func (app *App) NewOpentracingHandler() eudore.HandlerFunc {
	return tracer.NewOpentracingHandler(app.Tracer)
}

// Run 方法启动endpoint App。
func (app *App) Run() error {
	app.Listen(fmt.Sprintf(":%d", app.ServicePort))
	return app.App.Run()
}
