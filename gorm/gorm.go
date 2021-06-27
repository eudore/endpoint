package gorm

import (
	"fmt"
	"time"

	"github.com/eudore/eudore"
	"gorm.io/gorm"
)

// Database 定义别名 gorm.DB
type Database = gorm.DB

// Config 定义gorm使用的配置。
type Config struct {
	Dialector     func(string) gorm.Dialector `json:"-" alias:"-"`
	Logger        eudore.Logger               `json:"-" alias:"-"`
	LoggerLevel   eudore.LoggerLevel          `json:"loggerlevel" alias:"loggerlevel"`
	SlowThreshold time.Duration               `json:"slowthreshold" alias:"slowthreshold"`
	MaxIdle       int                         `json:"maxidle" alias:"maxidle"`
	MaxOpen       int                         `json:"maxopen" alias:"maxopen"`
	MaxLifetime   time.Duration               `json:"maxlifetime" alias:"maxlifetime"`
	Type          string                      `json:"type" alias:"type"`
	Host          string                      `json:"host" alias:"host"`
	Port          string                      `json:"port" alias:"port"`
	User          string                      `json:"user" alias:"user"`
	Password      string                      `json:"password" alias:"password"`
	Name          string                      `json:"name" alias:"name"`
	Options       string                      `json:"options" alias:"options"`
	Success       string                      `json:"success" alias:"success"`
}

// NewGorm 函数使用配置创建gorm实例。
func NewGorm(config *Config) (db *gorm.DB, err error) {
	ormconfig := &gorm.Config{
		Logger: NewGromLogger(config.Logger, config.LoggerLevel, config.SlowThreshold),
	}
	config.Type = eudore.GetString(config.Type, "sqlite")
	switch config.Type {
	case "sqlite":
		config.Host = eudore.GetString(config.Host, "sqlite.db")
		config.Success = fmt.Sprintf("init database to postgres sqlite %s", config.Host)
		db, err = gorm.Open(config.Dialector(config.Host), ormconfig)
	case "postgres":
		config.Host = eudore.GetString(config.Host, "127.0.0.1")
		config.Port = eudore.GetString(config.Port, "5432")
		config.User = eudore.GetString(config.User, "postgres")
		config.Name = eudore.GetString(config.Name, "postgres")
		config.Password = eudore.GetString(config.Password, "postgres")
		config.Options = eudore.GetString(config.Options, "sslmode=disable")
		config.Success = fmt.Sprintf("init database to postgres %s:%s/%s", config.Host, config.Port, config.Name)
		db, err = gorm.Open(config.Dialector(fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s %s",
			config.Host, config.Port, config.User, config.Password, config.Name, config.Options)), ormconfig)
	case "mysql":
		config.Host = eudore.GetString(config.Host, "127.0.0.1")
		config.Port = eudore.GetString(config.Port, "3306")
		config.User = eudore.GetString(config.User, "mysql")
		config.Name = eudore.GetString(config.Name, "mysql")
		config.Password = eudore.GetString(config.Password, "mysql")
		config.Options = eudore.GetString(config.Options, "charset=utf8mb4&parseTime=True&loc=Local")
		config.Success = fmt.Sprintf("init database to mysql %s:%s/%s", config.Host, config.Port, config.Name)
		db, err = gorm.Open(config.Dialector(fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
			config.User, config.Password, config.Host, config.Port, config.Name, config.Options)), ormconfig)
	default:
		err = fmt.Errorf("未定义db类型：'%s'", config.Type)
	}
	if err != nil {
		err = fmt.Errorf("endpoint init database error: %s", err.Error())
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(3)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(24 * time.Hour)
	return db, nil
}
