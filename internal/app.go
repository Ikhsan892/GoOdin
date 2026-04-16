package core

import (
	"context"
	"log/slog"

	"goodin/pkg/config"
	"goodin/pkg/datamanager"
	"goodin/pkg/driver"
	"goodin/pkg/hooks"

	"github.com/golang-migrate/migrate/v4"
)

// this is the base interface for application behaviour and
// Clean Architecture
type App interface {
	// Check is Development Environment
	IsDev() bool

	// all the configuration for this application
	Config() config.Config

	Context() context.Context

	Stop()

	// base logger application
	Logger() *slog.Logger

	// database instance for access data a.k.a driver
	// support multi database and multi vendor ( agnostic )
	Data() *datamanager.DataManager

	// hook for application on stopped
	OnApplicationStop() *hooks.Hook[CloseEvent]

	// hook for before application bootstrapped
	OnBeforeApplicationBootstrapped() *hooks.Hook[BeforeApplicationBootstrapped]

	// hook after application Bootstrapped
	OnAfterApplicationBootstrapped() *hooks.Hook[AfterApplicationBootstrapped]

	// bootstrap application
	Bootstrap() error

	// Driver manager
	Driver() *driver.Driver

	// migration database
	Migration(string) *migrate.Migrate

	// Shutdown Application
	Shutdown() error

	// internal built-in cache
	// TODO : Implement this feature
	Cache()

	// filesystem application api
	// TODO : Implement this feature
	Storage()

	// Restart the application
	// TODO : Implement this feature
	Restart()
}
