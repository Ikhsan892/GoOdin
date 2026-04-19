package cmd

import (
	"context"
	"fmt"
	"time"

	core "goodin/internal"
	"goodin/pkg/coordinator"

	"github.com/spf13/cobra"
)

// Order processing workflow:
//
//	validate_order      (FN)
//	  → run_checks      (FORK)  — fires inv/pay/fraud in background, main flow continues
//	  → cooldown        (WAIT 500ms)
//	  → log_step        (NOOP)
//	  → wait_checks     (JOIN)  — blocks until inv + pay + fraud all done
//	  → notify_parties  (DYNAMIC_FORK) — fires nw/nc/nf in background
//	  → wait_notifications (JOIN) — blocks until all 3 notify branches done
//	  → finalize_order  (FN)
func NewGraphCommand(app core.App) *cobra.Command {
	var configPath string

	command := &cobra.Command{
		Use:   "graph",
		Short: "Test Graph — order processing with FORK/JOIN/DYNAMIC_FORK/WAIT/NOOP",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks := []coordinator.Task{
				// FN: validate incoming order
				{
					Name: "validate_order",
					Ref:  "validated",
					Type: coordinator.FN,
					Mode: "sync",
					InputParams: map[string]any{
						"order_id": "{{ .workflow.input.orderId }}",
						"customer": "{{ .workflow.input.customer }}",
					},
				},

				// FORK: fire 3 parallel checks — does NOT block main flow
				{
					Name: "run_checks",
					Ref:  "fork_checks",
					Type: coordinator.Fork,
					ForkTasks: [][]coordinator.Task{
						{
							{
								Name: "check_inventory",
								Ref:  "inv",
								Type: coordinator.FN,
								Mode: "sync",
								InputParams: map[string]any{
									"order_id": "{{ .validated.order_id }}",
								},
							},
						},
						{
							{
								Name: "check_payment",
								Ref:  "pay",
								Type: coordinator.FN,
								Mode: "sync",
								InputParams: map[string]any{
									"order_id": "{{ .validated.order_id }}",
								},
							},
						},
						{
							{
								Name: "check_fraud",
								Ref:  "fraud",
								Type: coordinator.FN,
								Mode: "sync",
								InputParams: map[string]any{
									"order_id": "{{ .validated.order_id }}",
								},
							},
						},
					},
				},

				// WAIT: main flow pauses 500ms (branches still running in background)
				{
					Name: "cooldown",
					Ref:  "wait_main",
					Type: coordinator.Wait,
					InputParams: map[string]any{
						"duration": "500ms",
					},
				},

				// NOOP: placeholder step, no side effect
				{
					Name: "log_step",
					Ref:  "noop_log",
					Type: coordinator.Noop,
				},

				// JOIN: block until all 3 check branches finish
				{
					Name:   "wait_checks",
					Ref:    "join_checks",
					Type:   coordinator.Join,
					JoinOn: []string{"inv", "pay", "fraud"},
				},

				// DYNAMIC_FORK: notify parties — task list built at runtime, fires in background
				// Each entry must have "name" (handler key), "ref" (ctx output key), "input" (map)
				{
					Name: "notify_parties",
					Ref:  "dfork_notify",
					Type: coordinator.DynamicFork,
					InputParams: map[string]any{
						"tasks": []any{
							map[string]any{"name": "notify_warehouse", "ref": "nw", "input": map[string]any{"channel": "warehouse"}},
							map[string]any{"name": "notify_customer", "ref": "nc", "input": map[string]any{"channel": "email"}},
							map[string]any{"name": "notify_finance", "ref": "nf", "input": map[string]any{"channel": "finance"}},
						},
					},
				},

				// JOIN: block until all 3 dynamic notification branches finish
				{
					Name:   "wait_notifications",
					Ref:    "join_notify",
					Type:   coordinator.Join,
					JoinOn: []string{"nw", "nc"},
				},

				// FN: finalize order after everything is done
				{
					Name: "finalize_order",
					Ref:  "finalized",
					Type: coordinator.FN,
					Mode: "sync",
					InputParams: map[string]any{
						"order_id": "{{ .validated.order_id }}",
					},
				},
			}

			dag, err := coordinator.NewDAG().CompileTasks(tasks)
			if err != nil {
				return err
			}

			dag.InspectGraph()

			runner := coordinator.NewRunner()

			runner.Register("validate_order", func(_ context.Context, input map[string]any) (map[string]any, error) {
				fmt.Println("validate_order:", input)
				return map[string]any{"order_id": input["order_id"], "valid": true}, nil
			})

			runner.Register("check_inventory", func(_ context.Context, input map[string]any) (map[string]any, error) {
				fmt.Println("check_inventory:", input)
				return map[string]any{"in_stock": true}, nil
			})

			runner.Register("check_payment", func(_ context.Context, input map[string]any) (map[string]any, error) {
				fmt.Println("check_payment:", input)
				time.Sleep(3 * time.Second)
				return map[string]any{"payment_ok": true}, nil
			})

			runner.Register("check_fraud", func(_ context.Context, input map[string]any) (map[string]any, error) {
				fmt.Println("check_fraud:", input)
				return map[string]any{"fraud_score": 0.02}, nil
			})

			runner.Register("notify_warehouse", func(_ context.Context, input map[string]any) (map[string]any, error) {
				fmt.Println("notify_warehouse:", input)
				return map[string]any{"sent": true}, nil
			})

			runner.Register("notify_customer", func(_ context.Context, input map[string]any) (map[string]any, error) {
				fmt.Println("notify_customer:", input)
				return map[string]any{"sent": true}, nil
			})

			runner.Register("notify_finance", func(_ context.Context, input map[string]any) (map[string]any, error) {
				fmt.Println("notify_finance:", input)
				time.Sleep(10 * time.Second)
				return map[string]any{"sent": true}, nil
			})

			runner.Register("finalize_order", func(_ context.Context, input map[string]any) (map[string]any, error) {
				fmt.Println("finalize_order:", input)
				return map[string]any{"status": "completed"}, nil
			})

			return runner.Execute(dag.TaskLevels, map[string]any{
				"orderId":  "ORD-9901",
				"customer": "Fatihul",
			})
		},
	}

	command.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file location")
	return command
}
