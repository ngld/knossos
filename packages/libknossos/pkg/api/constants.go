package api

var (
	releaseBuild = "false"
	// Version contains the compiles libknossos version
	Version = "0.0.1"
	// Commit contains a short hash pointing to the commit that was used to compile libknossos
	Commit = "ffffff"
	// TwirpEndpoint points to Nebula's Twirp endpoint. This can be used to quickly switch between production
	// and dev environments.
	TwirpEndpoint = "https://nu.fsnebula.org/twirp"
	// SyncEndpoint points to Nebula's modsync endpoint
	SyncEndpoint = "https://nu.fsnebula.org/sync"
)

// ReleaseBuild indicates whether this build is a release build (true) or a debug build (false)
var ReleaseBuild = releaseBuild == "true"
