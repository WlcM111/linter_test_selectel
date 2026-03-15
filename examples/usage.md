# Usage examples

## Violations

```go
slog.Info("Starting server")
slog.Info("token: " + token)
slog.Info("request completed", slog.String("password", password))
logger.Info("request completed", zap.String("api_key", apiKey))
```

## Valid

```go
slog.Info("starting server")
slog.Info("token validated")
slog.Info("request completed", slog.String("status", "ok"))
sugar.Infow("request completed", "status", "ok")
```
