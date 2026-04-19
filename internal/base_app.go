package core

import (
	"context"
	"database/sql"
	"fmt"
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
)

var _ App = (*BaseApp)(nil)

type BaseApp struct {
	config *config.Config
	logger *slog.Logger
	data   *datamanager.DataManager
	driver *driver.Driver

	mu *sync.Mutex

	ctx        context.Context
	cancelFunc context.CancelFunc

	// hooks
	onBeforeApplicationBootstrapped *hooks.Hook[BeforeApplicationBootstrapped]
	onApplicationStop               *hooks.Hook[CloseEvent]
	onAfterApplicationBootstrapped  *hooks.Hook[AfterApplicationBootstrapped]
	onAfterServiceExecuted          *hooks.Hook[AfterServiceExecuted]
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

	return slog.New(handler)
}

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
		return
	}

	b.cancelFunc()
}

func (b *BaseApp) Driver() *driver.Driver {
	if b.driver != nil {
		return b.driver
	}

	b.mu.Lock()
	b.driver = driver.NewDriver()
	b.mu.Unlock()

	return b.driver
}

func (b *BaseApp) Shutdown() error {
	if err := b.OnApplicationStop().Add("close_connection_db", func(e CloseEvent) error {
		if e.App.Data() != nil {
			e.App.Data().Close("sql", func(err error) {
				e.App.Logger().Error(err.Error())
			})
		}
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

// Start The Application by blocking the main
func (b *BaseApp) Bootstrap() error {
	b.OnBeforeApplicationBootstrapped().Execute(BeforeApplicationBootstrapped{App: b})

	b.OnAfterApplicationBootstrapped().Add("a_bootstrap", func(e AfterApplicationBootstrapped) error {
		var cfg *config.Config
		if e.ConfigPath == "" {
			cfg = config.NewDefaultConfig()
		} else {
			cfg = config.NewConfig(config.ConfigParam{
				ConfigName: b.getFileName(e.ConfigPath),
				ConfigType: "toml",
				Path:       b.getDirectoryPath(e.ConfigPath),
			})
		}

		b.config = cfg
		b.logger = b.initLogger(cfg)

		// after logger initiate
		if e.ConfigPath == "" {
			e.App.Logger().Info("Load default config.toml")
		} else {
			e.App.Logger().Info(fmt.Sprintf("Load file config %s", b.getFileName(e.ConfigPath)))
		}

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
