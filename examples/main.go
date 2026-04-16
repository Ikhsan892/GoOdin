package main

import (
	service "goodin"
	core "goodin/internal"
)

func main() {
	template := service.New(service.BaseConfig{
		DisableBanner: true,
	})

	ctx := template.App.Context()

	template.ShutdownSignal()

	template.App.OnBeforeApplicationBootstrapped().Add("anything", func(e core.BeforeApplicationBootstrapped) error {
		// you can add event on before application bootstrapped here
		return nil
	})

	template.App.OnAfterApplicationBootstrapped().Add("anything", func(e core.AfterApplicationBootstrapped) error {
		// you can add event on before application bootstrapped here
		return nil
	})

	template.Start()

	if !template.RootCmd.IsAvailableCommand() {
		template.App.Logger().Warn("Command not found")
	}

	err := template.RootCmd.ExecuteContext(ctx)

	select {
	case <-template.App.Context().Done():
		template.App.Logger().Info("Application killed by user (Interrupt)")
	default:
		if err != nil {
			template.App.Logger().Error("Application finished with error", "err", err)
		} else {
			template.App.Logger().Info("Application finished successfully")
		}
	}

	template.Shutdown()
}
