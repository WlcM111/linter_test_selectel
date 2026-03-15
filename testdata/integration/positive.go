package integration

import (
	"context"
	"log/slog"

	"go.uber.org/zap"
)

type worker struct {
	logger *zap.Logger
	slog   *slog.Logger
}

func ok(passwordHash string) {
	slog.Info("starting server")
	slog.Info("request completed", "status", "ok")
	slog.Info("token validated")
	slog.Info("request completed", slog.String("status", "ok"))
	logger := zap.L().Named("worker")
	logger.Info("request completed")
	logger.Info("request completed", zap.String("status", "ok"))
	sugar := logger.Sugar()
	sugar.Infow("request completed", "status", "ok")
	slogger := slog.Default().WithGroup("request")
	slogger.LogAttrs(context.Background(), slog.LevelInfo, "request completed", slog.String("status", "ok"))
	_ = passwordHash
}
