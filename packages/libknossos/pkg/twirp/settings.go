package twirp

import (
	"context"
	"fmt"
	"runtime"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/libknossos/pkg/fsointerop"
	"github.com/ngld/knossos/packages/libknossos/pkg/platform"
	"github.com/ngld/knossos/packages/libopenal"
	"github.com/rotisserie/eris"
	"github.com/veandco/go-sdl2/sdl"
)

func (kn *knossosServer) LoadFSOSettings(ctx context.Context, req *client.NullMessage) (*client.FSOSettings, error) {
	return fsointerop.LoadSettings(ctx)
}

func (kn *knossosServer) SaveFSOSettings(ctx context.Context, req *client.FSOSettings) (*client.SuccessResponse, error) {
	err := fsointerop.SaveSettings(ctx, req)
	if err != nil {
		return nil, err
	}

	return &client.SuccessResponse{Success: true}, nil
}

func (kn *knossosServer) GetHardwareInfo(ctx context.Context, req *client.NullMessage) (*client.HardwareInfoResponse, error) {
	info, err := libopenal.GetDeviceInfo(ctx)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS == "darwin" {
		platform.RunOnMain(func() {
			err = sdl.Init(sdl.INIT_VIDEO)
		})
	} else {
		err = sdl.Init(sdl.INIT_VIDEO)
	}
	if err != nil {
		return nil, eris.Wrap(err, "failed to init SDL")
	}

	displayCount, err := sdl.GetNumVideoDisplays()
	if err != nil {
		return nil, eris.Wrap(err, "failed to count displays")
	}

	resolutions := make([]string, 0)
	lastRes := ""

	for display := 0; display < displayCount; display++ {
		modes, err := sdl.GetNumDisplayModes(display)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to retrieve modes for display %d", display)
		}

		for mode := 0; mode < modes; mode++ {
			modeInfo, err := sdl.GetDisplayMode(display, mode)
			if err != nil {
				return nil, eris.Wrapf(err, "failed to process mode %d for display %d", mode, display)
			}

			name := fmt.Sprintf("%dx%d - %d", modeInfo.W, modeInfo.H, display)
			if name != lastRes {
				lastRes = name
				resolutions = append(resolutions, name)
			}
		}
	}

	voices, err := platform.GetVoices(ctx)
	if err != nil {
		return nil, eris.Wrap(err, "failed to enumerate voices")
	}

	return &client.HardwareInfoResponse{
		AudioDevices:    info.Devices,
		CaptureDevices:  info.Captures,
		DefaultPlayback: info.DefaultDevice,
		DefaultCapture:  info.DefaultCapture,
		Resolutions:     resolutions,
		Voices:          voices,
	}, nil
}

func (kn *knossosServer) GetJoystickInfo(ctx context.Context, req *client.NullMessage) (*client.JoystickInfoResponse, error) {
	var err error
	if runtime.GOOS == "darwin" {
		platform.RunOnMain(func() {
			err = sdl.Init(sdl.INIT_JOYSTICK)
		})
	} else {
		err = sdl.Init(sdl.INIT_JOYSTICK)
	}
	if err != nil {
		return nil, eris.Wrap(err, "failed to init SDL")
	}

	joysticks := make([]*client.JoystickInfoResponse_Joystick, sdl.NumJoysticks())
	for idx := range joysticks {
		joysticks[idx] = &client.JoystickInfoResponse_Joystick{
			Name: sdl.JoystickNameForIndex(idx),
			UUID: sdl.JoystickGetGUIDString(sdl.JoystickGetDeviceGUID(idx)),
		}
	}

	return &client.JoystickInfoResponse{Joysticks: joysticks}, nil
}
