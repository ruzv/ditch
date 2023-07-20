package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/pflag"
)

var (
	client = openai.NewClient(
		"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",
	)

	help                = pflag.BoolP("help", "h", false, "show help")
	websiteTextFilepath = pflag.StringP(
		"website-text-path", "w", "", "path to website text file",
	)
)

func main() {
	pflag.Parse()

	if *help {
		pflag.PrintDefaults()

		return
	}

	if *websiteTextFilepath == "" {
		fmt.Println("website-text-path is required")

		return
	}

	websiteTextFile, err := os.Open(*websiteTextFilepath)
	if err != nil {
		panic(err)
	}

	defer websiteTextFile.Close()

	rawWebsiteText, err := io.ReadAll(websiteTextFile)
	if err != nil {
		panic(err)
	}

	websiteTextLines := strings.Split(string(rawWebsiteText), "\n")

	var (
		tokens float64
		batch  []string
	)

	for _, line := range websiteTextLines {
		if line == "" {
			continue
		}

		tokens += float64(len(line))
		batch = append(batch, line)

		if tokens >= 3000 {
			resp, err := getCompleation(strings.Join(batch, "\n"))
			if err != nil {
				panic(err)
			}

			// b, err := json.MarshalIndent(resp, "", "    ")
			// if err != nil {
			// 	panic(err)
			// }

			// fmt.Println(string(b))

			fmt.Println(resp.Choices[0].Message)

			tokens = 0
			batch = nil
		}
	}
}

func getCompleation(partialWebsiteText string) (*openai.ChatCompletionResponse, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:     openai.GPT3Dot5Turbo,
			MaxTokens: 2048,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(
						"Extract name, surname, company, "+
							"position in company information from the following "+
							"website text. Format resuts as CSV rows %q",
						partialWebsiteText,
					),
				},
			},
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chat completion")
	}

	if resp.Choices[0].FinishReason != "stop" {
		return nil, errors.Errorf(
			"unexpected finish reason - %s, expecting reason -  'stop'",
			resp.Choices[0].FinishReason,
		)
	}

	return &resp, nil
}
