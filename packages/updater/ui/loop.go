package ui

// #include <stdlib.h>
// #include <SDL2/SDL.h>
// #cgo         LDFLAGS: -lSDL2
// #cgo windows LDFLAGS: -ldinput8 -lshell32 -lsetupapi -ladvapi32 -luuid -lversion -loleaut32 -lole32 -limm32 -lwinmm -lgdi32 -luser32 -lm -Wl,--no-undefined
import "C"

import (
	"runtime"
	"unsafe"

	"github.com/inkyblackness/imgui-go/v4"
	"github.com/rotisserie/eris"
)

var (
	mainQueue chan func()
	running   bool
)

func RunOnMain(callback func()) {
	mainQueue <- callback
}

func getSDLError() string {
	msg := C.SDL_GetError()
	return C.GoString(msg)
}

func RunApp(title string, width, height int32) error {
	mainQueue = make(chan func())

	// Avoid threading issues around cgo / C
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := C.SDL_Init(C.SDL_INIT_VIDEO)
	if result < 0 {
		return eris.Errorf("failed to initialise SDL2: %s", getSDLError())
	}
	defer C.SDL_Quit()

	cTitle := C.CString(title)
	window := C.SDL_CreateWindow(cTitle, C.SDL_WINDOWPOS_CENTERED, C.SDL_WINDOWPOS_CENTERED, C.int(width), C.int(height),
		C.SDL_WINDOW_OPENGL|C.SDL_WINDOW_RESIZABLE|C.SDL_WINDOW_ALLOW_HIGHDPI)
	C.free(unsafe.Pointer(cTitle))

	if window == nil {
		return eris.Errorf("failed to create window: %s", getSDLError())
	}
	// If we fail to destroy the window during quitting, there's nothing we can do about it.
	//nolint:errcheck
	defer C.SDL_DestroyWindow(window)

	context := imgui.CreateContext(nil)
	defer context.Destroy()
	io := imgui.CurrentIO()

	// Disable ImGui's default ini settings store
	io.SetIniFilename("")
	initKeyMapping(io)
	loadFont(io)

	// If we fail to set an attribute, it's fine since none of them are critical.
	if runtime.GOOS == "darwin" {
		// Always required on Mac
		_ = C.SDL_GL_SetAttribute(C.SDL_GL_CONTEXT_FLAGS, C.SDL_GL_CONTEXT_FORWARD_COMPATIBLE_FLAG)
	} else {
		_ = C.SDL_GL_SetAttribute(C.SDL_GL_CONTEXT_FLAGS, 0)
	}

	_ = C.SDL_GL_SetAttribute(C.SDL_GL_CONTEXT_PROFILE_MASK, C.SDL_GL_CONTEXT_PROFILE_CORE)
	_ = C.SDL_GL_SetAttribute(C.SDL_GL_CONTEXT_MAJOR_VERSION, 3)
	_ = C.SDL_GL_SetAttribute(C.SDL_GL_CONTEXT_MINOR_VERSION, 2)

	_ = C.SDL_GL_SetAttribute(C.SDL_GL_DOUBLEBUFFER, 1)
	_ = C.SDL_GL_SetAttribute(C.SDL_GL_DEPTH_SIZE, 24)
	_ = C.SDL_GL_SetAttribute(C.SDL_GL_STENCIL_SIZE, 8)

	glCtx := C.SDL_GL_CreateContext(window)
	if glCtx == nil {
		return eris.Errorf("failed to create OpenGL context: %s", getSDLError())
	}

	result = C.SDL_GL_MakeCurrent(window, glCtx)
	if result < 0 {
		return eris.Errorf("failed to make the OpenGL context current: %s", getSDLError())
	}

	result = C.SDL_GL_SetSwapInterval(1)
	if result < 0 {
		return eris.Errorf("failed to set OpenGL swap interval: %s", getSDLError())
	}

	renderer, err := NewOpenGL3(io)
	if err != nil {
		return eris.Errorf("failed to initialise OpenGL3 renderer: %s", getSDLError())
	}

	lastTime := C.uint64_t(0)
	buttonsDown := make([]bool, 3)
	running = true

	for running {
		// Process events
		var event C.SDL_Event
		for count := C.SDL_PollEvent(&event); count > 0; count = C.SDL_PollEvent(&event) {
			evp := unsafe.Pointer(&event)
			switch (*C.SDL_CommonEvent)(evp)._type {
			case C.SDL_QUIT:
				running = false
			case C.SDL_MOUSEWHEEL:
				//nolint:forcetypeassert // type is guarenteed here
				wheelEvent := (*C.SDL_MouseWheelEvent)(evp)
				var deltaX, deltaY float32
				if wheelEvent.x > 0 {
					deltaX++
				} else if wheelEvent.x < 0 {
					deltaX--
				}
				if wheelEvent.y > 0 {
					deltaY++
				} else if wheelEvent.y < 0 {
					deltaY--
				}
				io.AddMouseWheelDelta(deltaX, deltaY)
			case C.SDL_MOUSEBUTTONDOWN:
				//nolint:forcetypeassert // type is guarenteed here
				buttonEvent := (*C.SDL_MouseButtonEvent)(evp)
				switch buttonEvent.button {
				case C.SDL_BUTTON_LEFT:
					buttonsDown[0] = true
				case C.SDL_BUTTON_RIGHT:
					buttonsDown[1] = true
				case C.SDL_BUTTON_MIDDLE:
					buttonsDown[2] = true
				}
			case C.SDL_TEXTINPUT:
				//nolint:forcetypeassert // type is guarenteed here
				inputEvent := (*C.SDL_TextInputEvent)(evp)
				io.AddInputCharacters(C.GoString(&inputEvent.text[0]))
			case C.SDL_KEYDOWN:
				//nolint:forcetypeassert // type is guarenteed here
				keyEvent := (*C.SDL_KeyboardEvent)(evp)
				io.KeyPress(int(keyEvent.keysym.scancode))
				updateKeyModifier(io)
			case C.SDL_KEYUP:
				//nolint:forcetypeassert // type is guarenteed here
				keyEvent := (*C.SDL_KeyboardEvent)(evp)
				io.KeyRelease(int(keyEvent.keysym.scancode))
				updateKeyModifier(io)
			}
		}

		// Update window size in case it was resized
		var displayWidth, displayHeight C.int
		C.SDL_GetWindowSize(window, &displayWidth, &displayHeight)
		io.SetDisplaySize(imgui.Vec2{X: float32(displayWidth), Y: float32(displayHeight)})

		var frameWidth, frameHeight C.int
		C.SDL_GL_GetDrawableSize(window, &frameWidth, &frameHeight)

		// Update time
		freq := C.SDL_GetPerformanceFrequency()
		curTime := C.SDL_GetPerformanceCounter()
		if lastTime > 0 {
			io.SetDeltaTime(float32(curTime-lastTime) / float32(freq))
		} else {
			// Assume 1/60 of a second (60 FPS)
			io.SetDeltaTime(1.0 / 60.0)
		}
		lastTime = curTime

		// Update mouse state
		var x, y C.int
		state := C.SDL_GetMouseState(&x, &y)
		io.SetMousePosition(imgui.Vec2{X: float32(x), Y: float32(y)})
		for i, button := range []C.uint{C.SDL_BUTTON_LEFT, C.SDL_BUTTON_RIGHT, C.SDL_BUTTON_MIDDLE} {
			io.SetMouseButtonDown(i, buttonsDown[i] || (state&button) != 0)
			buttonsDown[i] = false
		}

		imgui.NewFrame()
		render()
		imgui.Render()

		renderer.PreRender([3]float32{0.0, 0.0, 0.0})
		renderer.Render(
			[2]float32{float32(displayWidth), float32(displayHeight)},
			[2]float32{float32(frameWidth), float32(frameHeight)},
			imgui.RenderedDrawData(),
		)
		C.SDL_GL_SwapWindow(window)

		// Process callbacks
	callbackLoop:
		for {
			select {
			case callback := <-mainQueue:
				callback()
			default:
				break callbackLoop
			}
		}
	}

	return nil
}

func initKeyMapping(io imgui.IO) {
	keyMapping := map[int]int{
		imgui.KeyTab:        C.SDL_SCANCODE_TAB,
		imgui.KeyLeftArrow:  C.SDL_SCANCODE_LEFT,
		imgui.KeyRightArrow: C.SDL_SCANCODE_RIGHT,
		imgui.KeyUpArrow:    C.SDL_SCANCODE_UP,
		imgui.KeyDownArrow:  C.SDL_SCANCODE_DOWN,
		imgui.KeyPageUp:     C.SDL_SCANCODE_PAGEUP,
		imgui.KeyPageDown:   C.SDL_SCANCODE_PAGEDOWN,
		imgui.KeyHome:       C.SDL_SCANCODE_HOME,
		imgui.KeyEnd:        C.SDL_SCANCODE_END,
		imgui.KeyInsert:     C.SDL_SCANCODE_INSERT,
		imgui.KeyDelete:     C.SDL_SCANCODE_DELETE,
		imgui.KeyBackspace:  C.SDL_SCANCODE_BACKSPACE,
		imgui.KeySpace:      C.SDL_SCANCODE_BACKSPACE,
		imgui.KeyEnter:      C.SDL_SCANCODE_RETURN,
		imgui.KeyEscape:     C.SDL_SCANCODE_ESCAPE,
		imgui.KeyA:          C.SDL_SCANCODE_A,
		imgui.KeyC:          C.SDL_SCANCODE_C,
		imgui.KeyV:          C.SDL_SCANCODE_V,
		imgui.KeyX:          C.SDL_SCANCODE_X,
		imgui.KeyY:          C.SDL_SCANCODE_Y,
		imgui.KeyZ:          C.SDL_SCANCODE_Z,
	}

	for imKey, sdlKey := range keyMapping {
		io.KeyMap(imKey, sdlKey)
	}
}

func updateKeyModifier(io imgui.IO) {
	modState := C.SDL_GetModState()
	mapModifier := func(lMask C.SDL_Keymod, lKey int, rMask C.SDL_Keymod, rKey int) (lResult int, rResult int) {
		if (modState & lMask) != 0 {
			lResult = lKey
		}
		if (modState & rMask) != 0 {
			rResult = rKey
		}
		return
	}
	io.KeyShift(mapModifier(C.KMOD_LSHIFT, C.SDL_SCANCODE_LSHIFT, C.KMOD_RSHIFT, C.SDL_SCANCODE_RSHIFT))
	io.KeyCtrl(mapModifier(C.KMOD_LCTRL, C.SDL_SCANCODE_LCTRL, C.KMOD_RCTRL, C.SDL_SCANCODE_RCTRL))
	io.KeyAlt(mapModifier(C.KMOD_LALT, C.SDL_SCANCODE_LALT, C.KMOD_RALT, C.SDL_SCANCODE_RALT))
}
