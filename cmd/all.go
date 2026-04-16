package cmd

import (
	"log/slog"
	"os"

	datamanagerfx "goodin/drivers/datamanager_fx"
	httpdriver "goodin/drivers/http"
	nats_messaging "goodin/drivers/messaging/nats"
	"goodin/drivers/monitoring"
	watermillfx "goodin/drivers/watermill_fx"
	core "goodin/internal"

	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func NewAllCommand(app core.App) *cobra.Command {
	var configPath string

	command := &cobra.Command{
		Use:   "all",
		Short: "Start Application (Echo HTTP + Watermill bus, FX-wired)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// App lifecycle hook — fires before config/DB init; must run before FX starts.
			app.OnAfterApplicationBootstrapped().Execute(core.AfterApplicationBootstrapped{
				App:        app,
				ConfigPath: configPath,
			})

			// Telemetry runs outside FX (non-FX driver lifecycle, unchanged).
			if app.Config().Monitoring.EnableTelemetry {
				if err := app.Driver().RunDriver(monitoring.NewOtel(app)); err != nil {
					os.Exit(1)
				}
			} else {
				app.Logger().Info("Telemetry is disabled")
			}

			// NATS driver must start before FX so domains that need request-reply
			// (e.g. Loyalty → Orders ACL) can retrieve the infra from app.Driver().
			if err := app.Driver().RunDriver(nats_messaging.NewNatsMessaging(app, true)); err != nil {
				app.Logger().Error("Cannot run driver NATS", slog.Any("err", err.Error()))
			}

			fx.New(
				fx.WithLogger(func(logger *slog.Logger) fxevent.Logger {
					return &fxevent.SlogLogger{Logger: app.Logger().With("component", "uber/fx")}
				}),

				// Core singletons bridged from the existing App
				fx.Provide(
					func() *slog.Logger { return app.Logger() },
					func() core.App { return app },
				),

				// DataManager bridge — exposes named DB connections into FX
				fx.Provide(
					datamanagerfx.NewDataManager,
					datamanagerfx.ProvideGorm("gorm"),   // *gorm.DB  name:"gorm"
					datamanagerfx.ProvideSQL("default"), // *sql.DB   name:"default"
				),
				watermillfx.Module,
				httpdriver.Module,
				fx.Invoke(func(*echo.Echo) {}),
			).Run()

			return nil
		},
	}

	command.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file location")
	return command
}
