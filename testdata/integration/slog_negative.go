package integration

import (
	"context"
	"log/slog"
)

type service struct {
	logger *slog.Logger
}

func (s *service) run(token string, password string) {
	slog.Info("Starting server") // want "start with a lowercase"
	slog.Error("ошибка подключения") // want "contain only English letters"
	slog.Warn("connection failed!!!") // want "must not contain punctuation"
	slog.Info("request completed", "password", password) // want "must not contain sensitive data"
	slog.Info("token: " + token) // want "must not contain sensitive data"
	slog.Info("request completed", slog.String("password", password)) // want "must not contain sensitive data"
	s.logger.Info("Request validated") // want "start with a lowercase"
	s.logger.LogAttrs(context.Background(), slog.LevelInfo, "request completed", slog.String("token", token)) // want "must not contain sensitive data"
	slog.Info("request completed", slog.Group("auth", slog.String("api_key", token))) // want "must not contain sensitive data"
}
