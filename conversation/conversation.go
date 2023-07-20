package conversation

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
)

// ConversationID a set of data that identifies a conversation between a
// user and the bot.
type ConversationID struct {
	UserID    string
	ChannelID string
}

type ConversationManager struct {
	conf          *config
	conversations map[ConversationID]*Conversation
	mu            sync.RWMutex
}

type config struct {
	client          *openai.Client
	log             *logging.Logger
	model           string
	maxTokens       int
	tokenBufferSize int
}

func NewConversationManager(
	secretKey string, log *logging.Logger,
) *ConversationManager {
	return &ConversationManager{
		conf: &config{
			client:          openai.NewClient(secretKey),
			log:             log,
			model:           openai.GPT3Dot5Turbo,
			maxTokens:       4096,
			tokenBufferSize: 512,
		},
		conversations: make(map[ConversationID]*Conversation),
	}
}

func (cm *ConversationManager) GetConversation(id ConversationID) *Conversation {
	cm.mu.RLock()
	conv, ok := cm.conversations[id]
	cm.mu.RUnlock()

	if !ok {
		conv = &Conversation{
			conf: cm.conf,
		}
		cm.mu.Lock()
		cm.conversations[id] = conv
		cm.mu.Unlock()
	}

	return conv
}

type Conversation struct {
	conf   *config
	dialog []*conversationMessage
}

type conversationMessage struct {
	message openai.ChatCompletionMessage
	tokens  int
}

func (c *Conversation) Banter(
	ctx context.Context, userMessage string,
) (string, error) {
	c.clearTokenBuffer()

	tokensBefore := c.tokens()

	userDialog := &conversationMessage{
		message: openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userMessage,
		},
	}

	c.dialog = append(c.dialog, userDialog)

	resp, err := retry(
		3,
		func() (*openai.ChatCompletionResponse, error) {
			return c.getCompleation(ctx)
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to get completion")
	}

	userDialog.tokens = resp.Usage.PromptTokens - tokensBefore

	c.dialog = append(
		c.dialog,
		&conversationMessage{
			message: resp.Choices[0].Message,
			tokens:  resp.Usage.CompletionTokens,
		},
	)

	c.conf.log.Debugf("OpenAI response: \n %s", jsonString(resp))

	c.conf.log.Debugf("tokens: %d", c.tokens())

	return resp.Choices[0].Message.Content, nil
}

func (c *Conversation) getCompleation(
	ctx context.Context,
) (*openai.ChatCompletionResponse, error) {
	resp, err := c.conf.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:     c.conf.model,
			MaxTokens: c.conf.tokenBufferSize,
			Messages: mapSlice(
				c.dialog,
				func(m *conversationMessage) openai.ChatCompletionMessage {
					return m.message
				},
			),
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

func (c *Conversation) clearTokenBuffer() {
	tokens := c.tokens()

	for tokens > c.conf.maxTokens-c.conf.tokenBufferSize {
		tokens -= c.dialog[0].tokens
		c.dialog = c.dialog[1:]
	}
}

func (c *Conversation) tokens() int {
	var total int
	for _, m := range c.dialog {
		total += m.tokens
	}

	return total
}

func jsonString(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}

	return string(b)
}

func mapSlice[S, D any](s []S, f func(S) D) []D {
	d := make([]D, len(s))
	for i, v := range s {
		d[i] = f(v)
	}

	return d
}

func retry[T any](n int, f func() (T, error)) (T, error) {
	var (
		t, empty T
		err      error
	)

	for i := 0; i < n; i++ {
		t, err = f()
		if err != nil {
			continue
		}

		return t, nil
	}

	return empty, errors.Wrap(err, "failed to retry")
}
