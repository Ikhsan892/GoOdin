package coordinator

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

const defaultWaitDuration = 5 * time.Second

type Handler func(ctx context.Context, input map[string]any) (map[string]any, error)

type Runner struct {
	handlers   map[string]Handler
	ctx        *ContextStore
	branchDone map[string]chan error // ref → completion signal for fork branch tasks
}

func NewRunner() *Runner {
	return &Runner{
		handlers:   map[string]Handler{},
		ctx:        NewContextStore(),
		branchDone: map[string]chan error{},
	}
}

func (r *Runner) Register(name string, h Handler) {
	r.handlers[name] = h
}

func (r *Runner) Execute(levels [][]Task, initialInput map[string]any) error {
	r.ctx.Set("workflow", map[string]any{"input": initialInput})

	// Pre-allocate completion channels for every fork branch task
	for _, level := range levels {
		for _, task := range level {
			if task.IsForkBranch {
				r.branchDone[task.Ref] = make(chan error, 1)
			}
		}
	}

	for levelIdx, level := range levels {
		var mainTasks []Task
		var branchTasks []Task

		for _, t := range level {
			if t.IsForkBranch {
				branchTasks = append(branchTasks, t)
			} else {
				mainTasks = append(mainTasks, t)
			}
		}

		// Launch fork branches as fire-and-forget goroutines — signal channel when done
		for _, bt := range branchTasks {
			bt := bt
			go func() {
				r.branchDone[bt.Ref] <- r.runStep(bt)
			}()
		}

		if len(mainTasks) == 0 {
			continue
		}

		if len(mainTasks) == 1 {
			if err := r.runStep(mainTasks[0]); err != nil {
				return err
			}
			continue
		}

		// Multiple main-flow tasks at the same level — run in parallel
		fmt.Printf("⚡ Level %d: %d steps parallel\n", levelIdx, len(mainTasks))
		eg, egCtx := errgroup.WithContext(context.Background())
		for _, step := range mainTasks {
			step := step
			eg.Go(func() error {
				select {
				case <-egCtx.Done():
					return egCtx.Err()
				default:
					return r.runStep(step)
				}
			})
		}
		if err := eg.Wait(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) resolveInput(step Task) (map[string]any, error) {
	if len(step.InputParams) > 0 {
		return r.ctx.ResolveTemplate(step.InputParams)
	}
	return map[string]any{}, nil
}

func (r *Runner) runStep(step Task) error {
	switch step.Type {
	case Fork:
		r.ctx.Set(step.Ref, map[string]any{"forked": true})
		fmt.Printf("   ✓ %s (FORK) gateway — branches firing\n", step.Name)
		return nil
	case Join:
		return r.runJoin(step)
	case Noop:
		r.ctx.Set(step.Ref, map[string]any{})
		fmt.Printf("   ✓ %s (NOOP) skipped\n", step.Name)
		return nil
	case Wait:
		return r.runWait(step)
	case DynamicFork:
		return r.runDynamicFork(step)
	}

	start := time.Now()

	input, err := r.resolveInput(step)
	if err != nil {
		return fmt.Errorf("[%s] resolve input: %w", step.Name, err)
	}

	fmt.Printf("   → %s input: %v\n", step.Name, summarize(input))

	handler, ok := r.handlers[step.Name]
	if !ok {
		return fmt.Errorf("[%s] no handler registered", step.Name)
	}

	output, err := handler(context.Background(), input)
	if err != nil {
		return fmt.Errorf("[%s] failed: %w", step.Name, err)
	}

	r.ctx.Set(step.Ref, output)
	fmt.Printf("   ✓ %s done (%dms) output: %v\n", step.Name, time.Since(start).Milliseconds(), summarize(output))

	return nil
}

// runJoin blocks until every ref in JoinOn has signalled completion via its branch channel.
func (r *Runner) runJoin(step Task) error {
	fmt.Printf("   ⏳ %s (JOIN) waiting for: %v\n", step.Name, step.JoinOn)
	for _, ref := range step.JoinOn {
		ch, ok := r.branchDone[ref]
		if !ok {
			continue
		}
		if err := <-ch; err != nil {
			return fmt.Errorf("[JOIN %s] branch %q failed: %w", step.Name, ref, err)
		}
	}
	r.ctx.Set(step.Ref, map[string]any{"joined": true})
	fmt.Printf("   ✓ %s (JOIN) all branches done\n", step.Name)
	return nil
}

// runWait pauses execution for the duration specified in InputParams["duration"] (e.g. "3s", "500ms").
func (r *Runner) runWait(step Task) error {
	d := defaultWaitDuration
	if raw, ok := step.InputParams["duration"]; ok {
		if s, ok := raw.(string); ok {
			if parsed, err := time.ParseDuration(s); err == nil {
				d = parsed
			}
		}
	}
	fmt.Printf("   ⏳ %s (WAIT) sleeping %s\n", step.Name, d)
	time.Sleep(d)
	r.ctx.Set(step.Ref, map[string]any{"waited": d.String()})
	fmt.Printf("   ✓ %s (WAIT) done\n", step.Name)
	return nil
}

// runDynamicFork fires runtime-expanded branches as goroutines and returns immediately.
// InputParams must contain "tasks": []any, each entry a map with "name", "ref", and optional "input".
// A later JOIN task with those refs in JoinOn is the sync point.
func (r *Runner) runDynamicFork(step Task) error {
	input, err := r.resolveInput(step)
	if err != nil {
		return fmt.Errorf("[%s] resolve input: %w", step.Name, err)
	}

	raw, ok := input["tasks"]
	if !ok {
		return fmt.Errorf("[%s] dynamic_fork: input must contain 'tasks'", step.Name)
	}

	taskDefs, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("[%s] dynamic_fork: 'tasks' must be a list", step.Name)
	}

	for _, td := range taskDefs {
		tdMap, ok := td.(map[string]any)
		if !ok {
			return fmt.Errorf("[%s] dynamic_fork: each task entry must be a map", step.Name)
		}

		taskName, _ := tdMap["name"].(string)
		taskRef, _ := tdMap["ref"].(string)
		if taskName == "" || taskRef == "" {
			return fmt.Errorf("[%s] dynamic_fork: each entry must have 'name' and 'ref'", step.Name)
		}

		taskInput := map[string]any{}
		if params, ok := tdMap["input"].(map[string]any); ok {
			taskInput = params
		}

		// Allocate channel at runtime — JOIN will read from it via JoinOn
		r.branchDone[taskRef] = make(chan error, 1)

		go func() {
			handler, ok := r.handlers[taskName]
			if !ok {
				r.branchDone[taskRef] <- fmt.Errorf("[%s] dynamic_fork: no handler for %q", step.Name, taskName)
				return
			}
			output, err := handler(context.Background(), taskInput)
			if err == nil {
				r.ctx.Set(taskRef, output)
				fmt.Printf("   ✓ %s (dynamic) done\n", taskName)
			}
			r.branchDone[taskRef] <- err
		}()
	}

	fmt.Printf("   ✓ %s (DYNAMIC_FORK) %d branches firing\n", step.Name, len(taskDefs))
	return nil
}

func summarize(m map[string]any) string {
	if m == nil {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	if len(keys) > 4 {
		return fmt.Sprintf("{%d fields}", len(keys))
	}
	return fmt.Sprintf("%v", m)
}
