package logging

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"

	"github.com/tdeshazo/home-api/internal/build"
)

type closeFunc func() error

type Options struct {
	Level     slog.Level
	File      string
	Env       string
	Writer    io.Writer
	AddSource bool
}

type errWithAttrs struct {
	error
	attrs []slog.Attr
}

func WithAttrs(err error, args ...any) error {
	if err == nil {
		return nil
	}
	return &errWithAttrs{
		error: err,
		attrs: argsToAttr(args),
	}
}

// argsToAttr turns a list of typed or untyped values into a slice of [slog.Attr].
// args[i] is treated as a key if it is a string or an [slog.Attr]; otherwise, it
// is treated as a value with key "!BADKEY".
func argsToAttr(args []any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(args))
	for i := 0; i < len(args); {
		switch key := args[i].(type) {
		case slog.Attr:
			attrs = append(attrs, key)
			i++
		case string:
			if i+1 >= len(args) {
				attrs = append(attrs, slog.String("!BADKEY", key))
				i++
			} else {
				attrs = append(attrs, slog.Any(key, args[i+1]))
				i += 2
			}
		default:
			attrs = append(attrs, slog.Any("!BADKEY", args[i]))
			i++
		}
	}
	return attrs
}

func (e *errWithAttrs) Unwrap() error {
	return e.error
}

func (e *errWithAttrs) Attrs() []slog.Attr {
	return e.attrs
}

type attrError interface {
	Attrs() []slog.Attr
}

// Attrs recursively extracts all logging attributes from an error chain.
func Attrs(err error) []slog.Attr {
	var attrs []slog.Attr
	for err != nil {
		if ae, ok := err.(attrError); ok {
			attrs = append(attrs, ae.Attrs()...)
		}
		err = errors.Unwrap(err)
	}
	return attrs
}

type multiError interface {
	error
	Unwrap() []error
}

func errorAttrs(err error) []slog.Attr {
	return append([]slog.Attr{
		slog.String("message", err.Error()),
	}, Attrs(err)...)
}

func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key != "error" {
		return a
	}

	err, ok := a.Value.Any().(error)
	if !ok {
		return a
	}

	title := "error"
	var attrs []slog.Attr
	var m multiError
	if !errors.As(err, &m) {
		attrs = errorAttrs(err)
	} else {
		title = "errors"
		errs := m.Unwrap()
		for i := range errs {
			group := errorAttrs(errs[i])
			name := fmt.Sprintf("error_%d", i+1)
			attrs = append(attrs, slog.GroupAttrs(name, group...))
		}
	}

	if len(attrs) == 0 {
		return a
	}

	return slog.GroupAttrs(title, attrs...)
}

func New(opts Options) (*slog.Logger, closeFunc, error) {

	closers := []closeFunc{}
	close := func() error { return nil }

	handlerOptions := slog.HandlerOptions{
		AddSource:   opts.AddSource,
		Level:       opts.Level,
		ReplaceAttr: replaceAttr,
	}

	var handlers []slog.Handler
	if opts.Writer != nil {
		handlers = append(handlers, slog.NewJSONHandler(opts.Writer, &handlerOptions))
	} else {
		fd := os.Stderr.Fd()
		handlers = append(handlers, tint.NewHandler(os.Stderr, &tint.Options{
			AddSource:   opts.AddSource,
			Level:       opts.Level,
			ReplaceAttr: replaceAttr,
			NoColor:     !(isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)),
		}))
	}

	if opts.File != "" {
		f, err := os.OpenFile(opts.File, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return nil, close, err
		}
		bufferedFile := bufio.NewWriterSize(f, 8192)
		close = func() error {
			if err := bufferedFile.Flush(); err != nil {
				return fmt.Errorf("failed to flush log file: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("failed to close log file: %w", err)
			}
			return nil
		}
		handlers = append(handlers, slog.NewJSONHandler(bufferedFile, &slog.HandlerOptions{
			AddSource:   opts.AddSource,
			Level:       opts.Level,
			ReplaceAttr: replaceAttr,
		}))
		closers = append(closers, close)
	}
	closer := func() error {
		var errs []error
		for _, close := range closers {
			if err := close(); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	logger := slog.New(slog.NewMultiHandler(handlers...))

	env := opts.Env
	if env == "" {
		env = os.Getenv("ENV")
	}
	hostname, _ := os.Hostname()

	logger = logger.With(
		slog.String("git_sha", build.GitSHA),
		slog.String("build_time", build.BuildTime),
		slog.String("env", env),
		slog.String("hostname", hostname),
	)

	return logger, closer, nil
}
