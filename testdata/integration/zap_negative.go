package integration

import "go.uber.org/zap"

type server struct {
	logger *zap.Logger
}

func (s *server) run(apiKey string) {
	logger := zap.L()
	logger.Info("Started worker") // want "start with a lowercase"
	logger.Error("connection failed!!!") // want "must not contain punctuation"
	logger.Info("request completed", zap.String("api_key", apiKey)) // want "must not contain sensitive data"
	sugar := logger.Sugar()
	sugar.Infow("request completed", "api_key", apiKey) // want "must not contain sensitive data"
	s.logger.Info("сервер запущен") // want "contain only English letters"
}
