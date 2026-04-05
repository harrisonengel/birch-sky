package mcp

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeAnalyzer uses the Claude API to analyze data and answer questions
// without revealing the raw data to the buyer agent.
type ClaudeAnalyzer struct {
	client anthropic.Client
}

func NewClaudeAnalyzer(apiKey string) *ClaudeAnalyzer {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &ClaudeAnalyzer{client: client}
}

func (a *ClaudeAnalyzer) Analyze(ctx context.Context, data io.Reader, questions []string) ([]string, error) {
	rawData, err := io.ReadAll(io.LimitReader(data, 100*1024))
	if err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}

	prompt := fmt.Sprintf(`You are analyzing a dataset for a buyer's agent on the Information Exchange.
The buyer wants to understand this data WITHOUT seeing the raw data themselves.

IMPORTANT RULES:
- Answer the questions based on the data provided.
- Do NOT include raw data values, specific records, or exact data points in your answers.
- Provide summary statistics, trends, patterns, and qualitative descriptions.
- If a question asks for specific data points, describe the general pattern instead.

DATA:
%s

QUESTIONS:
%s

Answer each question on a new line, prefixed with the question number.`, string(rawData), formatQuestions(questions))

	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_5,
		MaxTokens: 2048,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude api: %w", err)
	}

	responseText := ""
	for _, block := range message.Content {
		if block.Type == "text" {
			responseText += block.Text
		}
	}

	answers := parseAnswers(responseText, len(questions))
	return answers, nil
}

// StubAnalyzer returns placeholder answers for testing without Claude API.
type StubAnalyzer struct{}

func NewStubAnalyzer() *StubAnalyzer {
	return &StubAnalyzer{}
}

func (a *StubAnalyzer) Analyze(_ context.Context, data io.Reader, questions []string) ([]string, error) {
	preview, _ := io.ReadAll(io.LimitReader(data, 1024))
	dataLen := len(preview)

	answers := make([]string, len(questions))
	for i, q := range questions {
		answers[i] = fmt.Sprintf("Analysis for '%s': This dataset contains approximately %d bytes of data. A full analysis would require the Claude API to be configured.", q, dataLen)
	}
	return answers, nil
}

func formatQuestions(questions []string) string {
	var sb strings.Builder
	for i, q := range questions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
	}
	return sb.String()
}

func parseAnswers(response string, numQuestions int) []string {
	answers := make([]string, numQuestions)
	lines := strings.Split(response, "\n")

	currentQ := -1
	var currentAnswer strings.Builder

	for _, line := range lines {
		for i := 0; i < numQuestions; i++ {
			prefix1 := fmt.Sprintf("%d.", i+1)
			prefix2 := fmt.Sprintf("%d:", i+1)
			if strings.HasPrefix(strings.TrimSpace(line), prefix1) || strings.HasPrefix(strings.TrimSpace(line), prefix2) {
				if currentQ >= 0 && currentQ < numQuestions {
					answers[currentQ] = strings.TrimSpace(currentAnswer.String())
				}
				currentQ = i
				currentAnswer.Reset()
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, prefix1) {
					currentAnswer.WriteString(strings.TrimSpace(trimmed[len(prefix1):]))
				} else {
					currentAnswer.WriteString(strings.TrimSpace(trimmed[len(prefix2):]))
				}
				break
			}
		}
		if currentQ >= 0 {
			currentAnswer.WriteString("\n" + line)
		}
	}
	if currentQ >= 0 && currentQ < numQuestions {
		answers[currentQ] = strings.TrimSpace(currentAnswer.String())
	}

	for i := range answers {
		if answers[i] == "" {
			answers[i] = response
		}
	}

	return answers
}
