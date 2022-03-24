package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/inkyblackness/imgui-go/v4"
	"github.com/ngld/knossos/packages/updater/downloader"
	"github.com/ngld/knossos/packages/updater/platform"
	"github.com/rotisserie/eris"
)

type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

type stateType uint8

const (
	stateIntro stateType = iota + 1
	stateInstalling
	stateError
	stateFinish
)

type logItem struct {
	timestamp string
	message   string
	level     LogLevel
}

var (
	logLines       = make([]logItem, 0)
	autoScroll     = true
	progressStatus = "Initialising..."
	progress       = float32(0.0)
	state          = stateIntro
)

func render() {
	viewport := imgui.MainViewport()
	imgui.SetNextWindowPos(imgui.Vec2{
		X: 0,
		Y: 0,
	})
	imgui.SetNextWindowSize(viewport.Size())

	imgui.BeginV("MainWindow", nil, imgui.WindowFlagsNoDecoration|imgui.WindowFlagsNoMove|imgui.WindowFlagsNoResize|imgui.WindowFlagsNoSavedSettings)

	switch state {
	case stateIntro:
		introWindow()
	case stateInstalling, stateError:
		progressWindow()
	case stateFinish:
		finishWindow()
	}

	imgui.End()
}

var (
	installPath     string
	versions        []string
	selectedVersion = ""
	desktopShortcut bool
	menuShortcut    bool
	token           string
)

func introWindow() {
	imgui.Text("Install Path: ")
	// imgui.SameLine()
	imgui.InputText("", &installPath)
	imgui.SameLine()
	if imgui.Button("...") {
		path, err := platform.OpenFolder("Select installation folder", "")
		if err == nil {
			installPath = path
		}
	}

	imgui.Text("Version: ")
	// imgui.SameLine()
	if imgui.BeginCombo("##X", selectedVersion) {
		for _, item := range versions {
			if imgui.Selectable(item) {
				selectedVersion = item
			}
		}
		imgui.EndCombo()
	}

	if runtime.GOOS == "windows" {
		imgui.Spacing()
		imgui.Checkbox("Desktop icon", &desktopShortcut)
		imgui.Checkbox("Start menu icon", &menuShortcut)
	}

	imgui.Spacing()
	if imgui.Button("Install") {
		if installPath == "" {
			platform.ShowError("Please select an installation folder.")
		} else {
			info, err := os.Stat(installPath)
			ok := false
			switch {
			case eris.Is(err, os.ErrNotExist):
				// This is fine, we can create the folder
				ok = true
			case err != nil:
				platform.ShowError(fmt.Sprintf("Failed to check installation folder:\n%s", err))
			case !info.IsDir():
				platform.ShowError("The entered path doesn't point to a folder!")
			default:
				ok = true
			}

			if ok {
				state = stateInstalling
				go PerformInstallation(installPath, selectedVersion, token)
			}
		}
	}

	imgui.SameLineV(0, 10)
	if imgui.Button("Cancel") {
		running = false
	}
}

func InitIntroWindow() {
	var err error
	token, err = downloader.GetToken(downloader.Repo)
	if err != nil {
		RunOnMain(func() {
			platform.ShowError(fmt.Sprintf("Failed to retrieve available versions:\n%v", err))
		})
		return
	}

	tags, err := downloader.GetAvailableVersions(context.TODO(), token)
	if err != nil {
		RunOnMain(func() {
			platform.ShowError(fmt.Sprintf("Failed to retrieve available versions:\n%v", err))
		})
		return
	}

	osPrefix := runtime.GOOS + "-"
	filtered := make([]string, 0)
	for idx := len(tags) - 1; idx >= 0; idx-- {
		version := tags[idx]
		if strings.HasPrefix(version, osPrefix) {
			filtered = append(filtered, version[len(osPrefix):])
		}
	}

	versions = filtered
	selectedVersion = versions[0]

	if installPath == "" && runtime.GOOS == "windows" {
		installPath = "C:\\Program Files\\Knossos"
	}

	if len(os.Args) >= 3 && os.Args[1] == "--auto" {
		installPath = os.Args[2]

		if os.Args[3] != "" {
			selectedVersion = os.Args[3]
		}

		if runtime.GOOS == "windows" {
			desktopShortcut = os.Args[4] == "true"
			menuShortcut = os.Args[5] == "true"
		}

		state = stateInstalling
		PerformInstallation(installPath, selectedVersion, token)
	}
}

func progressWindow() {
	imgui.Text(progressStatus)
	imgui.ProgressBar(progress)
	imgui.Spacing()

	if state == stateError {
		if imgui.Button("Retry") {
			state = stateIntro
			logLines = make([]logItem, 0)
		}

		imgui.SameLineV(0, 20)
	}

	if imgui.Button("Clear") {
		logLines = make([]logItem, 0)
	}

	imgui.SameLine()
	imgui.Checkbox("Auto-scroll", &autoScroll)

	imgui.Separator()
	imgui.BeginChildV("LogScrollArea", imgui.Vec2{X: 0, Y: -5}, true, imgui.WindowFlagsAlwaysVerticalScrollbar)
	// tweak line padding
	imgui.PushStyleVarVec2(imgui.StyleVarItemSpacing, imgui.Vec2{X: 4, Y: 1})

	for _, line := range logLines {
		var color imgui.Vec4
		switch line.level {
		case LogDebug:
			color = imgui.Vec4{X: 1, Y: 1, Z: 1, W: 0.5}
		case LogInfo:
			color = imgui.Vec4{X: 1, Y: 1, Z: 1, W: 1.0}
		case LogWarn:
			color = imgui.Vec4{X: 1, Y: 1, Z: 0.5, W: 1}
		case LogError:
			color = imgui.Vec4{X: 1, Y: 0, Z: 0, W: 1}
		}

		imgui.PushStyleColor(imgui.StyleColorText, color)

		imgui.PushFont(PtMonoFont)
		imgui.Text(fmt.Sprintf("[%s]", line.timestamp))
		imgui.PopFont()

		imgui.SameLine()
		imgui.Text(line.message)

		imgui.PopStyleColor()
	}

	if state == stateError {
		imgui.Spacing()
		if imgui.Button("Retry") {
			state = stateIntro
			logLines = make([]logItem, 0)
		}
	}

	if autoScroll && imgui.ScrollY() >= imgui.ScrollMaxY() {
		imgui.SetScrollHereY(1.0)
	}

	imgui.PopStyleVar()
	imgui.EndChild()
}

func SetProgress(fraction float32, status string) {
	progress = fraction
	progressStatus = status
}

func Log(level LogLevel, message string, args ...interface{}) {
	logLines = append(logLines, logItem{
		timestamp: time.Now().Format(time.Kitchen),
		level:     level,
		message:   fmt.Sprintf(message, args...),
	})
}

func finishWindow() {
	imgui.Text("Done!")
	imgui.Spacing()

	if imgui.Button("Open Knossos") {
		var binpath string
		switch runtime.GOOS {
		case "darwin":
			binpath = "Knossos.app/Contents/MacOS/knossos"
		case "windows":
			binpath = "knossos.exe"
		default:
			binpath = "knossos"
		}

		binpath = filepath.Join(installPath, binpath)
		err := exec.Command(binpath).Start()
		if err != nil {
			platform.ShowError(fmt.Sprintf("Failed to launch Knossos:\n%s", eris.ToString(err, true)))
		} else {
			running = false
		}
	}

	imgui.SameLine()

	if imgui.Button("Close") {
		running = false
	}
}
