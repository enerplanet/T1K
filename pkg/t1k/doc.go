// Package t1k converts one JSON structure into another, driven by a
// declarative mapping configuration, and converts the result back again.
//
// A configuration is a JSON file made of rules. A rule pairs a path in the
// source document with a path in the target document ("from" and "to"),
// optionally passing the value through an invertible converter. Paths may
// contain variables that iterate over arrays and object keys, so a rule can
// move whole collections and restructure them, for example turning an array
// of objects into an object keyed by one of their fields. "each" rules group
// nested rules relative to a pair of elements. Because every rule is written
// once and read in both directions, one configuration gives both the forward
// transformation (Transform) and its reverse (Reverse).
//
// The package ships with a default configuration, config/enerplanet-to-meme.json
// in the repository (embedded through the config package), that turns an
// EnerPlanET calculation payload into a MEME job. Any other configuration can
// be loaded with LoadConfig or LoadConfigFile.
//
//	task := t1k.NewTransformTask()
//	meme, err := task.Transform(enerplanetPayload)
//	...
//	back, err := task.Reverse(meme)
//
// Each conversion runs on its own state, so tasks may be used from several
// goroutines at once. The t1k command in cmd/t1k exposes the same
// functionality on the command line.
//
// This package is the public surface; the mapping language itself lives in
// the internal packages jsonpath (paths), convert (converters) and mapping
// (rule compilation and evaluation), on top of jsondoc (the document model).
package t1k
