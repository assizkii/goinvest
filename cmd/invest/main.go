package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi"
	grpcmw "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/rs/cors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	gqlapi "goinvest/gen/gql/generated"
	pb "goinvest/gen/proto/go/invest/v1"
	"goinvest/internal/config"
	"goinvest/internal/invest"
	"goinvest/internal/mysql"
	"goinvest/internal/redis"
	"goinvest/internal/services/gqlservice"
	"goinvest/internal/services/investservice"
	"goinvest/internal/services/providerservice"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type Config struct {
	Server struct {
		Host         string        `yaml:"host"`
		Port         string        `yaml:"port"`
		CloseTimeout time.Duration `yaml:"closeTimeout"`
		DebugPort    string        `yaml:"debugPort"`
		GqlPort      string        `yaml:"gqlPort"`
	} `yaml:"server"`
	Logger struct {
		Level string `yaml:"level"`
	} `yaml:"logger"`
	Database  mysql.DBConfig
	Cache     invest.CacheCredentials
	Providers invest.ProvidersConfig `yaml:"providers"`
}

func main() {

	const failed = 1
	logger, atomicLevel, err := newLogger()
	if err != nil {
		fmt.Printf("failed to create logger: %s\n", err)
		os.Exit(failed)
	}

	if err := run(logger, atomicLevel); err != nil {
		logger.Error("invest web server start / shutdown problem", zap.Error(err))
		os.Exit(failed)
	}

}

// run performs the following things:
// 1. Construct all dependencies, such as database, cache pools, external clients
// 2. Wraps them to handy abstractions, such as services and repositories
// 3. Pass dependencies to router and glue everything with http.Host
// 4. Starts http.Host with above mentioned dependencies and manages graceful shutdown.
func run(logger *zap.Logger, atomicLevel zap.AtomicLevel) error {

	conf, err := newConfig(logger)
	if err != nil {
		return fmt.Errorf("config initialization problem: %w", err)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)

	g, ctx := errgroup.WithContext(ctx)

	var (
		grpcServer *grpc.Server
		httpServer *http.Server
	)

	g.Go(func() error {

		defer func() {
			_ = logger.Sync()
		}()

		// DEBUG < INFO < WARN < ERROR < DPanic < PANIC < FATAL
		levels := map[string]zapcore.Level{
			"debug":  zap.DebugLevel,
			"info":   zap.InfoLevel,
			"error":  zap.ErrorLevel,
			"dpanic": zap.DPanicLevel,
			"panic":  zap.PanicLevel,
			"fatal":  zap.FatalLevel,
		}

		atomicLevel.SetLevel(levels[strings.ToLower(conf.Logger.Level)])
		db, closeDB, err := mysql.ConnectLoop(ctx, conf.Database, logger)
		if err != nil {
			return err
		}
		defer func() {
			if err := closeDB(); err != nil {
				logger.Error("problem occurred while closing database connection pool during server shutdown", zap.Error(err))
			}
		}()

		//migrations := conf.Migrations
		//if migrations.Enabled {
		//	goose.SetLogger(zap.NewStdLog(logger.With(zap.String("service", "goose"))))
		//	if err := goose.SetDialect(migrations.Dialect); err != nil {
		//		return fmt.Errorf("goose problem while setting dialect: %w", err)
		//	}
		//	goose.SetTableName(migrations.Table)
		//	goose.SetVerbose(migrations.Verbose)
		//	if err := goose.Up(db, migrations.Directory); err != nil {
		//		return fmt.Errorf("goose migration failed: %w", err)
		//	}
		//}

		cache, closeCache, err := redis.ConnectLoop(ctx, conf.Cache, logger)
		if err != nil {
			return err
		}
		defer func() {
			if err := closeCache(); err != nil {
				logger.Error("problem occurred while closing cache connection pool during server shutdown", zap.Error(err))
			}
		}()

		// let's define storage interfaces which incapsulates database operations
		var (
			mysqlStorage invest.Storage
		)

		mysqlStorage, err = mysql.NewStorage(db)
		if err != nil {
			return err
		}

		// let's define providers
		providerService, err := providerservice.NewProviderService(&conf.Providers, mysqlStorage, cache, logger)
		if err != nil {
			return err
		}

		// let's define some services that contain business-logic here
		investService, err := investservice.NewService(providerService, mysqlStorage, cache, logger)
		if err != nil {
			return err
		}

		router := chi.NewMux()
		router.Use(cors.New(cors.Options{
			AllowedOrigins:   []string{"http://localhost:8080"},
			AllowCredentials: true,
			Debug:            true,
		}).Handler)

		resolver, err := gqlservice.NewResolver(providerService, mysqlStorage, cache, logger)
		if err != nil {
			return err
		}
		gqlServer := handler.New(gqlapi.NewExecutableSchema(gqlapi.Config{Resolvers: resolver}))
		gqlServer.AddTransport(transport.POST{})
		gqlServer.Use(extension.Introspection{})

		// Graphql
		router.Group(func(r chi.Router) {
			r.Method("POST", "/graphql", gqlServer)
		})
		router.Get("/playground", playground.Handler("GraphQL playground", "/graphql"))

		httpServer = &http.Server{
			Addr:              net.JoinHostPort(conf.Server.Host, conf.Server.GqlPort),
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      15 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
			ErrorLog:          zap.NewStdLog(logger.With(zap.String("service", "http"))),
			Handler:           router,
		}
		logger.Info("starting gql server")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("gql server error: %w", err)
		}

		lis, err := net.Listen("tcp", net.JoinHostPort(conf.Server.Host, conf.Server.Port))
		if err != nil {
			return fmt.Errorf("problem while creating grpc listener: %w", err)
		}
		defer func() {
			if err := lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				logger.Error("problem while trying to close listener", zap.Error(err))
			}
		}()

		var opts []grpc.ServerOption
		opts = append(opts, grpcmw.WithUnaryServerChain(
			grpc_recovery.UnaryServerInterceptor(),
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_zap.UnaryServerInterceptor(logger),
			grpc_prometheus.UnaryServerInterceptor,
			investService.ErrorUnaryInterceptor,
			investService.ValidationUnaryInterceptor,
		))

		grpcServer = grpc.NewServer(opts...)
		pb.RegisterInvestServiceServer(grpcServer, investService)
		grpc_prometheus.EnableHandlingTimeHistogram()
		grpc_prometheus.Register(grpcServer)
		reflection.Register(grpcServer)

		logger.Info("starting grpc server", zap.String("port", conf.Server.Port))

		return grpcServer.Serve(lis)

	})

	// =========================================================================
	// Start Debug Service
	//
	// /debug/pprof - Added to the default mux by importing the net/http/pprof package.
	g.Go(func() error {

		router := chi.NewMux()
		//router.Route("/pprof", metrics.PprofRouter)
		//router.Handle("/metrics", metrics.PrometheusHandler())
		//router.HandleFunc("/health/ready", health.ReadinessHandler)
		//router.HandleFunc("/health/live", health.LiveNessHandler)

		httpServer = &http.Server{
			Addr:              net.JoinHostPort(conf.Server.Host, conf.Server.DebugPort),
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      15 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
			ErrorLog:          zap.NewStdLog(logger.With(zap.String("service", "http"))),
			Handler:           router,
		}
		logger.Info("starting http server")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server error: %w", err)
		}

		logger.Info("server shutdown gracefully")
		return nil

	})

	select {
	case <-interrupt:
		break
	case <-ctx.Done():
		break
	}

	logger.Info("shutting down...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if httpServer != nil {
		if err = httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("problem while shutting down http server", zap.Error(err))
		}
	}

	if grpcServer != nil {
		grpcServer.GracefulStop()
	}

	return g.Wait()

}

// newConfig is a constructor-like function which
// returns config object filled from YAML file specified in arguments
// if file was not specified it looks for env variable ${GEO_FACADE_CONFIG}
// if neither argument nor env was specified it tries to look for hardcoded path for conf
// returns error in case of file open error or if config does not comply with invariant.
func newConfig(log *zap.Logger) (*Config, error) {
	const configDir = "config"

	cfg := &Config{}
	err := config.Parse(cfg, config.Options{Dir: configDir, Type: "yaml"}, log)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func newLogger() (*zap.Logger, zap.AtomicLevel, error) {
	conf := zap.NewProductionConfig()
	conf.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	conf.EncoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	atomicLevel := zap.NewAtomicLevelAt(zapcore.DebugLevel)
	conf.Level = atomicLevel
	conf.DisableStacktrace = true
	logger, err := conf.Build()
	if err != nil {
		return nil, zap.AtomicLevel{}, fmt.Errorf("failed to build zap logger: %w", err)
	}
	return logger, atomicLevel, nil
}
