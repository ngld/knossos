package importer

type KnDep struct {
	ID       string
	Version  string
	Packages []string
}

type KnExe struct {
	File       string
	Label      string
	Properties struct {
		X64  bool
		SSE2 bool
		AVX  bool
		AVX2 bool
	}
}

type KnChecksum [2]string

type KnArchive struct {
	Filename string
	Dest     string
	Checksum KnChecksum
	FileSize int
	URLs     []string
}

type KnFile struct {
	Filename  string
	Archive   string
	ArchiveID int32
	OrigName  string
	Checksum  KnChecksum
}

type KnPackage struct {
	Name         string
	Notes        string
	Status       string
	Dependencies []KnDep
	Environment  string
	Folder       string
	IsVp         bool
	Executables  []KnExe
	Files        []KnArchive
	Filelist     []KnFile
}

type KnStruct struct {
	Mods []struct {
		ID            string
		Title         string
		Version       string
		Stability     string
		Description   string
		Logo          string
		Tile          string
		Banner        string
		ReleaseThread string `json:"release_thread"`
		Screenshots   []string
		Videos        []string
		Notes         string
		Folder        string
		FirstRelease  string `json:"first_release"`
		LastUpdate    string `json:"last_update"`
		Cmdline       string
		ModFlag       []string `json:"mod_flag"`
		Type          string
		Packages      []KnPackage
	}
}
