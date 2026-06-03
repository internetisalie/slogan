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

## Testing

Run all tests in the module:
```bash
go test ./...
```
