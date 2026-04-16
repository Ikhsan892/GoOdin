// Package datamanagerfx bridges the existing DataManager (multi-connection SQL layer)
// into the Uber FX dependency injection container.
//
// The DataManager itself is unchanged — it still owns all connection lifecycle.
// This package only extracts named connections out of it as typed FX values so
// that domain repositories and services can receive e.g. *gorm.DB via constructor
// injection instead of calling app.Data().Get("sql","gorm").(*gorm.DB) manually.
//
// Usage in cmd/fx.go or cmd/watermill.go:
//
//	fx.Provide(
//	    datamanagerfx.NewDataManager,          // provides *datamanager.DataManager
//	    datamanagerfx.ProvideGorm("gorm"),     // provides *gorm.DB  named "gorm"
//	    datamanagerfx.ProvideGorm("analytics"),// provides *gorm.DB  named "analytics"
//	    datamanagerfx.ProvideSQL("default"),   // provides *sql.DB   named "default"
//	)
//
// Injecting in a repository:
//
//	type Params struct {
//	    fx.In
//	    DB *gorm.DB `name:"gorm"`
//	}
package datamanagerfx

import (
	"database/sql"
	"fmt"

	core "goodin/internal"
	"goodin/pkg/datamanager"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

// NewDataManager extracts the already-initialised DataManager from the App.
// app.Bootstrap() must have been called before FX starts (cmd commands do this
// via app.OnAfterApplicationBootstrapped).
func NewDataManager(app core.App) *datamanager.DataManager {
	return app.Data()
}

// ProvideGorm returns an FX-annotated constructor that pulls a *gorm.DB for the
// given connection alias out of the DataManager and registers it under name:"<alias>".
//
// The returned value is passed directly to fx.Provide:
//
//	fx.Provide(datamanagerfx.ProvideGorm("gorm"))
func ProvideGorm(alias string) any {
	return fx.Annotate(
		func(dm *datamanager.DataManager) *gorm.DB {
			conn := dm.Get("sql", alias)
			db, ok := conn.(*gorm.DB)
			if !ok {
				panic(fmt.Sprintf("datamanagerfx: connection alias %q is not a *gorm.DB", alias))
			}
			return db
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, alias)),
	)
}

// ProvideSQL returns an FX-annotated constructor that pulls a *sql.DB for the
// given connection alias out of the DataManager and registers it under name:"<alias>".
//
//	fx.Provide(datamanagerfx.ProvideSQL("default"))
func ProvideSQL(alias string) any {
	return fx.Annotate(
		func(dm *datamanager.DataManager) *sql.DB {
			conn := dm.Get("sql", alias)
			db, ok := conn.(*sql.DB)
			if !ok {
				panic(fmt.Sprintf("datamanagerfx: connection alias %q is not a *sql.DB", alias))
			}
			return db
		},
		fx.ResultTags(fmt.Sprintf(`name:"%s"`, alias)),
	)
}
