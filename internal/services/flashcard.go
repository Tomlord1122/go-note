package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

// FlashcardService handles flashcard generation using LangChain
type FlashcardService struct {
	llm llms.Model
}

// NewFlashcardService creates a new flashcard service
func NewFlashcardService(ctx context.Context) (*FlashcardService, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable is required")
	}

	llm, err := googleai.New(
		ctx,
		googleai.WithAPIKey(apiKey),
		googleai.WithDefaultModel("gemini-1.5-flash"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LangChain GoogleAI client: %w", err)
	}

	return &FlashcardService{
		llm: llm,
	}, nil
}

// Close closes the flashcard service client (no-op for langchain)
func (s *FlashcardService) Close() error {
	return nil
}

// Note represents a note for flashcard generation
type Note struct {
	ID      string
	Title   string
	Content string
	Tags    []string
}

// Flashcard represents a generated flashcard
type Flashcard struct {
	Question    string   `json:"question"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation,omitempty"`
	Difficulty  string   `json:"difficulty,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// FlashcardStreamResponse represents different types of SSE messages
type FlashcardStreamResponse struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// StreamStatus represents the status of the streaming process
type StreamStatus struct {
	Stage       string `json:"stage"`
	Description string `json:"description"`
	Progress    int    `json:"progress"` // 0-100
}

// sendStatus sends a status update via SSE
func (s *FlashcardService) sendStatus(responseChan chan<- string, stage, description string, progress int) {
	status := StreamStatus{
		Stage:       stage,
		Description: description,
		Progress:    progress,
	}
	response := FlashcardStreamResponse{
		Type: "status",
		Data: status,
	}
	if jsonData, err := json.Marshal(response); err == nil {
		responseChan <- fmt.Sprintf("data: %s\n\n", string(jsonData))
	}
}

// sendChunk sends a content chunk via SSE
func (s *FlashcardService) sendChunk(responseChan chan<- string, chunk string) {
	response := FlashcardStreamResponse{
		Type:    "chunk",
		Message: chunk,
	}
	if jsonData, err := json.Marshal(response); err == nil {
		responseChan <- fmt.Sprintf("data: %s\n\n", string(jsonData))
	}
}

// sendError sends an error via SSE
func (s *FlashcardService) sendError(responseChan chan<- string, errorMsg string) {
	response := FlashcardStreamResponse{
		Type:  "error",
		Error: errorMsg,
	}
	if jsonData, err := json.Marshal(response); err == nil {
		responseChan <- fmt.Sprintf("data: %s\n\n", string(jsonData))
	}
}

// sendComplete sends completion with final flashcard via SSE
func (s *FlashcardService) sendComplete(responseChan chan<- string, flashcard *Flashcard) {
	response := FlashcardStreamResponse{
		Type: "complete",
		Data: flashcard,
	}
	if jsonData, err := json.Marshal(response); err == nil {
		responseChan <- fmt.Sprintf("data: %s\n\n", string(jsonData))
	}
}

// StreamFlashcardFromNotes generates a flashcard from multiple notes with SSE streaming
func (s *FlashcardService) StreamFlashcardFromNotes(ctx context.Context, notes []Note, responseChan chan<- string) error {
	defer close(responseChan)

	if len(notes) == 0 {
		s.sendError(responseChan, "至少需要一個筆記")
		return fmt.Errorf("at least one note is required")
	}

	// Send initial status
	s.sendStatus(responseChan, "preparing", "準備處理筆記...", 10)

	// Combine notes into a single context
	var notesContent strings.Builder
	var allTags []string
	tagSet := make(map[string]bool)

	for i, note := range notes {
		notesContent.WriteString(fmt.Sprintf("筆記 %d - %s:\n%s\n\n", i+1, note.Title, note.Content))

		// Collect unique tags
		for _, tag := range note.Tags {
			if !tagSet[tag] {
				allTags = append(allTags, tag)
				tagSet[tag] = true
			}
		}
	}

	s.sendStatus(responseChan, "generating", "正在生成閃卡...", 50)

	prompt := fmt.Sprintf(`基於以下筆記請幫用戶想三個問題，這三個問題來幫助他學習，切記只需要有問題以及剪短解答即可，務必要用條列式的方式說明。筆記:%s
專注於這些筆記中最重要的概念或關係。讓問題足夠具體，對學習有用，每個問題之間希望你能空兩格。你的回覆只需要是 markdown 即可。`, notesContent.String())

	// Collect all content first, then parse
	var fullContent strings.Builder

	// Use streaming generation
	_, err := s.llm.GenerateContent(ctx, []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(prompt),
			},
		},
	}, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		chunkStr := string(chunk)
		fullContent.WriteString(chunkStr)
		// Send each chunk as it arrives
		s.sendChunk(responseChan, chunkStr)
		return nil
	}))

	if err != nil {
		s.sendError(responseChan, fmt.Sprintf("生成閃卡失敗: %v", err))
		return fmt.Errorf("failed to generate flashcard: %w", err)
	}

	s.sendStatus(responseChan, "parsing", "解析閃卡內容...", 90)

	// Parse the complete response
	flashcard, err := s.parseJSONFlashcardResponse(fullContent.String())
	if err != nil {
		s.sendError(responseChan, fmt.Sprintf("解析閃卡失敗: %v", err))
		return fmt.Errorf("failed to parse flashcard: %w", err)
	}

	flashcard.Tags = allTags
	s.sendComplete(responseChan, flashcard)
	s.sendStatus(responseChan, "completed", "閃卡生成完成！", 100)

	return nil
}

// StreamFlashcardFromQuery generates a flashcard based on a user query and related notes with SSE streaming
func (s *FlashcardService) StreamFlashcardFromQuery(ctx context.Context, query string, relatedNotes []Note, responseChan chan<- string) error {
	defer close(responseChan)

	if query == "" {
		s.sendError(responseChan, "查詢不能為空")
		return fmt.Errorf("query cannot be empty")
	}
	if len(relatedNotes) == 0 {
		s.sendError(responseChan, "沒有找到相關筆記")
		return fmt.Errorf("no related notes found")
	}

	s.sendStatus(responseChan, "preparing", "準備處理查詢和相關筆記...", 10)

	// Combine notes into context
	var notesContent strings.Builder
	var allTags []string
	tagSet := make(map[string]bool)

	for i, note := range relatedNotes {
		notesContent.WriteString(fmt.Sprintf("筆記 %d - %s:\n%s\n\n", i+1, note.Title, note.Content))

		// Collect unique tags
		for _, tag := range note.Tags {
			if !tagSet[tag] {
				allTags = append(allTags, tag)
				tagSet[tag] = true
			}
		}
	}

	s.sendStatus(responseChan, "generating", "正在生成閃卡...", 50)

	prompt := fmt.Sprintf(`用戶詢問："%s" 基於以下筆記請幫用戶想三個問題，這三個問題來幫助他學習，切記只需要有問題以及剪短解答即可，務必要用條列式的方式說明。筆記:%s
專注於這些筆記中最重要的概念或關係。讓問題足夠具體，對學習有用，每個問題之間希望你能空兩格。你的回覆只需要是 markdown 即可。`, query, notesContent.String())

	// Collect all content first, then parse
	var fullContent strings.Builder

	// Use streaming generation
	_, err := s.llm.GenerateContent(ctx, []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(prompt),
			},
		},
	}, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		chunkStr := string(chunk)
		fullContent.WriteString(chunkStr)
		// Send each chunk as it arrives
		s.sendChunk(responseChan, chunkStr)
		return nil
	}))

	if err != nil {
		s.sendError(responseChan, fmt.Sprintf("生成閃卡失敗: %v", err))
		return fmt.Errorf("failed to generate flashcard: %w", err)
	}

	s.sendStatus(responseChan, "parsing", "解析閃卡內容...", 90)

	// Parse the complete response
	flashcard, err := s.parseJSONFlashcardResponse(fullContent.String())
	if err != nil {
		s.sendError(responseChan, fmt.Sprintf("解析閃卡失敗: %v", err))
		return fmt.Errorf("failed to parse flashcard: %w", err)
	}

	flashcard.Tags = allTags
	s.sendComplete(responseChan, flashcard)
	s.sendStatus(responseChan, "completed", "閃卡生成完成！", 100)

	return nil
}

// GenerateFlashcardFromNotes generates a flashcard from multiple notes (non-streaming version)
func (s *FlashcardService) GenerateFlashcardFromNotes(ctx context.Context, notes []Note) (*Flashcard, error) {
	if len(notes) == 0 {
		return nil, fmt.Errorf("at least one note is required")
	}

	// Combine notes into a single context
	var notesContent strings.Builder
	var allTags []string
	tagSet := make(map[string]bool)

	for i, note := range notes {
		notesContent.WriteString(fmt.Sprintf("筆記 %d - %s:\n%s\n\n", i+1, note.Title, note.Content))

		// Collect unique tags
		for _, tag := range note.Tags {
			if !tagSet[tag] {
				allTags = append(allTags, tag)
				tagSet[tag] = true
			}
		}
	}

	prompt := fmt.Sprintf(`基於以下筆記請幫用戶想三個問題，這三個問題來幫助他學習，切記只需要有問題以及剪短解答即可，務必要用條列式的方式說明。筆記:%s
專注於這些筆記中最重要的概念或關係。讓問題足夠具體，對學習有用，每個問題之間希望你能空兩格。你的回覆只需要是 markdown 即可。`, notesContent.String())

	resp, err := s.llm.GenerateContent(ctx, []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(prompt),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate flashcard: %w", err)
	}

	// Parse JSON response
	flashcard, err := s.parseJSONFlashcardResponse(resp.Choices[0].Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse flashcard response: %w", err)
	}

	flashcard.Tags = allTags
	return flashcard, nil
}

// GenerateFlashcardFromQuery generates a flashcard based on a user query and related notes (non-streaming version)
func (s *FlashcardService) GenerateFlashcardFromQuery(ctx context.Context, query string, relatedNotes []Note) (*Flashcard, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if len(relatedNotes) == 0 {
		return nil, fmt.Errorf("no related notes found")
	}

	// Combine notes into context
	var notesContent strings.Builder
	var allTags []string
	tagSet := make(map[string]bool)

	for i, note := range relatedNotes {
		notesContent.WriteString(fmt.Sprintf("筆記 %d - %s:\n%s\n\n", i+1, note.Title, note.Content))

		// Collect unique tags
		for _, tag := range note.Tags {
			if !tagSet[tag] {
				allTags = append(allTags, tag)
				tagSet[tag] = true
			}
		}
	}

	prompt := fmt.Sprintf(`用戶詢問："%s"基於以下筆記請幫用戶想三個問題，這三個問題來幫助他學習，切記只需要有問題以及剪短解答即可，務必要用條列式的方式說明。筆記:%s
專注於這些筆記中最重要的概念或關係。讓問題足夠具體，對學習有用，每個問題之間希望你能空兩格。你的回覆只需要是 markdown 即可。`, query, notesContent.String())

	resp, err := s.llm.GenerateContent(ctx, []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(prompt),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate flashcard: %w", err)
	}

	// Parse JSON response
	flashcard, err := s.parseJSONFlashcardResponse(resp.Choices[0].Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse flashcard response: %w", err)
	}

	flashcard.Tags = allTags
	return flashcard, nil
}

// parseJSONFlashcardResponse parses the JSON response from LLM into a Flashcard struct
func (s *FlashcardService) parseJSONFlashcardResponse(content string) (*Flashcard, error) {
	// Clean the content - remove markdown code blocks if present
	content = strings.TrimSpace(content)
	if strings.TrimPrefix(content, "```json") != "" {
		content = strings.TrimPrefix(content, "```json")
	}
	if strings.TrimPrefix(content, "```") != "" {
		content = strings.TrimPrefix(content, "```")
	}
	if strings.TrimSuffix(content, "```") != "" {
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	var flashcard Flashcard
	if err := json.Unmarshal([]byte(content), &flashcard); err != nil {
		// If JSON parsing fails, try to extract from text format
		return s.parseTextFlashcardResponse(content), nil
	}

	// Set default values if empty
	if flashcard.Question == "" {
		flashcard.Question = "學習問題"
	}
	if flashcard.Answer == "" {
		flashcard.Answer = content // Use entire content as fallback
	}
	if flashcard.Difficulty == "" {
		flashcard.Difficulty = "Medium"
	}

	return &flashcard, nil
}

// parseTextFlashcardResponse parses text format response as fallback
func (s *FlashcardService) parseTextFlashcardResponse(content string) *Flashcard {
	flashcard := &Flashcard{
		Difficulty: "Medium",
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Question:") || strings.HasPrefix(line, "問題:") {
			flashcard.Question = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Question:"), "問題:"))
		} else if strings.HasPrefix(line, "Answer:") || strings.HasPrefix(line, "答案:") {
			flashcard.Answer = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Answer:"), "答案:"))
		} else if strings.HasPrefix(line, "Explanation:") || strings.HasPrefix(line, "解釋:") {
			flashcard.Explanation = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Explanation:"), "解釋:"))
		} else if strings.HasPrefix(line, "Difficulty:") || strings.HasPrefix(line, "難度:") {
			flashcard.Difficulty = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Difficulty:"), "難度:"))
		}
	}

	// Fallback values
	if flashcard.Question == "" {
		flashcard.Question = "學習問題"
	}
	if flashcard.Answer == "" {
		flashcard.Answer = content
	}

	return flashcard
}
