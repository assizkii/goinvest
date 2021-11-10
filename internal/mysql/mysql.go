package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// DBConfig contains information sufficient for database connection.
type DBConfig struct {
	Net        string        `yaml:"net"`
	Host       string        `yaml:"host"`
	Port       int           `yaml:"port"`
	DBName     string        `yaml:"dbName"`
	User       string        `yaml:"user"`
	Password   string        `yaml:"password"`
	TimeZone   string        `yaml:"timeZone"`
	PoolConfig PoolConfig    `yaml:"poolConfig"`
	Timeout    time.Duration `yaml:"timeout"` // timeout for trying to connect to the database
}

type PoolConfig struct {
	MaxOpenConnections int           `yaml:"maxOpenConnections"`
	MaxIdleConnections int           `yaml:"maxIdleConnections"`
	MaxLifetime        time.Duration `yaml:"maxLifetime"`
}

// ConnectLoop takes config and specified database credentials as input, returning *sql.DB handle for interactions
// with database.
func ConnectLoop(ctx context.Context, cfg DBConfig, logger *zap.Logger) (db *sql.DB, closeFunc func() error, err error) {

	if logger == nil {
		return nil, nil, errors.New("database: provided logger is nil")
	}

	if len(cfg.TimeZone) == 0 {
		return nil, nil, errors.New("empty database time zone")
	}

	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse config database TimeZone as IANA time zone value: %w", err)
	}

	conf := mysql.NewConfig()
	conf.Net = cfg.Net
	conf.Addr = cfg.Host
	conf.User = cfg.User
	conf.Passwd = cfg.Password
	conf.DBName = cfg.DBName
	conf.ParseTime = true
	conf.Timeout = time.Second * 2
	conf.Loc = loc

	dsn := conf.FormatDSN()

	const driverName = "mysql"
	db, err = createDBPool(ctx, driverName, dsn)
	if nil == err {
		configureDBPool(db, cfg.PoolConfig)
		return db, db.Close, nil
	}

	logger.Error("failed to connect to the database", zap.Error(err))

	if cfg.Timeout == 0 {
		const defaultTimeout = 5
		cfg.Timeout = defaultTimeout
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	timeoutExceeded := time.After(cfg.Timeout)

	for {

		select {

		case <-timeoutExceeded:
			return nil, nil, fmt.Errorf("db connection failed after %s timeout", cfg.Timeout)

		case <-ticker.C:
			db, err := createDBPool(ctx, driverName, dsn)
			if nil == err {
				configureDBPool(db, cfg.PoolConfig)
				return db, db.Close, nil
			}
			logger.Error("failed to connect to the database", zap.Error(err))

		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}

}

func createDBPool(ctx context.Context, driverName string, dsn string) (*sql.DB, error) {

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("problem opens a database specified by its database driver: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("problem while trying to ping database: %w", err)
	}

	return db, nil

}

func configureDBPool(db *sql.DB, config PoolConfig) {
	db.SetMaxOpenConns(config.MaxOpenConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(config.MaxLifetime)
}
