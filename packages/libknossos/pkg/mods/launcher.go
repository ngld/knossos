package mods

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rotisserie/eris"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
)

func getEngineForMod(ctx context.Context, mod *common.Release) (*common.Release, error) {
	var engine *common.Release

	for modid, version := range mod.DependencySnapshot {
		dep, err := storage.GetMod(ctx, modid)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to resolve dependency %s (%s)", modid, version)
		}

		if dep.Type == common.ModType_ENGINE {
			if engine != nil {
				return nil, eris.New("more than one engine dependency")
			}

			engine, err = storage.GetModRelease(ctx, modid, version)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to find release %s (%s) during engine lookup", modid, version)
			}

			engine.Packages = FilterUnsupportedPackages(ctx, engine.Packages)
		}
	}

	if engine == nil {
		return nil, eris.New("no engine found")
	}

	return engine, nil
}

func getBinaryForEngine(ctx context.Context, engine *common.Release) (string, error) {
	binaryScore := uint32(0)
	binaryPath := ""

	engine.Packages = FilterUnsupportedPackages(ctx, engine.Packages)

	for _, pkg := range engine.Packages {
		for _, exe := range pkg.Executables {
			if exe.Label == "" && !exe.Debug && exe.Priority > binaryScore {
				binaryScore = exe.Priority
				binaryPath = filepath.Join(engine.Folder, pkg.Folder, exe.Path)
			}
		}
	}

	return binaryPath, nil
}

func getJSONFlagsForBinary(ctx context.Context, binaryPath string) (*storage.JSONFlags, error) {
	flags, err := storage.GetEngineFlags(ctx, binaryPath)
	if err != nil {
		return nil, err
	}
	if flags != nil {
		api.Log(ctx, api.LogInfo, "Using cached flags for %s", binaryPath)
		return flags, nil
	}

	api.Log(ctx, api.LogInfo, "Running \"%s -parse_cmdline_only -get_flags json_v1\"", binaryPath)
	proc := exec.Command(binaryPath, "-parse_cmdline_only", "-get_flags", "json_v1")
	proc.Env = append(proc.Env, "FSO_KEEP_STDOUT=1")
	out, err := proc.CombinedOutput()
	// Ignore the error if it's only about the exit code being 1 because that's normal.
	if err != nil && proc.ProcessState.ExitCode() != 1 {
		return nil, eris.Wrapf(err, "failed to run %s", binaryPath)
	}

	api.Log(ctx, api.LogInfo, "Got: %s", out)
	flags = new(storage.JSONFlags)
	err = json.Unmarshal(out, flags)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to parse output from %s", binaryPath)
	}

	err = storage.SaveEngineFlags(ctx, binaryPath, flags)
	if err != nil {
		return nil, eris.Wrapf(err, "failed to save flags for %s", binaryPath)
	}

	return flags, nil
}

func GetFlagsForMod(ctx context.Context, mod *common.Release) (map[string]*client.FlagInfo_Flag, error) {
	engine, err := getEngineForMod(ctx, mod)
	if err != nil {
		return nil, err
	}

	return GetFlagsForEngine(ctx, engine)
}

func GetFlagsForEngine(ctx context.Context, engine *common.Release) (map[string]*client.FlagInfo_Flag, error) {
	result := make(map[string]*client.FlagInfo_Flag)

	binaryPath, err := getBinaryForEngine(ctx, engine)
	if err != nil {
		return nil, err
	}

	flags, err := getJSONFlagsForBinary(ctx, binaryPath)
	if err != nil {
		return nil, err
	}

	for _, flag := range flags.Flags {
		result[flag.Name] = &client.FlagInfo_Flag{
			Flag:     flag.Name,
			Label:    flag.Description,
			Category: flag.Type,
			Help:     flag.WebURL,
		}
	}

	return result, nil
}

func LaunchMod(ctx context.Context, mod *common.Release, settings *client.UserSettings) error {
	// Resolve the engine by checking all relevant options in the following order:
	//  1. custom build in the user settings (manual path to the binary)
	//  2. custom engine version (reference to an engine-type Release)
	//  3. mod default

	var err error
	binary := settings.GetCustomBuild()

	// TODO unnest
	//nolint:nestif
	if binary == "" {
		var engine *common.Release

		engOpts := settings.GetEngineOptions()
		if engOpts.GetModid() != "" {
			engine, err = storage.GetModRelease(ctx, engOpts.Modid, engOpts.Version)
			if err != nil {
				return eris.Wrap(err, "failed to fetch user engine")
			}
		} else {
			engine, err = getEngineForMod(ctx, mod)
			if err != nil {
				return err
			}
		}

		binary, err = getBinaryForEngine(ctx, engine)
		if err != nil {
			return eris.Wrapf(err, "failed to find binary for engine %s (%s)", engine.Modid, engine.Version)
		}
	}

	// Use the user's command line if one is set for this mod and fall back to the mod default otherwise.
	cmdline := settings.GetCmdline()
	if cmdline == "" {
		cmdline = mod.Cmdline
	}

	modMeta, err := storage.GetMod(ctx, mod.Modid)
	if err != nil {
		return err
	}

	parentVersions, err := storage.GetVersionsForMod(ctx, modMeta.Parent)
	if err != nil {
		return err
	}

	parent, err := storage.GetModRelease(ctx, modMeta.Parent, parentVersions[len(parentVersions)-1])
	if err != nil {
		return err
	}

	// Build the -mod flag
	modFlag := make([]string, 0, len(mod.DependencySnapshot))
	for _, ID := range mod.ModOrder {
		var rel *common.Release

		if ID == mod.Modid {
			rel = mod
		} else {
			version, ok := mod.DependencySnapshot[ID]
			if !ok {
				// This dependency is probably optional and missing, just skip it.
				// TODO Make this more explicit
				continue
			}

			rel, err = storage.GetModRelease(ctx, ID, version)
			if err != nil {
				return eris.Wrap(ModMissing{
					ModID:   ID,
					Version: version,
				}, "part of the dependency snapshot is missing")
			}
		}

		// TODO Allow mod authors to specify which (optional) packages from dependencies should be used.
		// For now, we just use all installed packages.

		for _, pkg := range rel.Packages {
			flagPath, err := filepath.Rel(parent.Folder, filepath.Join(rel.Folder, pkg.Folder))
			if err != nil {
				return err
			}

			modFlag = append(modFlag, flagPath)
		}
	}

	cmdline += " -mod \""
	cmdline += strings.Join(modFlag, ",")
	cmdline += "\""

	flags, err := getJSONFlagsForBinary(ctx, binary)
	if err != nil {
		return err
	}

	cmdlineFile := filepath.Join(flags.PrefPath, "data", "cmdline_fso.cfg")
	cmdlineFolder := filepath.Dir(cmdlineFile)
	err = os.MkdirAll(cmdlineFolder, 0770)
	if err != nil {
		return err
	}

	hdl, err := os.Create(cmdlineFile)
	if err != nil {
		return err
	}

	api.Log(ctx, api.LogInfo, "Command line flags: %s", cmdline)

	_, err = hdl.WriteString(cmdline)
	if err != nil {
		return err
	}

	err = hdl.Close()
	if err != nil {
		return err
	}

	proc := exec.Command(binary)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Dir = parent.Folder

	api.Log(ctx, api.LogInfo, "Launching %s in %s", binary, proc.Dir)

	err = proc.Start()
	if err != nil {
		return err
	}

	running := true
	go func() {
		err := proc.Wait()
		if err != nil {
			api.Log(ctx, api.LogError, "Failed to launch FSO: %s", eris.ToString(err, true))
		}

		running = false
	}()

	time.Sleep(3 * time.Second)

	if !running {
		var code string
		if runtime.GOOS == "windows" {
			code = fmt.Sprintf("%x", proc.ProcessState.ExitCode())
		} else {
			code = fmt.Sprintf("%d", proc.ProcessState.ExitCode())
		}

		return eris.Errorf("FSO closed after less than three seconds with exit code %s!", code)
	}

	return nil
}
