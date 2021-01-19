package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/kyos0109/test-wallet/config"
	"github.com/kyos0109/test-wallet/modules"
)

// DBConn ...
type DBConn struct {
	conn *gorm.DB
}

var (
	ctx      context.Context
	once     sync.Once
	dbClient *DBConn
	dsn      string
)

// InitWithCtx ...
func InitWithCtx(pctx *context.Context, c *config.DatabaseConfig) context.Context {
	dsn = fmt.Sprintf("user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=%s",
		c.UserName, c.Password, c.DBName, c.Port, c.TimeZone)

	ctx = context.WithValue(*pctx, DBConn{}, GetDBInstance())

	return ctx
}

// GetDBInstance ...
func GetDBInstance() *DBConn {
	once.Do(func() {
		sqlDB, err := sql.Open("pgx", dsn)
		if err != nil {
			log.Fatalln(err)
		}

		sqlDB.SetMaxIdleConns(config.DB.SetMaxOpenConns)
		sqlDB.SetMaxOpenConns(config.DB.SetMaxIdleConns)
		sqlDB.SetConnMaxIdleTime(config.DB.SetConnMaxIdleTime)

		gormDB := &gorm.DB{}
		gormDB, err = gorm.Open(postgres.New(postgres.Config{
			Conn: sqlDB,
		}), &gorm.Config{
			SkipDefaultTransaction: true,
			PrepareStmt:            true,
		})

		if err != nil {
			log.Fatalln(err)
		}

		dbClient = &DBConn{gormDB}
	})

	return dbClient
}

// CreateFakceUser ...
func (db *DBConn) CreateFakceUser(usersData []modules.User) {
	db.conn.WithContext(ctx).Create(&usersData)
}

// CreateFakceAgent ...
func (db *DBConn) CreateFakceAgent(agentData []modules.Agent) {
	db.conn.WithContext(ctx).Create(&agentData)
}

// CreateFakceWallet ...
func (db *DBConn) CreateFakceWallet(walletsData []modules.Wallet) {
	db.conn.WithContext(ctx).Create(&walletsData)
}

// Conn ...
func (db *DBConn) Conn() *gorm.DB {
	// return db.conn.WithContext(ctx).Debug()
	return db.conn.WithContext(ctx)
}
