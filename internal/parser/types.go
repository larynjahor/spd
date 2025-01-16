package parser

type Package struct {
	// ID is a unique identifier for a package,
	// in a syntax provided by the underlying build system.
	//
	// Because the syntax varies based on the build system,
	// clients should treat IDs as opaque and not attempt to
	// interpret them.
	ID string

	// Name is the package name as it appears in the package source code.
	Name string

	// PkgPath is the package path as used by the go/types package.
	PkgPath string

	// Dir is the directory associated with the package, if it exists.
	//
	// For packages listed by the go command, this is the directory containing
	// the package files.
	Dir string

	// Errors contains any errors encountered querying the metadata
	// of the package, or while parsing or type-checking its files.
	Errors []Error `json:"Errors,omitempty"`

	// TypeErrors contains the subset of errors produced during type checking.
	// TypeErrors []types.Error

	// GoFiles lists the absolute file paths of the package's Go source files.
	// It may include files that should not be compiled, for example because
	// they contain non-matching build tags, are documentary pseudo-files such as
	// unsafe/unsafe.go or builtin/builtin.go, or are subject to cgo preprocessing.
	GoFiles []string

	// CompiledGoFiles lists the absolute file paths of the package's source
	// files that are suitable for type checking.
	// This may differ from GoFiles if files are processed before compilation.
	CompiledGoFiles []string

	// OtherFiles lists the absolute file paths of the package's non-Go source files,
	// including assembly, C, C++, Fortran, Objective-C, SWIG, and so on.
	OtherFiles []string `json:"OtherFiles,omitempty"`

	// EmbedFiles lists the absolute file paths of the package's files
	// embedded with go:embed.
	EmbedFiles []string `json:"EmbedFiles,omitempty"`

	// EmbedPatterns lists the absolute file patterns of the package's
	// files embedded with go:embed.
	EmbedPatterns []string `json:"EmbedPatterns,omitempty"`

	// IgnoredFiles lists source files that are not part of the package
	// using the current build configuration but that might be part of
	// the package using other build configurations.
	IgnoredFiles []string `json:"IgnoredFiles,omitempty"`

	// ExportFile is the absolute path to a file containing type
	// information for the package as provided by the build system.
	ExportFile string `json:"ExportFile,omitempty"`

	// Target is the absolute install path of the .a file, for libraries,
	// and of the executable file, for binaries.
	Target string `json:"Target,omitempty"`

	// Imports maps import paths appearing in the package's Go source files
	// to corresponding loaded Packages.
	Imports map[string]string `json:"Imports"`

	// Module is the module information for the package if it exists.
	//
	// Note: it may be missing for std and cmd; see Go issue #65816.
	Module *Module `json:"Module,omitempty"`
}

// An Error describes a problem with a package's metadata, syntax, or types.
type Error struct {
	Pos  string // "file:line:col" or "file:line" or "" or "-"
	Msg  string
	Kind ErrorKind
}

// ErrorKind describes the source of the error, allowing the user to
// differentiate between errors generated by the driver, the parser, or the
// type-checker.
type ErrorKind int

const (
	UnknownError ErrorKind = iota
	ListError
	ParseError
	TypeError
)

func (err Error) Error() string {
	pos := err.Pos
	if pos == "" {
		pos = "-" // like token.Position{}.String()
	}
	return pos + ": " + err.Msg
}

type Module struct {
	Vendored  bool
	Path      string       // module path
	Version   string       // module version
	Main      bool         // is this the main module?
	Indirect  bool         // is this module only an indirect dependency of main module?
	Dir       string       // directory holding files for this module, if any
	GoMod     string       // path to go.mod file used when loading this module, if any
	GoVersion string       // go version used in module
	Error     *ModuleError // error loading module
}

// ModuleError holds errors loading a module.
type ModuleError struct {
	Err string // the error itself
}
