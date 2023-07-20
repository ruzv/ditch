package main

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

func main() {

	client := openai.NewClient("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")
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hello!",
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}

	fmt.Println(resp.Choices[0].Message.Content)
}
