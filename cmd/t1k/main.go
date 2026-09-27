// Command t1k converts a JSON document with a T1K mapping configuration.
//
//	t1k [-config FILE] [-reverse] [-in FILE] [-out FILE] [-compact]
//
// Without -config the mapping embedded in the package is used: the
// conversion of an EnerPlanET calculation payload into a MEME job. Without
// -in the document is read from standard input; without -out the result is
// written to standard output.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	t1k "github.com/enerplanet/T1K"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("t1k", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "mapping configuration file (default: the embedded EnerPlanET to MEME mapping)")
	inPath := fs.String("in", "", "input JSON file (default: standard input)")
	outPath := fs.String("out", "", "output JSON file (default: standard output)")
	rev := fs.Bool("reverse", false, "apply the reverse transformation (target structure to source structure)")
	compact := fs.Bool("compact", false, "write compact JSON instead of indented output")
	showVersion := fs.Bool("version", false, "print the version and exit")
	printConfig := fs.Bool("print-config", false, "print the embedded default configuration and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: t1k [flags]\n\nConverts a JSON document with a T1K mapping configuration.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "t1k: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return exitUsage
	}
	switch {
	case *showVersion:
		fmt.Fprintf(stdout, "t1k %s\n", version)
		return exitOK
	case *printConfig:
		_, err := stdout.Write(t1k.DefaultConfigJSON())
		if err == nil {
			_, err = io.WriteString(stdout, "\n")
		}
		return exitOn(stderr, err)
	}

	cfg := t1k.DefaultConfig()
	if *configPath != "" {
		var err error
		if cfg, err = t1k.LoadConfigFile(*configPath); err != nil {
			return exitOn(stderr, err)
		}
	}
	input, err := readInput(*inPath, stdin)
	if err != nil {
		return exitOn(stderr, err)
	}

	opts := []t1k.Option{t1k.WithConfig(cfg)}
	if !*compact {
		opts = append(opts, t1k.WithIndent("", "  "))
	}
	task := t1k.NewTransformTask(opts...)
	var output []byte
	if *rev {
		output, err = task.Reverse(input)
	} else {
		output, err = task.Transform(input)
	}
	if err != nil {
		return exitOn(stderr, err)
	}
	return exitOn(stderr, writeOutput(*outPath, stdout, append(output, '\n')))
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read standard input: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return data, nil
}

func writeOutput(path string, stdout io.Writer, data []byte) error {
	if path == "" {
		_, err := stdout.Write(data)
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func exitOn(stderr io.Writer, err error) int {
	if err == nil {
		return exitOK
	}
	fmt.Fprintf(stderr, "t1k: %v\n", err)
	return exitError
}
