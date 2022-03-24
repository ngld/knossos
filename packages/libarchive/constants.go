package libarchive

const (
	ModeRegular uint32 = 0o100000
	ModeLink    uint32 = 0o120000
	ModeSocket  uint32 = 0o140000
	ModeChar    uint32 = 0o020000
	ModeBlock   uint32 = 0o060000
	ModeDir     uint32 = 0o040000
	ModeFifo    uint32 = 0o010000
	ModePerm    uint32 = 0o777
)
