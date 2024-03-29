package buildsys

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rotisserie/eris"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

type (
	runtimeCtxKey struct{}
	runtimeCtx    struct {
		runTasks    map[string]bool
		projectRoot string
	}
)

func getRuntimeCtx(ctx context.Context) *runtimeCtx {
	runCtx, ok := ctx.Value(runtimeCtxKey{}).(*runtimeCtx)
	if !ok {
		panic("found wrong type in runtime context")
	}

	return runCtx
}

func getTaskEnv(task *Task) expand.Environ {
	return expand.ListEnviron(getEnvVars(task.Env)...)
}

var defaultExecHandler = interp.DefaultExecHandler(2)

func execHandler(ctx context.Context, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "cp", "mv", "rm", "mkdir", "touch", "sleep":
			// always use our cross-platform implementation for these operations to make sure
			// they behave consistently
			args = append([]string{"tool"}, args...)
		}
	}

	return defaultExecHandler(ctx, args)
}

var defaultOpenHandler = interp.DefaultOpenHandler()

func openHandler(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	if path == "/dev/null" {
		path = os.DevNull
	}

	return defaultOpenHandler(ctx, path, flag, perm)
}

func resolvePatternLists(ctx context.Context, base string, patterns []string) ([]string, error) {
	result := []string{}
	cfg := expand.Config{
		ReadDir:  shellReadDir,
		GlobStar: true,
	}

	parser := syntax.NewParser()
	parserCtx := &parserCtx{
		filepath:    "invalid",
		projectRoot: getRuntimeCtx(ctx).projectRoot,
	}

	for _, item := range patterns {
		item = normalizePath(parserCtx, base, item)
		item = filepath.ToSlash(item)

		words := make([]*syntax.Word, 0)
		err := parser.Words(strings.NewReader(item), func(w *syntax.Word) bool {
			words = append(words, w)
			return true
		})
		if err != nil {
			return nil, eris.Wrapf(err, "failed to parse pattern %s", item)
		}

		matches, err := expand.Fields(&cfg, words...)
		if err != nil {
			return nil, eris.Wrapf(err, "Failed to resolve pattern %s", item)
		}

		for _, match := range matches {
			// If a pattern didn't match anything, it's returned as a result. Skip those results.
			if !strings.Contains(match, "*") {
				result = append(result, match)
			}
		}
	}
	return result, nil
}

// RunTask executes the given task
func RunTask(ctx context.Context, projectRoot, task string, tasks TaskList, dryRun, force bool) error {
	rctx := runtimeCtx{
		projectRoot: projectRoot,
		runTasks:    make(map[string]bool),
	}

	ctx = context.WithValue(ctx, runtimeCtxKey{}, &rctx)
	taskMeta, found := tasks[task]
	if !found {
		return eris.Errorf("Task %s not found", task)
	}

	return runTaskInternal(ctx, taskMeta, tasks, dryRun, force, true)
}

func runTaskInternal(ctx context.Context, task *Task, tasks TaskList, dryRun, force, canSkip bool) error {
	if ctx.Err() != nil {
		return eris.Wrap(ctx.Err(), "context failed")
	}

	rctx := getRuntimeCtx(ctx)
	status, ok := rctx.runTasks[task.Short]
	if ok {
		if status {
			// this task has already been run
			log(ctx).Debug().Msgf("Task %s already run", task.Short)
			return nil
		}

		if !status {
			return eris.Errorf("Task %s was called recursively", task.Short)
		}
	}

	rctx.runTasks[task.Short] = false

	for _, dep := range task.Deps {
		if !rctx.runTasks[dep] {
			depTask, ok := tasks[dep]
			if !ok {
				return eris.Errorf("Task %s not found", dep)
			}

			err := runTaskInternal(ctx, depTask, tasks, dryRun, false, true)
			if err != nil {
				return eris.Wrapf(err, "Task %s failed due to its dependency %s", task.Short, dep)
			}
		}
	}

	if canSkip && !force {
		skipList, err := resolvePatternLists(ctx, task.Base, task.SkipIfExists)
		if err != nil {
			return eris.Wrapf(err, "failed to resolve skipIfExists list")
		}

		found := 0
		for _, item := range skipList {
			_, err := os.Stat(item)
			if err == nil {
				found++
			} else if !eris.Is(err, os.ErrNotExist) {
				return eris.Wrapf(err, "Failed to check %s", item)
			}
		}

		if found > 0 && found == len(skipList) {
			log(ctx).Info().
				Str("task", task.Short).
				Msg("skipped because all skip files exist")

			rctx.runTasks[task.Short] = true
			return nil
		}
	}

	if !force {
		var newestInput time.Time
		var newestInName string
		inputList, err := resolvePatternLists(ctx, task.Base, task.Inputs)
		if err != nil {
			return eris.Wrap(err, "failed to resolve inputs")
		}

		outputList, err := resolvePatternLists(ctx, task.Base, task.Outputs)
		if err != nil {
			return eris.Wrap(err, "failed to resolve output list")
		}

		for _, item := range inputList {
			info, err := os.Stat(item)
			if err != nil {
				return eris.Wrapf(err, "Failed to check input %s", item)
			}

			if info.ModTime().Sub(newestInput) > 0 {
				newestInput = info.ModTime()
				newestInName = item
			}
		}

		missing := false
		if !newestInput.IsZero() {
			var newestOutput time.Time
			var newestOutName string
			oldestOutput := time.Now()

			for _, item := range outputList {
				info, err := os.Stat(item)
				if err != nil && !eris.Is(err, os.ErrNotExist) {
					return eris.Wrapf(err, "Failed to check output %s", item)
				}

				if eris.Is(err, os.ErrNotExist) {
					missing = true
				}

				if err == nil {
					mt := info.ModTime()
					if mt.Sub(newestOutput) > 0 {
						newestOutput = mt
						newestOutName = item
					}

					if oldestOutput.Sub(mt) > 0 {
						oldestOutput = mt
					}
				}
			}

			if !missing {
				if newestOutput.Sub(oldestOutput) > 10*time.Minute {
					log(ctx).Warn().
						Str("task", task.Short).
						Msgf("oldest output is %f minutes older than the newest output", newestOutput.Sub(oldestOutput).Minutes())
				}

				if newestOutput.Sub(newestInput) >= 0 {
					log(ctx).Info().
						Str("task", task.Short).
						Msgf("nothing to do (output is %f seconds newer)", newestOutput.Sub(newestInput).Seconds())

					rctx.runTasks[task.Short] = true
					return nil
				}

				log(ctx).Info().
					Str("task", task.Short).
					Msgf("rebuild necessary since %s is newer than %s", newestInName, newestOutName)
			}
		}
	}

	// Default to running with 'set -e'
	params := ""
	if !task.IgnoreExit {
		params += "-e"
	}

	// With the skip and input/output checks done, we can finally start executing
	runner, err := interp.New(
		interp.Dir(task.Base),
		interp.Env(getTaskEnv(task)),
		interp.ExecHandler(execHandler),
		interp.OpenHandler(openHandler),
		interp.StdIO(nil, os.Stdout, os.Stderr),
		interp.Params(params),
	)
	if err != nil {
		return eris.Wrap(err, "Failed to initialize runner")
	}

	parser := syntax.NewParser()
	printer := syntax.NewPrinter(
		syntax.Minify(true),
	)
	strBuffer := strings.Builder{}

	for _, item := range task.Cmds {
		stmts, err := item.ToShellStmts(parser)
		if err != nil {
			return eris.Wrap(err, "failed to parse shell script")
		}
		if stmts != nil {
			for _, stm := range stmts {
				strBuffer.Reset()
				printer.Print(&strBuffer, stm)
				log(ctx).Info().
					Str("task", task.Short).
					Bool("command", true).
					Msg(strBuffer.String())

				if !dryRun {
					err = runner.Run(ctx, stm)
					if err != nil {
						_, exit := interp.IsExitStatus(err)
						if !exit || (exit && !task.IgnoreExit) {
							// try resetting the console but ignore any errors since the previous error is
							// more important
							_ = resetConsole()
							return eris.Wrapf(err, "failed to run script for %s", task.Short)
						}
					}

					// reset the console in case a process we ran in this step changed the console mode
					err = resetConsole()
					if err != nil && os.Getenv("CI") != "true" {
						return eris.Wrap(err, "failed to reset console")
					}

					if runner.Exited() {
						return nil
					}
				}
			}
		} else {
			subTask, err := item.ToTask()
			if err != nil {
				return eris.Wrap(err, "failed to retrieve task ref")
			}

			if subTask != nil {
				err = runTaskInternal(ctx, subTask, tasks, dryRun, force, true)
				if err != nil {
					return err
				}
			} else {
				return eris.Errorf("unexpected task command %+v", item)
			}
		}

		if err = ctx.Err(); err != nil {
			return eris.Wrap(err, "context failed")
		}
	}

	if task.Short != "" {
		rctx.runTasks[task.Short] = true
	}

	outputList, err := resolvePatternLists(ctx, task.Base, task.Outputs)
	if err != nil {
		return eris.Wrap(err, "failed to resolve output list")
	}

	// Touch all output files once to make sure that we don't rerun this step even if the step didn't actually modify
	// its outputs.
	now := time.Now()
	for _, path := range outputList {
		err := os.Chtimes(path, now, now)
		if err != nil && !eris.Is(err, os.ErrNotExist) {
			return eris.Wrapf(err, "failed to stamp output %s", path)
		}
	}
	return nil
}
