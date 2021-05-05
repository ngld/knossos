// Package exporter contains code to convert metadata from our database into exportable
// (i.e. Protobuf) structs and store them in files.
// Most notably, this package is responsible for writing the modsync files which
// Knossos uses to update its list of available mods.
package exporter
