package database

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

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
)

// InitWithCtx ...
func InitWithCtx(pctx *context.Context) {
	ctx = *pctx
}

// GetDBInstance ...
func GetDBInstance() *DBConn {
	once.Do(func() {
		dsn := "user=happy password=Aa123456 dbname=wallte port=5432 sslmode=disable TimeZone=Asia/Taipei"

		sqlDB, err := sql.Open("pgx", dsn)
		if err != nil {
			log.Fatalln(err)
		}

		sqlDB.SetMaxIdleConns(50)
		sqlDB.SetMaxOpenConns(2000)
		sqlDB.SetConnMaxIdleTime(time.Hour)

		gormDB := &gorm.DB{}
		gormDB, err = gorm.Open(postgres.New(postgres.Config{
			Conn: sqlDB,
		}), &gorm.Config{})

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

// CreateFakceWallet ...
func (db *DBConn) CreateFakceWallet(walletsData []modules.Wallet) {
	db.conn.WithContext(ctx).Create(&walletsData)
}

// Conn ...
func (db *DBConn) Conn() *gorm.DB {
	// return db.conn.WithContext(ctx).Debug()
	return db.conn.WithContext(ctx)
}
