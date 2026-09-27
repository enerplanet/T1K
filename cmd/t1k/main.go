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

	"github.com/enerplanet/T1K/pkg/t1k"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

// Exit statuses: 0 on success, 1 when the configuration, the input or a rule
// failed, 2 on a usage error.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// options are the parsed command-line flags.
type options struct {
	configPath  string
	inPath      string
	outPath     string
	reverse     bool
	compact     bool
	showVersion bool
	printConfig bool
}

// run is the command with its streams as parameters, so tests can drive it
// without a process.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, status, done := parseFlags(args, stderr)
	if done {
		return status
	}
	if err := opts.execute(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "t1k: %v\n", err)
		return exitError
	}
	return exitOK
}

// parseFlags reads the flags; done reports that the command has nothing
// more to do (help was printed, or the flags were invalid) with status.
func parseFlags(args []string, stderr io.Writer) (opts options, status int, done bool) {
	fs := flag.NewFlagSet("t1k", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.configPath, "config", "", "mapping configuration file (default: the embedded EnerPlanET to MEME mapping)")
	fs.StringVar(&opts.inPath, "in", "", "input JSON file (default: standard input)")
	fs.StringVar(&opts.outPath, "out", "", "output JSON file (default: standard output)")
	fs.BoolVar(&opts.reverse, "reverse", false, "apply the reverse transformation (target structure to source structure)")
	fs.BoolVar(&opts.compact, "compact", false, "write compact JSON instead of indented output")
	fs.BoolVar(&opts.showVersion, "version", false, "print the version and exit")
	fs.BoolVar(&opts.printConfig, "print-config", false, "print the embedded default configuration and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: t1k [flags]\n\nConverts a JSON document with a T1K mapping configuration.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, exitOK, true
		}
		return opts, exitUsage, true
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "t1k: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return opts, exitUsage, true
	}
	return opts, exitOK, false
}

// execute performs the action the flags select: print the version or the
// embedded configuration, or convert a document.
func (o options) execute(stdin io.Reader, stdout io.Writer) error {
	switch {
	case o.showVersion:
		_, err := fmt.Fprintf(stdout, "t1k %s\n", version)
		return err
	case o.printConfig:
		return writeOutput("", stdout, append(t1k.DefaultConfigJSON(), '\n'))
	default:
		return o.convert(stdin, stdout)
	}
}

// convert loads the configuration, reads the input, transforms it in the
// selected direction and writes the result.
func (o options) convert(stdin io.Reader, stdout io.Writer) error {
	task, err := o.task()
	if err != nil {
		return err
	}
	input, err := readInput(o.inPath, stdin)
	if err != nil {
		return err
	}
	output, err := o.transform(task, input)
	if err != nil {
		return err
	}
	return writeOutput(o.outPath, stdout, append(output, '\n'))
}

// task builds the transformation task from the configuration and output
// format the flags select.
func (o options) task() (*t1k.TransformTask, error) {
	cfg := t1k.DefaultConfig()
	if o.configPath != "" {
		var err error
		if cfg, err = t1k.LoadConfigFile(o.configPath); err != nil {
			return nil, err
		}
	}
	opts := []t1k.Option{t1k.WithConfig(cfg)}
	if !o.compact {
		opts = append(opts, t1k.WithIndent("", "  "))
	}
	return t1k.NewTransformTask(opts...), nil
}

func (o options) transform(task *t1k.TransformTask, input []byte) ([]byte, error) {
	if o.reverse {
		return task.Reverse(input)
	}
	return task.Transform(input)
}

// readInput reads the named file, or standard input when no file is named.
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

// writeOutput writes to the named file, or to standard output when no file
// is named.
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
