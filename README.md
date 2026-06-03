# slogan

`slogan` is a powerful and flexible logging and error handling module for Go, built on top of the standard `log/slog` package. It provides structured error decoration, recursive metadata extraction, and a rich API for structured logging.

## Key Features

- **Consolidated API**: All logging and error handling utilities are integrated into a single `log` package.
- **Rich Logging API**: Standard (`Info`, `Warn`), and Context-aware (`InfoContext`) methods.
- **Structured Metadata**: Easily attach attributes and groups to loggers.
- **Structured Error Wrapping**: Decorate errors with log levels, custom attributes, and stack traces using the unified `log.Wrap` API.
- **Recursive Metadata Hoisting**: Automatically extract and promote error metadata (levels and attributes) from complex error trees, including those created with `errors.Join`.
- **Flexible Logger Creation**: Create loggers with functional options for writers, formats, levelers, sanitizers, and middleware.
- **Intelligent Backtraces**: Generate clean, focused diagnostic traces that recursively follow error chains while eliding redundant metadata wrappers.
- **Dynamic Level Management**: Control log levels globally or per-logger at runtime.
- **Data Sanitization**: Protect sensitive information with pluggable global or local sanitizers.

## Getting Started

### Convenience Constructors

`slogan` provides several ways to create loggers using the standard `*slog.Logger` API:

#### Slog API (`*slog.Logger`)

```go
// 1. Get the framework root logger (named "slogan"). 
// Use sparingly for high-level events or diagnostics.
logger := log.StandardLogger()

// 2. Create a new named logger with default handlers (console + remote)
logger = log.NewLogger("my-service")

// 3. Create a logger automatically named after the current package.
// Recommended for most application logic.
logger = log.NewPackageLogger()
```

#### Package Logger Configuration

When using `NewPackageLogger`, you can register prefixes to be trimmed from the logger name (e.g., to remove your module name):

```go
func init() {
    log.RegisterPackageLoggerPrefix("github.com/my-org/my-project/")
}
```

### Basic Logging

```go
logger.Info("simple message")
logger.Log(context.Background(), log.LevelTrace, "very verbose debugging") // Custom LevelTrace (-8)

// Context-aware logging
logger.InfoContext(ctx, "processing request")
```

### Operation Context

Track the current operation throughout your application using specialized context helpers:

```go
// 1. Inject operation into context
ctx = log.ContextWithOperation(ctx, "ProcessOrder")

// 2. Retrieve operation from context if needed
op := log.OperationFromContext(ctx) // returns "ProcessOrder"

// 3. Log using the context (operation is automatically included)
logger.InfoContext(ctx, "order received")
// Output: level=INFO msg="order received" operation=ProcessOrder
```

## Configuring Loggers

### Logger Options

Use `NewLoggerWithOpts` for granular control over logger behavior:

```go
logger := log.NewLoggerWithOpts("my-app",
    log.WithFormat(log.FormatJson),
    log.WithWriter(os.Stderr),
    log.WithLeveler(slog.LevelDebug),
    log.WithAttrs(slog.Int("pid", os.Getpid())),
    log.WithAddSource(true),
)
```

### Dynamic Level Management

Control log levels for all loggers or specific ones at runtime:

```go
// Set all loggers to WARN
log.SetAllLoggerLevels(slog.LevelWarn)

// Override a specific logger back to DEBUG
log.SetLoggerLevel("my-app", slog.LevelDebug)
```

## Structured Logging

Attach context to your loggers using groups or attributes.

```go
// Add attributes
logger = logger.With(
    slog.Int("user_id", 123),
    slog.String("role", "admin"),
)

// Add a structured group
logger = logger.WithGroup("request").With(
    slog.String("method", "POST"),
    slog.String("path", "/login"),
)

logger.Info("login attempt")
// Output (logfmt): level=INFO msg="login attempt" user_id=123 role=admin request.method=POST request.path=/login
```

## Advanced Error Handling

### Error Wrapping & Metadata Hoisting

Errors can carry their own log levels and attributes which are automatically hoisted to the top level of the log record.

```go
err := log.Wrap(fmt.Errorf("database failure"),
    log.WithErrorLevel(slog.LevelError),
    log.WithErrorAttrs(slog.String("db_id", "prod-01")),
    log.WithErrorStack(),
)

// The record is promoted to ERROR and db_id is hoisted automatically
logger.Info("operation failed", slog.Any(log.ErrorKey, err))
// Output: level=ERROR db_id=prod-01 error.message="database failure" ...
```

### Recursive Metadata (Joined Errors)

The middleware recursively explores error trees, ensuring metadata from `errors.Join` is correctly processed across the entire chain:

```go
err1 := log.Wrap(errors.New("e1"), log.WithErrorLevel(slog.LevelWarn))
err2 := log.Wrap(errors.New("e2"), log.WithErrorAttrs(slog.String("id", "456")))
joined := errors.Join(err1, err2)

logger.Info("multiple failures", slog.Any(log.ErrorKey, joined))
// Output: level=WARN id=456 ...
```

### Intelligent Backtraces

`slogan` generates clean diagnostic traces. It recursively follows error chains but automatically **elides** redundant metadata wrappers (implementing the `Decorator` interface) unless they provide a new stack trace.

## Extensibility

### Custom Error Decorators

Implement the `Decorator` interface to create custom metadata wrappers that are automatically handled by the elision logic in backtraces:

```go
type MyDecorator struct { error }
func (m MyDecorator) IsDecorator() {}
func (m MyDecorator) LogAttrs() []slog.Attr { return []slog.Attr{...} }
```

### Handlers & Middleware

Register additional handlers or inject middleware into the `slog` pipeline:

- **`WithHandlers(...slog.Handler)`**: Add extra output targets (e.g., Sentry, file).
- **`WithMiddleware(...slogmulti.Middleware)`**: Add pipeline processing (e.g., metrics, trace propagation).

### Data Sanitization

Protect sensitive fields using the `Sanitizer` interface. Sanitizers can be global (via `RegisterSanitizer`) or per-logger:

```go
masker := log.SanitizerFunc(func(a slog.Attr) slog.Value {
    if a.Key == "password" { return slog.StringValue("********") }
    return a.Value
})

logger := log.NewLoggerWithOpts("api", log.WithSanitizers(masker))
```

## Error Inspection & Diagnostics

### Stack Trace Capture

Capture and inspect stack traces for detailed error diagnostics:

```go
// Capture current stack
stack := log.NewStack(0) // skip=0 starts at caller of NewStack

// Use with error decoration
err := log.Wrap(fmt.Errorf("operation failed"),
    log.WithErrorStack(), // Automatically captures stack
)

// Iterate over frames
for frame := range log.Callers(skip) {
    file, line := frame.FileLine()
    funcName := frame.Function()
    shortName := frame.FunctionShort()
}
```

### Error Message Extraction

Extract and analyze error messages from error chains:

```go
err := fmt.Errorf("db error: %w", fmt.Errorf("connection timeout"))

// Get primary message (first component)
primary := log.Message(err) // returns "db error"

// Get all message components
messages := log.Messages(err) // returns []string{"db error", "connection timeout"}

// Generate formatted backtrace
trace := log.BackTrace(err) // Returns []byte with formatted trace

// Create structured logging map
logMap := log.LogValues("operation", err, log.NewStack(0))
// logMap contains: {"message": "operation", "cause": err, "stack": [...]}
```

### Error String Construction

Build error messages programmatically:

```go
// Combine message and cause
errStr := log.ErrorString("operation failed", cause)
// Returns "operation failed: <cause.Error()>"
```

## Context Utilities

### Propagating Log Attributes Through Call Chains

Beyond operation tracking, attach arbitrary log attributes to context for automatic inclusion in all logs:

```go
// Add multiple attributes to context
ctx = log.ContextWithLogAttrs(ctx,
    slog.String("request_id", "req-123"),
    slog.String("user_id", "user-456"),
)

// Attributes are automatically included in all logs from this context
logger.InfoContext(ctx, "processing")
// Output: level=INFO msg="processing" request_id=req-123 user_id=user-456

// Merge with existing attributes (new ones take precedence)
ctx = log.ContextWithLogAttrs(ctx, slog.String("request_id", "req-789"))
```

## Handler Types & Output Formats

### Available Handlers

slogan provides multiple output handlers, each optimized for different use cases:

| Handler | Format | Best For | Output |
| :--- | :--- | :--- | :--- |
| Human | `human` | Development & debugging | Multi-line formatted text with structure |
| JSON | `json` | Log aggregation & parsing | Compact JSON objects |
| Logfmt | `logfmt` (default) | CLI tools & awk/grep | key=value pairs |
| Tint | `tint` | Terminals with color | Colored key=value pairs |
| Plain | `plain` | Container logs & simplicity | Message only, no structure |

### Format Selection

```go
// Auto-detect based on TTY
logger := log.NewLogger("my-app") // Uses FormatTint on terminals, FormatLogFmt otherwise

// Force specific format
logger := log.NewLoggerWithOpts("my-app",
    log.WithFormat(log.FormatJson), // JSON output
)

// Available format constants:
// - log.FormatHuman
// - log.FormatJson
// - log.FormatLogFmt
// - log.FormatTint
// - log.FormatPlain
```

### Human Format Example

```
 12:34:56.123  INFO    simple message
                       key=value nested.key=value

 12:34:57.456  WARN    something concerning
                       reason=timeout retry=3
```

## Stdlib Compatibility

### LevelLogger (Drop-in Replacement)

For legacy code or frameworks that expect `*log.Logger` from the standard library:

```go
// Create a LevelLogger wrapping an slog.Logger
stdLogger := log.NewLevelLogger(logger, slog.LevelInfo)

// Use as drop-in replacement for *log.Logger
// All standard methods work:
stdLogger.Print("message")
stdLogger.Printf("formatted: %v", value)
stdLogger.Println("multi-line")
stdLogger.Fatal("fatal error")
stdLogger.Fatalf("fatal: %s", err)
stdLogger.Panic("panic!")
stdLogger.Panicf("panic: %v", err)

// Also supports structured logging:
stdLogger.Infoln("info")
stdLogger.Warnln("warning")
stdLogger.Errorln("error")

// And implements io.Writer for use with frameworks
io.Writer(stdLogger) // Logs writes as INFO level
```

### Standard Logger Reference

Get the framework root logger (use sparingly for high-level diagnostics):

```go
rootLogger := log.StandardLogger() // Named "slogan"
```

## Attribute Manipulation

For advanced use cases, manipulate structured attributes at the log level:

```go
// Convert map to attributes
attrs := log.MapAttrs(map[string]any{
    "user": "alice",
    "age": 30,
})

// Convert slice/array to indexed attributes
attrs := log.SliceAttrs(reflect.ValueOf([]int{1, 2, 3}))
// Result: [Attr{Key: "0", ...}, Attr{Key: "1", ...}, ...]

// Navigate nested attribute trees
value, ok := log.GetValueAtPath(attrs, "user", "profile", "email")

// Set attributes at nested paths (creates groups as needed)
attrs = log.SetAttrsAtPath(attrs, []string{"user"}, []slog.Attr{
    slog.String("role", "admin"),
})

// Merge attribute slices (groups recursively merged)
merged := log.MergeAttrs(existing, newAttrs)

// Convert values to proper slog.Value types
val := log.Value(any)     // Handles time.Time, net.IP, custom types
val := log.Attr("key", any) // Creates complete slog.Attr
```

## Reference: Constructor Options

The following options are available via `NewLoggerWithOpts`:

| Option | Description |
| :--- | :--- |
| `WithWriter(io.Writer)` | Sets the output writer for the default console handler. |
| `WithFormat(string)` | Sets the output format (`json`, `logfmt`, `human`, `tint`, `plain`). |
| `WithAttrs(...slog.Attr)` | Adds default attributes to all log records. |
| `WithLeveler(slog.Leveler)` | Sets the dynamic level controller for the logger. |
| `WithTimeFormat(string)` | Sets the timestamp format (pass `""` to suppress time). |
| `WithSanitizers(...Sanitizer)`| Adds local sanitizers for data masking. |
| `WithHandlers(...slog.Handler)` | Adds additional handlers to the fanned-out output. |
| `WithMiddleware(...Middleware)`| Adds custom `slogmulti` middleware to the pipeline. |
| `WithAddSource(bool)` | Enables/disables inclusion of source file and line number. |
| `WithoutConsole()` | Disables the default console handler. |

## Performance

slogan is designed for high-performance structured logging without sacrificing features:

- **Efficient Buffer Pooling**: Uses `bytebufferpool` for stack trace rendering and log formatting, reducing allocations
- **Lazy Stack Capture**: Stack traces are only captured when explicitly requested via `WithErrorStack()`
- **Attribute Merging**: Efficiently merges attributes using copy-on-write semantics to avoid unnecessary allocations
- **Dynamic Leveling**: Change log levels at runtime without handler recreation

## Testing

Run all tests in the module:
```bash
go test ./...
```
