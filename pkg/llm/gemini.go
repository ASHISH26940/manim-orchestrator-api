// pkg/llm/gemini.go

package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/config" // To get the API key
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	log "github.com/sirupsen/logrus"
)

// LLMClient represents a client for interacting with the LLM.
type LLMClient struct {
	client *genai.Client // Store the top-level client
	model  *genai.GenerativeModel
}

// NewLLMClient initializes a new Gemini LLM client.
func NewLLMClient(cfg *config.Config) (*LLMClient, error) {
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("Gemini API key is not provided")
	}

	ctx := context.Background()
	
	// Create the top-level client
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Get the GenerativeModel from the client
	model := client.GenerativeModel("gemini-1.5-flash")

	return &LLMClient{client: client, model: model}, nil // Store both
}

// Close closes the underlying Gemini client.
func (c *LLMClient) Close() {
	if c.client != nil { // Call Close() on the top-level client
		c.client.Close()
	}
	// No need to close c.model as it doesn't have a Close() method
}

// GenerateManimCode takes a user prompt and returns generated Manim Python code.
func (c *LLMClient) GenerateManimCode(userPrompt string) (string, error) {
	ctx := context.Background()

	// This is where you craft your prompt to guide the LLM.
	// Good prompt engineering is key to getting relevant Manim code.
	//
	// Instructions for the LLM:
	// 1. You are an expert Manim animation code generator.
	// 2. ONLY provide valid Python code using the Manim library.
	// 3. Do NOT include any explanations, comments (unless within code), or extra text outside the code block.
	// 4. Ensure the code is self-contained and runnable.
	// 5. The scene class must inherit from `Scene`.
	// 6. Use `self.play()` for animations and `self.wait()` for pauses.
	// 7. Make the animation simple but illustrative of the prompt.
	// 8. If the prompt is too complex or ambiguous, generate a simple, safe default Manim animation.
	// 9. IMPORTANT: Wrap the entire generated Manim Python code in a single Markdown code block (```python ... ```).
	//
    promptTemplate := `Generate Manim Python code based on this request.

Instructions:
- Provide ONLY valid, runnable Manim Python code.
- No explanations, external comments, or extra text.
- Code must be self-contained in a class inheriting from 'Scene'.
- Use 'self.play()' for animations and 'self.wait()' for pauses.
- For complex/unclear requests, output a simple default animation.

Example Input: "Animate a blue circle fading in."
Example Output:
` + "\nfrom manim import *\n\nclass MyAnimation(Scene):\n    def construct(self):\n        circle = Circle(color=BLUE)\n        self.play(FadeIn(circle))\n        self.wait(1)\n" + `

User request: "%s"`

    fullPrompt := fmt.Sprintf(promptTemplate, userPrompt)
    
    log.WithFields(log.Fields{"user_prompt": userPrompt}).Info("Sending prompt to Gemini LLM.")

	// Use c.model directly as it's already configured from the client
    resp, err := c.model.GenerateContent(ctx, genai.Text(fullPrompt))
    if err != nil {
    	return "", fmt.Errorf("failed to generate content from Gemini: %w", err)
    }

    if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
    	log.Warn("Gemini returned no candidates or empty content.")
    	return "", fmt.Errorf("Gemini returned no valid content for the prompt")
    }

    // Extract text from the first part of the first candidate's content
    generatedText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
    
    log.WithFields(log.Fields{"generated_text_length": len(generatedText)}).Debug("Received response from Gemini.")

    // The LLM is instructed to wrap the code in ```python ... ```
    // We need to extract only the code part.
    codeBlockStart := "```python\n"
    codeBlockEnd := "```"

    startIndex := strings.Index(generatedText, codeBlockStart)
    if startIndex == -1 {
        log.Warnf("Could not find start of Python code block in LLM response: %s", generatedText)
        return "", fmt.Errorf("LLM response did not contain a valid Python code block marker")
    }
    
    // Adjust start index to skip the marker itself
    startIndex += len(codeBlockStart)

    endIndex := strings.LastIndex(generatedText, codeBlockEnd)
    if endIndex == -1 || endIndex < startIndex {
        log.Warnf("Could not find end of Python code block in LLM response: %s", generatedText)
        return "", fmt.Errorf("LLM response did not contain a valid closing code block marker")
    }

    manimCode := generatedText[startIndex:endIndex]
    manimCode = strings.TrimSpace(manimCode) // Trim any leading/trailing whitespace

    log.WithFields(log.Fields{"extracted_code_length": len(manimCode)}).Info("Manim code extracted successfully from LLM response.")
    return manimCode, nil
}