package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func promptGPT3(ctx context.Context, input string) (string, error) {
	return "", nil
}

type gptPrioritizeInputTask struct {
	ID    string `json:"id"`    // notion page id
	Name  string `json:"name"`  // notion page title
	Notes string `json:"notes"` // commonmark of the page
}

type gptPrioritizeInput struct {
	DailyFocus string                   `json:"todays_focus"`
	Tasks      []gptPrioritizeInputTask `json:"tasks"`
}

type gptPrioritizeOutputTask struct {
	ID            string        `json:"id"`             // notion page id
	EstimatedTime time.Duration `json:"estimated_time"` // estimated time to complete the task
}

type gptPromptOutput struct {
	DailyFocus string                    `json:"todays_focus"`
	Tasks      []gptPrioritizeOutputTask `json:"tasks"`
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
	r, err := promptGPT3(ctx, prompt)
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
