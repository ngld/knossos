package libarchive

const (
	ModeRegular uint32 = 0100000
	ModeLink    uint32 = 0120000
	ModeSocket  uint32 = 0140000
	ModeChar    uint32 = 0020000
	ModeBlock   uint32 = 0060000
	ModeDir     uint32 = 0040000
	ModeFifo    uint32 = 0010000
	ModePerm    uint32 = 0777
)
