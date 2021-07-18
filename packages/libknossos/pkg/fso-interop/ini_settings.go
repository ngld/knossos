package fsoInterop

type FSOLanguage string

// see https://github.com/scp-fs2open/fs2open.github.com/blob/a8a9127d811278baf49cda247417c66db2c148fc/code/localization/localize.cpp#L35-L40
const (
	FSOEnglish FSOLanguage = "English"
	FSOGerman  FSOLanguage = "German"
	FSOFrench  FSOLanguage = "French"
	FSOPolish  FSOLanguage = "Polish"
)

type IniSettings struct {
	Default       DefaultSettings
	Video         VideoSettings
	Sound         SoundSettings
	PXO           PXOSettings
	ForceFeedback ForceFeedbackSettings
}

type DefaultSettings struct {
	// this one should *really* be obsolete but it's still implemented
	UseLowMem  uint32
	LastPlayer string

	// defaults to 2, only works on Windows, see this for an explanation of the mask:
	// https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-setprocessaffinitymask
	ProcessorAffinity uint32

	// format works like this: OGL -(%dx%d)x%d bit
	// with the following values: width, height, depth (usually 32)
	// see https://github.com/scp-fs2open/fs2open.github.com/blob/f1deee8566a1c47f6c2c8e75462d9dd615958ce5/code/graphics/2d.cpp#L1479
	VideocardFs2open string
	ForceFullscreen  uint32
	// defaults to 0, valid range is [20, 120]
	MaxFPS uint32
	// defaults to 1, valid values are 0 (Bilinear) and 1 (Trilinear)
	TextureFilter       uint32
	OGLAntiAliasSamples uint32 `ini:"OGL_AntiAliasSamples"`

	Language FSOLanguage

	CurrentJoystickGUID string
	CurrentJoystick     int32
	EnableJoystickFF    bool
	EnableHitEffect     bool

	// used to generate the screenshot filenames
	ScreenshotNum uint32
	// valid values: Slow, 56K, ISDN, Cable, Fast
	ConnectionSpeed string

	SpeechVolume    uint32
	SpeechVoice     uint32
	SpeechTechroom  bool
	SpeechBriefings bool
	SpeechIngame    bool
	SpeechMulti     bool

	// apparently this is used to generate the filenames of exported DDS files
	// see https://github.com/scp-fs2open/fs2open.github.com/blob/3aa61dfccbe7ad661fa4f186d1fc16549ac63319/code/ddsutils/ddsutils.cpp#L383
	ImageExportNum uint32

	// default port is 7808
	ForcePort uint32
	// default is 1
	PXOBanners uint32
}

type VideoSettings struct {
	Display string
}

type SoundSettings struct {
	PlaybackDevice string
	CaptureDevice  string
	// default is 1 for medium (DS_SQ_MEDIUM), range is [0, 2]
	Quality uint32
	// default depends on Quality, 44800 aka 44.8kHz is the default for the high quality
	SampleRate uint32
	// this setting only works if the extension ALC_EXT_EFX is present in OpenAL
	EnableEFX bool
}

type ForceFeedbackSettings struct {
	Strength uint32
}

type PXOSettings struct {
	Login     string
	Password  string
	SquadName string
}
