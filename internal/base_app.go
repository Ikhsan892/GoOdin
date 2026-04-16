package core

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"goodin/adapters"
	"goodin/pkg/config"
	"goodin/pkg/datamanager"
	"goodin/pkg/driver"
	"goodin/pkg/hooks"
	"goodin/tools/logs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/file"
	slogmulti "github.com/samber/slog-multi"
)

var _ App = (*BaseApp)(nil)

type BaseApp struct {
	config   *config.Config
	logger   *slog.Logger
	data     *datamanager.DataManager
	driver   *driver.Driver
	driverv2 *driverv2.Manager

	mu         *sync.Mutex
	ctx        context.Context
	cancelFunc context.CancelFunc

	ctx        context.Context
	cancelFunc context.CancelFunc

	// hooks
	onBeforeApplicationBootstrapped *hooks.Hook[BeforeApplicationBootstrapped]
	onApplicationStop               *hooks.Hook[CloseEvent]
	onAfterApplicationBootstrapped  *hooks.Hook[AfterApplicationBootstrapped]
	onAfterServiceExecuted          *hooks.Hook[AfterServiceExecuted]
}

// Context implements [App].
func (b *BaseApp) Context() context.Context {
	b.mu.Lock()

	defer b.mu.Unlock()

	if b.ctx != nil {
		return b.ctx
	}

	b.ctx, b.cancelFunc = context.WithCancel(context.Background())

	return b.ctx
}

// Stop implements [App].
func (b *BaseApp) Stop() {
	if b.cancelFunc == nil {
		// nothing to do
		return
	}

	b.cancelFunc()
}

func NewBaseApp() *BaseApp {
	b := &BaseApp{
		mu: &sync.Mutex{},
	}

	b.logger = b.initLogger(nil)

	return b
}

func NewTestApp() *BaseApp {
	return &BaseApp{}
}

func (b *BaseApp) Migration(migrationPath string) *migrate.Migrate {
	connection := b.data.Get("sql", "default")

	driver, err := postgres.WithInstance(connection.(*sql.DB), &postgres.Config{})
	if err != nil {
		log.Fatal(err)
	}

	fSrc, err := (&file.File{}).Open(migrationPath)
	if err != nil {
		log.Fatal(err)
	}

	m, err := migrate.NewWithInstance(
		"file",
		fSrc,
		"postgres",
		driver,
	)
	if err != nil {
		log.Fatal(err)
	}

	return m
}

func (b *BaseApp) initLogger(cfg *config.Config) *slog.Logger {
	minLevel := slog.LevelDebug

	if cfg != nil && cfg.App.Mode == "production" {
		minLevel = slog.LevelInfo
	}

	handler := logs.NewBaseLogHandler(&logs.LogOptions{
		AddSource: true,
		MinLevel:  minLevel,
		Filterrable: func(ctx context.Context, logByte []byte, log *logs.Log) bool {
			// you can change this, maybe to push to monitoring metric
			logs.PrintLog(log, logByte)
			return true
		},
		WriteFunc: func(ctx context.Context, log []*logs.Log) error {
			return nil
		},
	})

	slog.SetDefault(slog.New(
		slogmulti.Fanout(
			// slog.NewJSONHandler(os.Stdout, nil),
			handler,
		)))

	return slog.Default()
}

func (b *BaseApp) DriverV1() *driver.Driver {
	if b.driver != nil {
		return b.driver
	}

	b.mu.Lock()
	b.driver = driver.NewDriver()
	b.mu.Unlock()

	return b.driver
}

func (b *BaseApp) Driver() *driverv2.Manager {
	return b.DriverV2()
}

func (b *BaseApp) DriverV2() *driverv2.Manager {
	if b.driverv2 != nil {
		return b.driverv2
	}

	b.mu.Lock()
	b.driverv2 = driverv2.NewManagerV2()
	b.mu.Unlock()

	return b.driverv2
}

func (b *BaseApp) Shutdown() error {
	if err := b.OnApplicationStop().Add("close_connection_db", func(e CloseEvent) error {
		if e.App.Data() == nil {
			e.App.Logger().Warn("skip closing driver")
			return nil
		}

		e.App.Data().Close("sql", func(err error) {
			e.App.Logger().Error(err.Error())
		})
		return nil
	}); err != nil {
		return err
	}

	b.OnApplicationStop().Add("print_log", func(e CloseEvent) error {
		e.App.Logger().Info("Application Stopped, ciao !")
		return nil
	})

	b.OnApplicationStop().Execute(CloseEvent{App: b})

	return nil
}

func (b *BaseApp) initDatasource() *datamanager.DataManager {
	a := datamanager.New()

	a.AddAdapter("sql", adapters.NewSqlAdapters(b.config, adapters.RegisteredSQLAdapters))
	a.Connect("sql", func(e error) {
		b.logger.Error(e.Error())
	})

	return a
}

func (b *BaseApp) OnBeforeApplicationBootstrapped() *hooks.Hook[BeforeApplicationBootstrapped] {
	b.mu.Lock()

	defer b.mu.Unlock()

	if b.onBeforeApplicationBootstrapped != nil {
		return b.onBeforeApplicationBootstrapped
	}

	b.onBeforeApplicationBootstrapped = hooks.New[BeforeApplicationBootstrapped]()

	return b.onBeforeApplicationBootstrapped
}

func (b *BaseApp) OnAfterApplicationBootstrapped() *hooks.Hook[AfterApplicationBootstrapped] {
	b.mu.Lock()

	defer b.mu.Unlock()

	if b.onAfterApplicationBootstrapped != nil {
		return b.onAfterApplicationBootstrapped
	}

	b.onAfterApplicationBootstrapped = hooks.New[AfterApplicationBootstrapped]()

	return b.onAfterApplicationBootstrapped
}

func (b *BaseApp) OnApplicationStop() *hooks.Hook[CloseEvent] {
	b.mu.Lock()

	defer b.mu.Unlock()

	if b.onApplicationStop != nil {
		return b.onApplicationStop
	}

	b.onApplicationStop = hooks.New[CloseEvent]()

	return b.onApplicationStop
}

// Cache implements App.
func (b *BaseApp) Cache() {
	panic("unimplemented")
}

// Config implements App.
func (b *BaseApp) Config() config.Config {
	return *b.config
}

// Data implements App.
func (b *BaseApp) Data() *datamanager.DataManager {
	return b.data
}

func (b *BaseApp) Sql(connName string) *sql.DB {
	return b.Data().Get("sql", connName).(*sql.DB)
}

// IsDev implements App.
func (b *BaseApp) IsDev() bool {
	return b.config.App.Mode == "development"
}

// Logger implements App.
func (b *BaseApp) Logger() *slog.Logger {
	if b.logger == nil {
		return slog.Default()
	}

	return b.logger
}

// Restart implements App.
func (b *BaseApp) Restart() {
	panic("unimplemented")
}

func (b *BaseApp) getDirectoryPath(filePath string) string {
	return filepath.Dir(filePath)
}

func (b *BaseApp) getFileName(filePath string) string {
	base := filepath.Base(filePath)                     // Get the full file name (e.g., config.toml)
	return strings.TrimSuffix(base, filepath.Ext(base)) // Remove the extension
}

func (b *BaseApp) loadConfig(configPath string) *config.Config {
	var cfg *config.Config
	if configPath == "" {
		cfg = config.NewDefaultConfig()
	} else {
		cfg = config.NewConfig(config.ConfigParam{
			ConfigName: b.getFileName(configPath),
			ConfigType: "toml",
			Path:       b.getDirectoryPath(configPath),
		})
	}

	return cfg
}

// Start The Application by blocking the main
func (b *BaseApp) Bootstrap() error {
	b.OnBeforeApplicationBootstrapped().Execute(BeforeApplicationBootstrapped{App: b})

	b.OnAfterApplicationBootstrapped().Add("a_load_cfg", func(e AfterApplicationBootstrapped) error {
		cfg := b.loadConfig(e.ConfigPath)
		b.config = cfg
		b.logger = b.initLogger(cfg)

		return nil
	})

	b.OnAfterApplicationBootstrapped().Add("b_bootstrap", func(e AfterApplicationBootstrapped) error {
		cfg := b.loadConfig(e.ConfigPath)

		b.config = cfg
		b.logger = b.initLogger(cfg)
		e.App.Logger().Info("Bootstrapping Application...")

		b.data = b.initDatasource()

		e.App.Logger().Info("Application Bootstrapped")

		return nil
	})

	return nil
}

// Storage implements App.
func (b *BaseApp) Storage() {
	panic("unimplemented")
}
