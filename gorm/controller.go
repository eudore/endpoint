package gorm

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/eudore/eudore"
	"github.com/eudore/eudore/policy"
	"gorm.io/gorm"
)

// GormController 定义gorm控制器，可以直接实现单model基础方法。
type GormController struct {
	eudore.ControllerAutoRoute
	ModelType        reflect.Type
	ModelColumnNames []string
	ModelColumnTypes []string
	WithDB           func(ctx eudore.Context) *gorm.DB
}

// NewGormController 函数创建gorm控制器。
func NewGormController(db *gorm.DB, model interface{}) *GormController {
	cols, typs, err := getGormModelColumns(db, model)
	if err != nil {
		return nil
	}
	db.AutoMigrate(model)
	name := getGormTableName(db, model)
	db = db.Model(model)
	return &GormController{
		ModelType:        reflect.Indirect(reflect.ValueOf(model)).Type(),
		ModelColumnNames: cols,
		ModelColumnTypes: typs,
		WithDB: func(ctx eudore.Context) *gorm.DB {
			sql, vals := policy.CreateExpressions(ctx, name, cols, -1)
			return db.Where(sql, vals...).WithContext(context.WithValue(ctx.GetContext(), ContextItemGormLogger, ctx.Logger()))
		},
	}
}

func getGormTableName(db *gorm.DB, model interface{}) string {
	stmt := &gorm.Statement{DB: db}
	stmt.Parse(model)
	return stmt.Schema.Table
}

func getGormModelColumns(db *gorm.DB, model interface{}) ([]string, []string, error) {
	coltypes, err := db.Migrator().ColumnTypes(model)
	if err != nil {
		return nil, nil, err
	}
	cols := make([]string, len(coltypes))
	typs := make([]string, len(coltypes))
	for i, coltype := range coltypes {
		cols[i] = coltype.Name()
		switch coltype.DatabaseTypeName() {
		case "int8":
			typs[i] = "int"
		case "text":
			typs[i] = "string"
		case "timestamptz":
			typs[i] = "time"
		}
	}
	return cols, typs, nil
}

// ControllerGroup 方法返回控制器名称，如果是单model返回model名称，如果组合控制器返回控制器名称。
func (ctl *GormController) ControllerGroup(name string) string {
	if name == "GormController" {
		name = ctl.ModelType.Name()
		buf := make([]rune, 0, len(name)*2)
		for _, i := range name[:] {
			if 64 < i && i < 91 {
				buf = append(buf, '/', i+0x20)
			} else {
				buf = append(buf, i)
			}
		}
		return string(buf)
	}
	if strings.HasSuffix(name, "Controller") || strings.HasSuffix(name, "controller") {
		name = name[:len(name)-10]
	}
	return strings.ToLower(name)
}

// ControllerParam 方法定义控制器参数，生成action参数。
func (ctl *GormController) ControllerParam(pkg, name, method string) string {
	pos := strings.LastIndexByte(pkg, '/') + 1
	if pos != 0 {
		pkg = pkg[pos:]
	}
	if strings.HasSuffix(name, "Controller") {
		name = name[:len(name)-len("Controller")]
	}
	return fmt.Sprintf("action=%s:%s:%s", pkg, name, method)
}

// ControllerRoute 方法返回控制器路由推导修改信息。
func (ctl *GormController) ControllerRoute() map[string]string {
	return map[string]string{
		"Get":  "",
		"Post": "",
	}
}

type gormPaging struct {
	Page   int         `json:"page" alias:"page"`
	Size   int         `json:"size" alias:"size"`
	Order  string      `json:"order" alias:"order"`
	Total  int64       `json:"total" alias:"total"`
	Search string      `json:"search" alias:"search"`
	Data   interface{} `json:"data" alias:"data"`
}

// Get 方法处理get请求，请求参数page、size、order定义页码、数量、排序。
func (ctl *GormController) Get(ctx eudore.Context) (interface{}, error) {
	paging := &gormPaging{Size: 20, Order: "id desc"}
	err := ctx.Bind(paging)
	if err != nil {
		return nil, err
	}

	paging.Data = reflect.New(reflect.SliceOf(ctl.ModelType)).Interface()
	db := ctl.WithDB(ctx)
	if paging.Search != "" {
		cond, conddata := ctl.parseSearchExpression(paging.Search)
		db = db.Where(cond, conddata...)
	}
	err = db.Count(&paging.Total).Error
	if err != nil || paging.Total == 0 {
		return paging, err
	}
	err = db.Limit(paging.Size).Offset(paging.Size * paging.Page).Order(paging.Order).Find(paging.Data).Error
	return paging, err
}

var regs = regexp.MustCompile(`(\S+\'.*\'|\S+)`)
var regc = regexp.MustCompile(`(\w*)(=|>|<|<>|!=|>=|<=|~|!~|:)(.*)`)

func (ctl *GormController) parseSearchExpression(key string) (string, []interface{}) {
	var sql string
	var data []interface{}
	for _, key := range regs.FindAllString(key, -1) {
		exp := regc.FindStringSubmatch(key)
		if len(exp) != 4 {
			if strings.HasSuffix(sql, "AND ") {
				sql = sql[:len(sql)-4] + "OR "
			}
			continue
		}
		if !stringSliceIn(ctl.ModelColumnNames, exp[1]) {
			continue
		}
		if exp[2] == "~" {
			exp[2] = "LIKE"
			exp[3] = "%" + exp[3] + "%"
		} else if exp[2] == "!~" {
			exp[2] = "NOT LIKE"
			exp[3] = "%" + exp[3] + "%"
		} else if exp[2] == ":" && strings.Contains(exp[3], ",") {
			exp[2] = "IN"
		}
		sql += fmt.Sprintf("%s %s ? AND ", exp[1], exp[2])
		if exp[2] == "IN" {
			data = append(data, strings.Split(exp[3], ","))
		} else {
			data = append(data, exp[3])
		}
	}
	if strings.HasSuffix(sql, " AND ") || strings.HasSuffix(sql, " OR ") {
		sql = sql[:len(sql)-4]
	}
	if len(data) != 0 {
		return sql, data
	}
	return ctl.parseSearchLike(key)
}

func (ctl *GormController) parseSearchLike(key string) (string, []interface{}) {
	typ, val := ctl.parseSearchType(key)

	var sqls []string
	var vals []interface{}
	for i, coltyp := range ctl.ModelColumnTypes {
		if typ != coltyp {
			continue
		}
		switch coltyp {
		case "string":
			sqls = append(sqls, fmt.Sprintf("%s LIKE ?", ctl.ModelColumnNames[i]))
			vals = append(vals, val)
		case "int", "bool":
			sqls = append(sqls, fmt.Sprintf("%s=?", ctl.ModelColumnNames[i]))
			vals = append(vals, val)
		case "time":
		}
	}
	return strings.Join(sqls, " OR "), vals
}

func (ctl *GormController) parseSearchType(key string) (string, interface{}) {
	i, err := strconv.ParseInt(key, 10, 64)
	if err == nil {
		return "int", i
	}

	f, err := strconv.ParseFloat(key, 64)
	if err == nil {
		return "float", f
	}

	b, err := strconv.ParseBool(key)
	if err == nil {
		return "bool", b
	}

	return "string", key
}

func stringSliceIn(strs []string, str string) bool {
	for _, i := range strs {
		if i == str {
			return true
		}
	}
	return false
}

// GetById 方法处理获取指定id数据。
func (ctl *GormController) GetById(ctx eudore.Context) (interface{}, error) {
	data := reflect.New(ctl.ModelType).Interface()
	err := ctl.WithDB(ctx).Find(data, "id=?", ctx.GetParam("id")).Error
	return data, err
}

// Post 方法创建新数据。
func (ctl *GormController) Post(ctx eudore.Context) (interface{}, error) {
	data := reflect.New(ctl.ModelType).Interface()
	err := ctx.Bind(data)
	if err != nil {
		return nil, err
	}
	return data, ctl.WithDB(ctx).Save(data).Error
}

// PutById 方法修改指定id数据。
func (ctl *GormController) PutById(ctx eudore.Context) (interface{}, error) {
	data := reflect.New(ctl.ModelType).Interface()
	err := ctx.Bind(data)
	if err != nil {
		return nil, err
	}
	return data, ctl.WithDB(ctx).Where("id=?", ctx.GetParam("id")).Updates(data).Error
}

// DeleteById 方法删除指定id数据。
func (ctl *GormController) DeleteById(ctx eudore.Context) error {
	data := reflect.New(ctl.ModelType).Interface()
	err := ctl.WithDB(ctx).Where("id=?", ctx.GetParam("id")).Delete(data).Error
	return err
}
