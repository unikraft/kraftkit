package tableprinter

import "fmt"

// TablePrinterOption is a type of func(*TablePrinter).
type TablePrinterOption func(*TablePrinter) error

// WithOutputFormat returns a function func(opts *TablePrinter)
// that sets `format` in TablePrinter pointer instance.
func WithOutputFormat(format TableOutputFormat) TablePrinterOption {
	return func(opts *TablePrinter) error {
		opts.format = format
		return nil
	}
}

// WithOutputFormatFromString returns a function func(opts *TablePrinter)
// that sets `format` in TablePrinter pointer instance of type `TableOutputFormat` from string.
func WithOutputFormatFromString(format string) TablePrinterOption {
	return func(opts *TablePrinter) error {
		if format == "" {
			return fmt.Errorf("unsupported table printer format: %s", format)
		}
		opts.format = TableOutputFormat(format)
		return nil
	}
}

// WithTableDelimeter returns a function func(opts *TablePrinter)
// that sets `delimeter` in TablePrinter pointer instance.
func WithTableDelimeter(delim string) TablePrinterOption {
	return func(opts *TablePrinter) error {
		opts.delimeter = delim
		return nil
	}
}

// WithFieldTruncateFunc returns a function func(opts *TablePrinter)
// that sets `truncateFunc` in TablePrinter pointer instance.
func WithFieldTruncateFunc(truncateFunc func(int, string) string) TablePrinterOption {
	return func(opts *TablePrinter) error {
		opts.truncateFunc = truncateFunc
		return nil
	}
}

// WithMaxWidth returns a function func(opts *TablePrinter)
// that sets `maxWidth` in TablePrinter pointer instance.
func WithMaxWidth(maxWidth int) TablePrinterOption {
	return func(opts *TablePrinter) error {
		opts.maxWidth = maxWidth
		return nil
	}
}
