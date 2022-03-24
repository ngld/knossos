package buildsys

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aidarkhanov/nanoid"
	"github.com/rotisserie/eris"
	"go.starlark.net/starlark"
	"mvdan.cc/sh/v3/syntax"
)

type parserCtx struct {
	ctx          context.Context
	globals      starlark.StringDict
	options      map[string]ScriptOption
	optionValues map[string]string
	envOverrides map[string]string
	moduleCache  map[string]starlark.StringDict
	yamlCache    map[string]interface{}
	filepath     string
	projectRoot  string
	tasks        []*Task
	initPhase    bool
}

// * Helpers

func getCtx(thread *starlark.Thread) *parserCtx {
	ctx, ok := thread.Local("parserCtx").(*parserCtx)
	if !ok {
		panic("found wrong type for parser context in starlark thread")
	}

	return ctx
}

type starlarkIterable interface {
	Len() int
	Iterate() starlark.Iterator
}

func starlarkIterable2stringSlice(input starlarkIterable, field string) ([]string, error) {
	if value, ok := input.(*starlark.List); ok && value == nil {
		return []string{}, nil
	}

	result := make([]string, 0, input.Len())
	iter := input.Iterate()

	var item starlark.Value
	for iter.Next(&item) {
		switch value := item.(type) {
		case starlark.String:
			result = append(result, value.GoString())
		default:
			return nil, eris.Errorf("expected all items in %s to be strings but found %s", field, item.Type())
		}
	}
	return result, nil
}

func shellReadDir(path string) ([]os.FileInfo, error) {
	if path == "" {
		path = "."
	}

	// This is just a simple wrapper around ioutil.ReadDir, there's not much of value to add here.
	//nolint:wrapcheck
	return ioutil.ReadDir(path)
}

func processCmdParts(parts starlark.Tuple, parser *syntax.Parser, base string) (*syntax.CallExpr, error) {
	envVars := make([]string, 0, len(parts))
	for _, part := range parts {
		end := false
		switch value := part.(type) {
		case starlark.String:
			if strings.Contains(value.GoString(), "=") {
				envVars = append(envVars, value.GoString())
			} else {
				end = true
			}
		default:
			break
		}

		if end {
			break
		}
	}

	var cmd *syntax.CallExpr
	if len(envVars) > 0 {
		joinedEnvVars := strings.Join(envVars, " ")
		result, err := parser.Parse(strings.NewReader(joinedEnvVars), "env vars")
		if err != nil {
			return nil, eris.Wrapf(err, "failed to parse command vars %s", joinedEnvVars)
		}

		if len(result.Stmts) != 1 || result.Stmts[0].Cmd == nil {
			return nil, eris.Errorf("malformed env vars %s", joinedEnvVars)
		}

		var ok bool
		cmd, ok = result.Stmts[0].Cmd.(*syntax.CallExpr)
		if !ok || cmd.Assigns == nil {
			return nil, eris.Errorf("malformed env vars %s", joinedEnvVars)
		}
	} else {
		cmd = new(syntax.CallExpr)
	}

	argCount := len(parts) - len(envVars)
	cmd.Args = make([]*syntax.Word, 0, argCount)
	for _, arg := range parts[len(envVars):] {
		var encodedValue string

		skip := false
		switch value := arg.(type) {
		case starlark.String:
			encodedValue = value.GoString()
		case StarlarkPath:
			encodedValue = string(value)

			if filepath.IsAbs(encodedValue) {
				// absolute paths cause issues on Windows
				var err error
				relValue, err := filepath.Rel(base, encodedValue)
				if err == nil {
					encodedValue = relValue
				}
			}

			encodedValue = filepath.ToSlash(encodedValue)
		case StarlarkShellArgs:
			for _, arg := range value {
				cmd.Args = append(cmd.Args, arg)
			}

			skip = true
		default:
			return nil, eris.Errorf("found argument of type %s but only strings and paths are supported: %s", arg.Type(), arg.String())
		}

		if skip {
			continue
		}

		var wordPart syntax.WordPart

		if strings.ContainsAny(encodedValue, " $'") {
			node := new(syntax.SglQuoted)
			node.Left = syntax.Pos{}
			node.Right = syntax.Pos{}
			node.Value = encodedValue

			wordPart = syntax.WordPart(node)
		} else {
			node := new(syntax.Lit)
			node.ValuePos = syntax.Pos{}
			node.ValueEnd = syntax.Pos{}
			node.Value = encodedValue

			wordPart = syntax.WordPart(node)
		}

		word := new(syntax.Word)
		word.Parts = []syntax.WordPart{wordPart}
		cmd.Args = append(cmd.Args, word)
	}

	return cmd, nil
}

func info(thread *starlark.Thread, msg string, args ...interface{}) {
	ctx := getCtx(thread)
	pos := thread.CallFrame(1).Pos

	filepath := simplifyPath(ctx, ctx.filepath)

	log(ctx.ctx).Info().
		Msgf("%s:%d:%d: %s", filepath, pos.Line, pos.Col, fmt.Sprintf(msg, args...))
}

func warn(thread *starlark.Thread, msg string, args ...interface{}) {
	ctx := getCtx(thread)
	pos := thread.CallFrame(1).Pos

	filepath := simplifyPath(ctx, ctx.filepath)

	log(ctx.ctx).Warn().
		Msgf("%s:%d:%d: %s", filepath, pos.Line, pos.Col, fmt.Sprintf(msg, args...))
}

// * Builtin functions

func option(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var defaultValue starlark.String
	var help string

	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &name, "default?", &defaultValue, "help?", &help)
	if err != nil {
		return nil, err
	}

	ctx := getCtx(thread)
	if !ctx.initPhase {
		return nil, eris.New("can only be called during the init phase (in the global scope)")
	}

	ctx.options[name] = ScriptOption{
		DefaultValue: defaultValue,
		Help:         help,
	}

	value, ok := ctx.optionValues[name]
	if ok {
		return starlark.String(value), nil
	}

	return defaultValue, nil
}

func task(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var deps *starlark.List
	var skipIfExists *starlark.List
	var inputs *starlark.List
	var outputs *starlark.List
	var env *starlark.Dict
	var cmds *starlark.List

	task := new(Task)

	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "short??", &task.Short, "hidden?", &task.Hidden,
		"desc?", &task.Desc, "deps?", &deps, "base?", &task.Base, "skip_if_exists?", &skipIfExists, "inputs?",
		&inputs, "outputs?", &outputs, "env?", &env, "cmds?", &cmds, "ignore_exit?", &task.IgnoreExit)
	if err != nil {
		return nil, err
	}

	if task.Short == "" {
		task.Hidden = true
		task.Short = "auto#" + nanoid.New()
	}

	if task.Short == "configure" {
		return nil, eris.New(`the task name "configure" is reserved, please use a different name`)
	}

	task.Env = map[string]string{}

	if task.Base == "" {
		task.Base = "."
	}
	task.Base = normalizePath(getCtx(thread), task.Base)

	task.Deps, err = starlarkIterable2stringSlice(deps, "deps")
	if err != nil {
		return nil, err
	}

	task.SkipIfExists, err = starlarkIterable2stringSlice(skipIfExists, "skip_if_exists")
	if err != nil {
		return nil, err
	}

	task.Inputs, err = starlarkIterable2stringSlice(inputs, "inputs")
	if err != nil {
		return nil, err
	}

	task.Outputs, err = starlarkIterable2stringSlice(outputs, "outputs")
	if err != nil {
		return nil, err
	}

	if env != nil {
		for _, rawKey := range env.Keys() {
			var key string

			switch value := rawKey.(type) {
			case starlark.String:
				key = value.GoString()
			default:
				return nil, eris.Errorf("found key type %s in env map but only strings are supported", rawKey.Type())
			}

			rawValue, _, err := env.Get(rawKey)
			if err != nil {
				return nil, eris.Wrapf(err, "could not find key %s in env dict even though it appears in the key list", rawKey)
			}
			switch value := rawValue.(type) {
			case starlark.String:
				task.Env[key] = value.GoString()
			default:
				return nil, eris.Errorf("found value of type %s for key %s but only strings are supported", rawValue.Type(), key)
			}
		}
	}

	strBuffer := strings.Builder{}
	printer := syntax.NewPrinter(syntax.Minify(true))
	parser := syntax.NewParser()
	task.Cmds = make([]TaskCmd, 0)
	iter := cmds.Iterate()
	defer iter.Done()

	var item starlark.Value
	idx := 0
	for iter.Next(&item) {
		switch value := item.(type) {
		case starlark.String:
			task.Cmds = append(task.Cmds, TaskCmdScript{Content: value.GoString()})
		case starlark.Tuple:
			cmd, err := processCmdParts(value, parser, task.Base)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to process command #%d", idx)
			}

			strBuffer.Reset()
			err = printer.Print(&strBuffer, cmd)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to process command #%d", idx)
			}

			task.Cmds = append(task.Cmds, TaskCmdScript{Content: strBuffer.String()})
		case *starlark.List:
			parts := make(starlark.Tuple, value.Len())
			subIter := value.Iterate()
			var subItem starlark.Value
			subIdx := 0
			for subIter.Next(&subItem) {
				parts[subIdx] = subItem
				subIdx++
			}
			subIter.Done()

			cmd, err := processCmdParts(parts, parser, task.Base)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to process command #%d", idx)
			}

			strBuffer.Reset()
			err = printer.Print(&strBuffer, cmd)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to process command #%d", idx)
			}

			task.Cmds = append(task.Cmds, TaskCmdScript{Content: strBuffer.String()})
		case *Task:
			task.Cmds = append(task.Cmds, TaskCmdTaskRef{Task: value})
		default:
			return nil, eris.Errorf("unexpected type %s. Only strings, tuples and lists are valid", item.Type())
		}

		idx++
	}
	iter.Done()

	if inputs != nil && inputs.Len() > 0 && (outputs == nil || outputs.Len() == 0) {
		warn(thread, "found inputs but no outputs")
	}

	ctx := getCtx(thread)
	ctx.tasks = append(ctx.tasks, task)
	return task, nil
}

func hasTask(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &name)
	if err != nil {
		return nil, err
	}

	ctx := getCtx(thread)
	for _, task := range ctx.tasks {
		if task.Short == name {
			return starlark.True, nil
		}
	}

	return starlark.False, nil
}

func loadModule(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	ctx := getCtx(thread)
	module = normalizePath(ctx, module)

	result, found := ctx.moduleCache[module]
	if !found {
		shortModule := simplifyPath(ctx, module)

		oldFilepath := ctx.filepath
		ctx.filepath = module
		modThread := &starlark.Thread{
			Name:  shortModule,
			Print: thread.Print,
			Load:  thread.Load,
		}
		modThread.SetLocal("parserCtx", ctx)

		script, err := ioutil.ReadFile(module)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to load %s", shortModule)
		}

		result, err = starlark.ExecFile(thread, shortModule, script, ctx.globals)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to run %s", shortModule)
		}

		ctx.moduleCache[module] = result
		ctx.filepath = oldFilepath
	}

	return result, nil
}

// RunScript executes a starlake scripts and returns the declared options. If doConfigure is true, the script's
// configure function is called and the declared tasks are collected and returned.
func RunScript(ctx context.Context, filename, projectRoot string, options map[string]string, doConfigure bool) (TaskList, map[string]ScriptOption, []string, error) {
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, nil, nil, eris.Wrapf(err, "failed to build absolute path to %s", projectRoot)
	}

	filename, err = filepath.Abs(filename)
	if err != nil {
		return nil, nil, nil, eris.Wrapf(err, "failed to build absolute path to %s", filename)
	}

	builtins := starlark.StringDict{
		// global constants
		"OS":   starlark.String(runtime.GOOS),
		"ARCH": starlark.String(runtime.GOARCH),

		// log outputs
		"info":  starlark.NewBuiltin("info", starInfo),
		"warn":  starlark.NewBuiltin("warn", starWarn),
		"error": starlark.NewBuiltin("error", starError),

		// FS helpers
		"resolve_path":     starlark.NewBuiltin("resolve_path", resolvePath),
		"to_slashes":       starlark.NewBuiltin("to_slashes", toSlashes),
		"isdir":            starlark.NewBuiltin("isdir", starIsdir),
		"isfile":           starlark.NewBuiltin("isfile", starIsfile),
		"read_yaml":        starlark.NewBuiltin("read_yaml", readYaml),
		"write_file":       starlark.NewBuiltin("write_file", starWriteFile),
		"execute":          starlark.NewBuiltin("execute", starExec),
		"parse_shell_args": starlark.NewBuiltin("parse_shell_args", starParseShellArgs),

		// env handling
		"getenv":       starlark.NewBuiltin("getenv", getenv),
		"setenv":       starlark.NewBuiltin("setenv", setenv),
		"prepend_path": starlark.NewBuiltin("prepend_path", prependPathDir),

		// buildsys stuff
		"option":  starlark.NewBuiltin("option", option),
		"task":    starlark.NewBuiltin("task", task),
		"hastask": starlark.NewBuiltin("hastask", hasTask),

		// OS / compiler helpers
		"load_vcvars": starlark.NewBuiltin("load_vcvars", starLoadVcvars),
		"lookup_lib":  starlark.NewBuiltin("lookup_lib", starLookupLib),
	}

	thread := &starlark.Thread{
		Name: "main",
		Print: func(thread *starlark.Thread, msg string) {
			log(ctx).Info().Str("thread", thread.Name).Msg(msg)
		},
		Load: loadModule,
	}
	threadCtx := parserCtx{
		ctx:          ctx,
		globals:      builtins,
		filepath:     filename,
		projectRoot:  projectRoot,
		options:      make(map[string]ScriptOption),
		optionValues: options,
		envOverrides: make(map[string]string, 0),
		tasks:        make([]*Task, 0),
		moduleCache:  make(map[string]starlark.StringDict),
		yamlCache:    make(map[string]interface{}),
		initPhase:    true,
	}
	thread.SetLocal("parserCtx", &threadCtx)

	script, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, nil, eris.Wrapf(err, "failed to read file")
	}

	// wrap the entire script in a function to work around the limitation that ifs are only allowed inside functions
	globals, err := starlark.ExecFile(thread, simplifyPath(&threadCtx, filename), script, builtins)
	if err != nil {
		var evalError *starlark.EvalError
		if eris.As(err, &evalError) {
			return nil, nil, nil, eris.Errorf("failed to execute %s:\n%s", simplifyPath(&threadCtx, filename), evalError.Backtrace())
		}
		return nil, nil, nil, eris.Wrap(err, "failed to execute")
	}

	tasks := TaskList{}
	if doConfigure {
		configure, ok := globals["configure"]
		if !ok {
			return nil, nil, nil, eris.Errorf("%s did not declare a configure function", simplifyPath(&threadCtx, filename))
		}

		configureFunc, ok := configure.(starlark.Callable)
		if !ok {
			return nil, nil, nil, eris.Errorf("%s did declare a configure value but it's not a function", simplifyPath(&threadCtx, filename))
		}

		threadCtx.initPhase = false
		_, err = starlark.Call(thread, configureFunc, make(starlark.Tuple, 0), make([]starlark.Tuple, 0))
		if err != nil {
			var evalError *starlark.EvalError
			if eris.As(err, &evalError) {
				return nil, nil, nil, eris.New(evalError.Backtrace())
			}
			return nil, nil, nil, eris.Wrapf(err, "failed configure call in %s", simplifyPath(&threadCtx, filename))
		}

		for _, task := range threadCtx.tasks {
			tasks[task.Short] = task

			for name, value := range threadCtx.envOverrides {
				_, present := task.Env[name]
				if !present {
					task.Env[name] = value
				}
			}
		}
	}

	scriptFiles := make([]string, 0, len(threadCtx.moduleCache))
	for path := range threadCtx.moduleCache {
		scriptFiles = append(scriptFiles, path)
	}

	return tasks, threadCtx.options, scriptFiles, nil
}
