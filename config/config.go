// Package config holds the mapping configurations shipped with T1K and embeds
// them, so the t1k package can load its default mapping without touching the
// file system. A mapping is added by placing its JSON file in this directory.
package config

import "embed"

// Default is the file name of the mapping used when none is given: the
// conversion of an EnerPlanET calculation payload into a MEME job.
const Default = "enerplanet-to-meme.json"

// Files holds every mapping in this directory, by file name.
//
//go:embed *.json
var Files embed.FS

// Read returns the JSON text of a shipped mapping.
func Read(name string) ([]byte, error) {
	return Files.ReadFile(name)
}
