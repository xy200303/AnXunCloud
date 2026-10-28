package ai

import (
	"context"

	"anxuncloud/internal/pkg/strutil"
)

type PingParams struct {
	Protocol       string
	BaseURL        string
	APIKey         string
	Model          string
	TimeoutSeconds int
}

func Ping(ctx context.Context, p PingParams) (string, error) {
	result, err := ChatText(ctx, TextChatParams{
		Protocol: p.Protocol, BaseURL: p.BaseURL, APIKey: p.APIKey, Model: p.Model,
		TimeoutSeconds: p.TimeoutSeconds, MaxOutput: 32,
		Messages: []TextMessage{{Role: "user", Content: "ping，请回复 ok"}},
	})
	if err != nil {
		return "", err
	}
	return strutil.Truncate(result.Text, 200), nil
}
