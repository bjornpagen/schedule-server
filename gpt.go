package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"

	gpt "github.com/PullRequestInc/go-gpt3"
)

var _gptc gpt.Client

func setGptClient(apiKey string) {
	_gptc = gpt.NewClient(os.Getenv("OPENAI_API_KEY"), gpt.WithDefaultEngine(gpt.TextDavinci003Engine))
}

func promptGpt3(ctx context.Context, input string) (string, error) {
	r, err := _gptc.Completion(ctx, gpt.CompletionRequest{
		Prompt:      []string{input},
		Stop:        []string{"\n"},
		N:           gpt.IntPtr(1),
		MaxTokens:   gpt.IntPtr(512),
		Temperature: gpt.Float32Ptr(0),
		Echo:        false,
	})
	if err != nil {
		log.Fatalln(err)
	}

	return r.Choices[0].Text, nil
}

type gptPrioritizeInputTask struct {
	ID    string `json:"id"`    // intermediate task id
	Name  string `json:"name"`  // notion page title
	Notes string `json:"notes"` // notion page contents
}

type gptPrioritizeInput struct {
	DailyFocus string                   `json:"dailyFocus"`
	Tasks      []gptPrioritizeInputTask `json:"tasks"`
}

type gptPrioritizeOutputTask struct {
	ID      string `json:"id"`      // intermediate task id
	Minutes int    `json:"minutes"` // minutes to spend on task
}

type gptPromptOutput struct {
	Tasks []gptPrioritizeOutputTask `json:"tasks"`
}

// gptPrioritizeFmt is the format string for the GPT-3 prompt, has two %s
//
//go:embed prompts/prioritize/in
var gptPrioritizeFmt string

// gptPrioritizeOutstub is the output stub for the GPT-3 prompt
//
//go:embed prompts/prioritize/outstub
var gptPrioritizeOutstub string

func gptPrioritize(ctx context.Context, in gptPrioritizeInput) (out gptPromptOutput, err error) {
	// marshal input into json string
	b, err := json.Marshal(in)
	if err != nil {
		return gptPromptOutput{}, fmt.Errorf("failed to marshal input: %w", err)
	}

	// format the prompt
	prompt := fmt.Sprintf(gptPrioritizeFmt, string(b), gptPrioritizeOutstub)

	// prompt GPT-3
	r, err := promptGpt3(ctx, prompt)
	if err != nil {
		return gptPromptOutput{}, fmt.Errorf("failed to prompt GPT-3: %w", err)
	}

	out, err = gptPrioritizeParseRawOutput(r)
	if err != nil {
		return gptPromptOutput{}, fmt.Errorf("failed to parse GPT-3 output: %w", err)
	}

	return out, nil
}

func gptPrioritizeParseRawOutput(raw string) (out gptPromptOutput, err error) {
	// prepend the output stub
	raw = gptPrioritizeOutstub + raw

	// unmarshal the output
	err = json.Unmarshal([]byte(raw), &out)
	if err != nil {
		return gptPromptOutput{}, fmt.Errorf("failed to unmarshal output: %w", err)
	}

	return out, nil
}
