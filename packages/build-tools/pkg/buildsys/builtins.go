package buildsys

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/rotisserie/eris"
	"go.starlark.net/starlark"
	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func path2string(_ *starlark.Thread, starPath starlark.Value) (string, error) {
	switch value := starPath.(type) {
	case starlark.String:
		return value.GoString(), nil
	case StarlarkPath:
		return string(value), nil
	default:
		return "", eris.Errorf("only accepts string arguments but argument was a %s", value.Type())
	}
}

func resolvePath(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	base := ""
	ctx := getCtx(thread)

	if len(kwargs) > 0 {
		for _, kv := range kwargs {
			starKey, ok := kv[0].(starlark.String)
			if !ok {
				return nil, eris.New("expected keyword arguments to be strings")
			}

			key := starKey.GoString()

			if key == "base" {
				switch value := kv[1].(type) {
				case starlark.String:
					base = value.GoString()
				case StarlarkPath:
					base = string(value)
				default:
					return nil, eris.Errorf("invalid type %s for keyword base, expected string or path", kv[1].Type())
				}

				base = normalizePath(ctx, base)
			} else {
				return nil, eris.Errorf("unexpected keyword argument %s", key)
			}
		}
	}

	if len(args) < 1 {
		return nil, eris.New("expects at least one argument")
	}

	parts := make([]string, len(args))
	for idx, path := range args {
		switch value := path.(type) {
		case starlark.String:
			parts[idx] = value.GoString()
		case StarlarkPath:
			parts[idx] = string(value)
		default:
			return nil, eris.Errorf("only accepts string arguments but argument %d was a %s", idx, path.Type())
		}
	}

	normPath := normalizePath(ctx, parts...)
	if base != "" {
		var err error
		normPath, err = filepath.Rel(base, normPath)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to build relative path for %s in %s", normPath, base)
		}
	}

	return StarlarkPath(normPath), nil
}

func toSlashes(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var starPath starlark.Value

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &starPath)
	if err != nil {
		return nil, err
	}

	path, err := path2string(thread, starPath)
	if err != nil {
		return nil, err
	}

	return starlark.String(filepath.ToSlash(path)), nil
}

func starInfo(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &message)
	if err != nil {
		return nil, err
	}

	info(thread, message)
	return starlark.None, nil
}

func starWarn(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &message)
	if err != nil {
		return nil, err
	}

	warn(thread, message)
	return starlark.None, nil
}

func starError(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &message)
	if err != nil {
		return nil, err
	}

	return nil, eris.New(message)
}

func getenv(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &key)
	if err != nil {
		return nil, err
	}

	envOverrides := getCtx(thread).envOverrides
	value, ok := envOverrides[key]
	if !ok {
		value = os.Getenv(key)
	}

	return starlark.String(value), nil
}

func setenv(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string
	var value string

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 2, &key, &value)
	if err != nil {
		return nil, err
	}

	envOverrides := getCtx(thread).envOverrides
	envOverrides[key] = value

	return starlark.True, nil
}

func prependPathDir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pathDir string

	if len(args) != 1 {
		return nil, eris.New("got %d arguments, want 1")
	}

	switch value := args[0].(type) {
	case starlark.String:
		pathDir = value.GoString()
	case StarlarkPath:
		pathDir = string(value)
	default:
		return nil, eris.Errorf("for parameter 1: got %s, want path or string", args[0].Type())
	}

	envOverrides := getCtx(thread).envOverrides
	path, ok := envOverrides["PATH"]
	if !ok {
		path = os.Getenv("PATH")
	}

	envOverrides["PATH"] = normalizePath(getCtx(thread), pathDir) + string(os.PathListSeparator) + path

	return starlark.String(envOverrides["PATH"]), nil
}

func readYaml(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var starYamlFile starlark.Value
	var yamlKey string
	var defaultValue starlark.Value

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 2, &starYamlFile, &yamlKey, &defaultValue)
	if err != nil {
		return nil, err
	}

	yamlFile, err := path2string(thread, starYamlFile)
	if err != nil {
		return nil, err
	}

	yamlFile = normalizePath(getCtx(thread), yamlFile)

	cache := getCtx(thread).yamlCache
	doc, loaded := cache[yamlFile]
	if !loaded {
		content, err := ioutil.ReadFile(yamlFile)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to open file %s", yamlFile)
		}

		err = yaml.Unmarshal(content, &doc)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to parse file %s", yamlFile)
		}

		cache[yamlFile] = doc
	}

	// parse the key
	value := reflect.ValueOf(doc)
	for _, key := range strings.Split(yamlKey, ".") {
		switch value.Kind() {
		case reflect.Map:
			value = value.MapIndex(reflect.ValueOf(key))
		case reflect.Slice:
			idx, err := strconv.Atoi(key)
			if err != nil {
				value = reflect.ValueOf(nil)
				goto endLoop
			} else {
				if idx >= value.Len() {
					value = reflect.ValueOf(nil)
					goto endLoop
				}
				value = value.Index(idx)
			}
		case reflect.Invalid:
			goto endLoop
		default:
			return nil, eris.Errorf("encountered unexpected value of kind %v in YAML document", value.Kind())
		}
	}

endLoop:
	if value.Kind() == reflect.Invalid || value.IsNil() {
		return defaultValue, nil
	}

	result, err := interfaceToStarlark(thread, value.Interface())
	if err != nil {
		return nil, eris.Wrap(err, fn.Name())
	}

	return result, nil
}

func starIsdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var starDirPath starlark.Value

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &starDirPath)
	if err != nil {
		return nil, err
	}

	dirPath, err := path2string(thread, starDirPath)
	if err != nil {
		return nil, err
	}

	dirPath = normalizePath(getCtx(thread), dirPath)
	info, err := os.Stat(dirPath)
	if err == nil && info.IsDir() {
		return starlark.True, nil
	}

	return starlark.False, nil
}

func starIsfile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var starFilePath starlark.Value

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &starFilePath)
	if err != nil {
		return nil, err
	}

	filePath, err := path2string(thread, starFilePath)
	if err != nil {
		return nil, err
	}

	filePath = normalizePath(getCtx(thread), filePath)
	info, err := os.Stat(filePath)
	if err == nil && info.Mode().IsRegular() {
		return starlark.True, nil
	}

	return starlark.False, nil
}

func starExec(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command starlark.Value
	var outputFormat string
	var showError bool

	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "command", &command, "format?", &outputFormat, "show_error?", &showError)
	if err != nil {
		return nil, err
	}

	if outputFormat == "" {
		outputFormat = "text"
	}

	if outputFormat != "text" && outputFormat != "json" {
		return nil, eris.Errorf("unsupported format %s", outputFormat)
	}

	var shellCmd []syntax.Node
	parser := syntax.NewParser()
	ctx := getCtx(thread)
	base := filepath.Dir(ctx.filepath)

	switch command := command.(type) {
	case starlark.String:
		part := TaskCmdScript{
			TaskName: fn.Name(),
			Index:    0,
			Content:  command.GoString(),
		}

		stmts, err := part.ToShellStmts(parser)
		if err != nil {
			return nil, err
		}

		shellCmd = make([]syntax.Node, len(stmts))
		for idx, stmt := range stmts {
			shellCmd[idx] = stmt
		}
	case starlark.Tuple:
		expr, err := processCmdParts(command, parser, base)
		if err != nil {
			return nil, err
		}

		shellCmd = []syntax.Node{expr}
	default:
		return nil, eris.Errorf("unexpected type %s for command parameter, only strings and tuples are valid", command.Type())
	}

	outputBuffer := strings.Builder{}
	errOut := os.Stderr

	if !showError {
		errOut = nil
	}

	runner, err := interp.New(
		interp.Dir(base),
		interp.Env(expand.ListEnviron(getEnvVars(ctx.envOverrides)...)),
		interp.ExecHandler(execHandler),
		interp.OpenHandler(openHandler),
		interp.StdIO(nil, &outputBuffer, errOut),
		interp.Params("-e"),
	)
	if err != nil {
		return nil, eris.Wrap(err, "failed to initialize runner")
	}

	success := true
	for _, cmd := range shellCmd {
		err := runner.Run(ctx.ctx, cmd)
		if err != nil {
			if showError {
				log(ctx.ctx).Error().Err(err).Msg("shell error")
			}
			success = false
			break
		}
	}

	if !success {
		return starlark.False, nil
	}

	if outputFormat == "json" {
		var decoded interface{}
		err = json.Unmarshal([]byte(outputBuffer.String()), &decoded)
		if err != nil {
			return nil, eris.Wrap(err, "failed to parse command output")
		}

		return interfaceToStarlark(thread, decoded)
	}

	return starlark.String(outputBuffer.String()), nil
}

func starLoadVcvars(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	arch := "amd64"

	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 0, &arch)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS != "windows" {
		return starlark.True, nil
	}

	ctx := getCtx(thread)

	vsWherePath := "C:\\Program Files (x86)\\Microsoft Visual Studio\\Installer\\vswhere.exe"
	cmd := exec.Command(vsWherePath, "-property", "installationPath", "-latest")
	output, err := cmd.Output()
	if err != nil {
		return nil, eris.Wrapf(err, "failed to run %s", vsWherePath)
	}

	vsPath := strings.Trim(string(output), " \r\n")
	if vsPath == "" {
		return nil, eris.New("No Visual Studio installation found. If you recently updated VS, you might have to restart your PC.")
	}

	info, err := os.Stat(vsPath)
	if err != nil {
		return nil, eris.Wrap(err, "failed to check VS installation directory")
	}

	if !info.IsDir() {
		return nil, eris.Errorf("the detected VS installation path %s does not exist", vsPath)
	}

	vcvarsall := filepath.Join(vsPath, "VC", "Auxiliary", "Build", "vcvarsall.bat")
	_, err = os.Stat(vcvarsall)
	if err != nil {
		return nil, eris.Wrap(err, "could not find vcvarsall.bat")
	}

	// A weak random number is fine here since it's just used to avoid collisions with other instances running in
	// parallel which is rare to begin with.
	//nolint:gosec
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("knbuildsys-%d", rand.Int()))
	err = os.Mkdir(tmpDir, 0o700)
	if err != nil {
		return nil, eris.Wrap(err, "could not create temporary directory")
	}
	defer os.RemoveAll(tmpDir)

	script := filepath.Join(tmpDir, "vchelper.bat")
	err = ioutil.WriteFile(script, []byte(`@echo off
call "`+vcvarsall+`" %*
echo KN_PATH=%PATH%
echo KN_INCLUDE=%INCLUDE%
echo KN_LIBPATH=%LIBPATH%
echo KN_LIB=%LIB%
`), 0o700)
	if err != nil {
		return nil, eris.Wrap(err, "failed to write helper script")
	}

	cmd = exec.Command("cmd", "/C", script, arch)
	cmd.Env = getEnvVars(ctx.envOverrides)
	output, err = cmd.Output()
	if err != nil {
		return nil, eris.Wrap(err, "failed to run helper script")
	}

	for _, line := range strings.Split(string(output), "\r\n") {
		if strings.HasPrefix(line, "KN_") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) < 2 {
				log(ctx.ctx).Error().Msgf("vchelper produced malformed line %s", line)
			} else {
				ctx.envOverrides[parts[0][3:]] = parts[1]
			}
		}
	}

	return starlark.True, nil
}

var libCache map[string]string

func starLookupLib(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var lib string
	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &lib)
	if err != nil {
		return nil, err
	}

	if libCache == nil {
		info(thread, "Running ldconfig -p")
		pattern := regexp.MustCompile(`^\s*([^ ]+) \([^)]+\) => (.*)$`)

		ldCmd := exec.Command("ldconfig", "-p")
		ldCmd.Stderr = os.Stderr

		output, err := ldCmd.Output()
		if err != nil {
			return nil, eris.Wrap(err, "failed to run ldconfig")
		}

		lines := strings.Split(string(output), "\n")
		libCache = make(map[string]string)
		for _, line := range lines {
			if line == "" {
				continue
			}

			match := pattern.FindStringSubmatch(line)
			if match == nil {
				warn(thread, "Skipping unexpected line from ldconfig: %s", line)
				continue
			}

			libCache[match[1]] = match[2]
		}
	}

	path, ok := libCache[lib]
	if !ok {
		return starlark.None, nil
	}

	return starlark.String(path), nil
}

func starParseShellArgs(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var argString string
	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &argString)
	if err != nil {
		return nil, err
	}

	if argString == "" {
		return make(StarlarkShellArgs, 0), nil
	}

	reader := strings.NewReader(argString)
	parser := syntax.NewParser()
	shellArgs, err := parser.Parse(reader, "shell args")
	if err != nil {
		return nil, eris.Wrapf(err, "failed to parse %s", argString)
	}

	if len(shellArgs.Stmts) != 1 {
		return nil, eris.Errorf("expected 1 statement, found %d", len(shellArgs.Stmts))
	}

	if shellArgs.Stmts[0].Cmd == nil {
		return nil, eris.New("could not parse shell args as a valid command")
	}

	if len(shellArgs.Stmts[0].Redirs) > 0 {
		return nil, eris.New("redirects (i.e. > /dev/null) are not supported in shell args")
	}

	call, ok := shellArgs.Stmts[0].Cmd.(*syntax.CallExpr)
	if !ok {
		return nil, eris.New("passed arguments are a valid shell expression but not arguments")
	}

	if len(call.Assigns) > 0 {
		return nil, eris.New("found variable assignments / env vars in shell args")
	}

	return StarlarkShellArgs(call.Args), nil
}

func starWriteFile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filename string
	var content string
	err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 2, &filename, &content)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(filename)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to open %s", filename)
	}

	_, err = f.WriteString(content)
	if err != nil {
		f.Close()
		return nil, eris.Wrapf(err, "failed to write %s", filename)
	}

	err = f.Close()
	if err != nil {
		return nil, eris.Wrapf(err, "failed to close %s", filename)
	}

	return starlark.None, nil
}
