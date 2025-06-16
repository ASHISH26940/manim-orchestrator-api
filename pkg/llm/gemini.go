// pkg/llm/gemini.go

package llm

import (
	"context"
	"fmt"
	"strings" // New import for string manipulation

	"github.com/google/generative-ai-go/genai"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

// Service holds the Gemini AI client.
type Service struct {
	client *genai.GenerativeModel
	ctx    context.Context // Context for API calls
}

// NewGeminiService creates a new Gemini AI service instance.
func NewGeminiService(apiKey string) (*Service, error) {
	ctx := context.Background() // Use a background context for the service
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	// Use the 'gemini-pro' model for text generation
	model := client.GenerativeModel("gemini-1.5-flash")
	return &Service{client: model, ctx: ctx}, nil
}

// // DecomposePrompt takes a complex user prompt and uses Gemini to break it down
// // into a JSON array of simpler, independent animation descriptions.
// // Each description in the array is expected to be a self-contained unit.
// func (s *Service) DecomposePrompt(complexPrompt string) ([]string, error) {
// 	log.Debugf("Attempting to decompose complex prompt: %s", complexPrompt)

// 	// Construct the prompt for Gemini. It's crucial to instruct it to return JSON.
// 	decompositionPrompt := fmt.Sprintf(`
// 	You are an expert Manim animation designer.
// 	Decompose the following complex Manim animation request into an ordered JSON array of simple, self-contained Manim animation descriptions.
// 	Each description should be a single string that can be used to generate a small, complete Manim animation segment.
// 	Ensure the entire response is a valid JSON array of strings, with no additional text or formatting outside the array.

// 	Example Request: "Animate a red square fading in, then a blue circle transforms into a green triangle, and finally, a text 'The End' appears."
// 	Example Response: ["Animate a red square fading in.", "A blue circle transforms into a green triangle.", "Display the text 'The End'."]

// 	Complex animation request to decompose: "%s"
// 	`, complexPrompt)

// 	resp, err := s.client.GenerateContent(s.ctx, genai.Text(decompositionPrompt))
// 	if err != nil {
// 		log.Errorf("Error generating content for decomposition: %v", err)
// 		return nil, fmt.Errorf("gemini API call failed during decomposition: %w", err)
// 	}

// 	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
// 		log.Warn("Gemini returned no candidates or content for decomposition.")
// 		return nil, fmt.Errorf("gemini API returned no content for decomposition")
// 	}

// 	// Extract the text response
// 	geminiResponsePart := resp.Candidates[0].Content.Parts[0]
// 	geminiResponse, ok := geminiResponsePart.(genai.Text)
// 	if !ok {
// 		log.Errorf("Gemini response part is not text: %v", geminiResponsePart)
// 		return nil, fmt.Errorf("gemini API returned non-text content for decomposition")
// 	}

// 	responseString := string(geminiResponse)
// 	log.Debugf("Gemini raw decomposition response: %s", responseString)

// 	// Attempt to parse the JSON array
// 	var decomposedPrompts []string
// 	// Gemini sometimes includes markdown fences (```json ... ```).
// 	// We need to strip them to ensure valid JSON unmarshaling.
// 	cleanResponse := strings.TrimSpace(responseString)
// 	if strings.HasPrefix(cleanResponse, "```json") && strings.HasSuffix(cleanResponse, "```") {
// 		cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
// 		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
// 		cleanResponse = strings.TrimSpace(cleanResponse)
// 	} else if strings.HasPrefix(cleanResponse, "```") && strings.HasSuffix(cleanResponse, "```") { // Less specific markdown fence
// 		cleanResponse = strings.TrimPrefix(cleanResponse, "```")
// 		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
// 		cleanResponse = strings.TrimSpace(cleanResponse)
// 	}


// 	err = json.Unmarshal([]byte(cleanResponse), &decomposedPrompts)
// 	if err != nil {
// 		log.Errorf("Failed to unmarshal Gemini decomposition response '%s': %v", cleanResponse, err)
// 		return nil, fmt.Errorf("failed to parse decomposition JSON from Gemini: %w", err)
// 	}

// 	log.Infof("Successfully decomposed prompt into %d parts.", len(decomposedPrompts))
// 	return decomposedPrompts, nil
// }

// GenerateManimCode takes a simple animation description and uses Gemini to generate
// the corresponding Manim Python code.
// This method's core logic remains the same, but it will now be called for each
// decomposed sub-prompt by the handler.
func (s *Service) GenerateManimCode(prompt string) (string, error) {
	log.Debugf("Attempting to generate Manim code for prompt: %s", prompt)

promptTemplate := `Generate complete and valid Manim Python code for the animation described in the user request.

### Pre-computation and Reasoning Steps (Internal):
1.  **Analyze and Deconstruct**: First, thoroughly analyze the user request to identify all explicit and implicit visual elements (Mobjects), animations, durations, colors, positions, and relationships between elements.
2.  **Object Identification**: Extract all specific Manim Mobject types mentioned or implied (e.g., Circle, Square, Text, Line, Arc, Equation, Graph).
3.  **Animation Mapping**: Map identified actions/verbs from the request to appropriate Manim animation functions (e.g., "create" -> Create, "show" -> FadeIn, "move" -> Transform/MoveTo, "rotate" -> Rotate). Consider natural animation types for each object.
4.  **Property Extraction**: Identify all specified properties for each object and animation (e.g., color, size, radius, fill_opacity, stroke_width, duration, speed). Pay close attention to hex codes or standard Manim colors.
5.  **Scene Flow Planning**: Determine the sequential flow of animations. If multiple actions are implied concurrently, consider [self.play(anim1, anim2)]. If sequential, use separate [self.play()] calls followed by [self.wait()].
6.  **Conflict Resolution**: If there are conflicting instructions (e.g., "make it red and blue simultaneously"), prioritize explicit color requests over general descriptions. If an animation style contradicts an object's inherent property, prioritize the animation style for that specific [self.play()] call, but retain the object's base properties for subsequent animations. If ambiguity persists, default to a sensible visual choice.
7.  **Ambiguity Handling**: If the request is truly ambiguous, nonsensical, or too complex to reasonably fulfill given Manim's capabilities or the prompt's constraints, default to the simple fallback animation as per "Strict Requirements #7".

### Strict Requirements for Output:
1.  **Code Only**: Provide ONLY the Python code. Do NOT include any explanations, external comments (other than standard Manim class/method docstrings or very brief line-level comments for complex logic), or conversational text.
2.  **Self-Contained Class**: The entire animation logic must be within a single class that inherits from 'Scene'.
3.  **Specific Class Name**: The main animation class MUST be named 'MyScene'.
4.  **Colors (Hex Codes)**: When using colors, define them using hex codes (e.g., '#FF0000' for red, '#0000FF' for blue) or standard Manim color constants (e.g., RED, BLUE, WHITE, BLACK, YELLOW, GREEN). If a specific color is requested and a standard constant doesn't exist, use a suitable hex code.
5.  **Scene Progression**: Every animation sequence MUST include at least one 'self.play()' call, which should then be followed by a 'self.wait(1)' or 'self.wait(duration)' for scene progression.
6.  **Imports**: Include all necessary Manim imports at the top (e.g., 'from manim import *').
7.  **Error Handling**: If the user request is ambiguous, nonsensical, or too complex to reasonably fulfill, output a simple default animation (e.g., a fading square or circle) instead.

### Example 1:
Input: "create a square"
Output:
` + "\nfrom manim import *\n\nclass MyScene(Scene):\n    def construct(self):\n        square = Square(color=RED)\n        self.play(FadeIn(square))\n        self.wait(1)\n" + `

### Example 2:
Input: "Create a flower using circles. It should have a yellow center and pink petals. Also, add a green stem and a leaf."
Output:
` + "\nfrom manim import *\n\nclass MyScene(Scene):\n    def construct(self):\n        center_circle = Circle(radius=0.5, color=YELLOW, fill_opacity=1)\n        self.play(Create(center_circle))\n        self.wait(0.5)\n\n        petal_color = PINK\n        petal_radius = 0.4\n        num_petals = 8\n\n        petals = VGroup()\n\n        for i in range(num_petals):\n            angle = i * (2 * PI / num_petals)\n            x = (center_circle.radius + petal_radius * 0.8) * np.cos(angle)\n            y = (center_circle.radius + petal_radius * 0.8) * np.sin(angle)\n            \n            petal = Circle(radius=petal_radius, color=petal_color, fill_opacity=0.7)\n            petal.move_to(np.array([x, y, 0]))\n            petals.add(petal)\n\n        self.play(LaggedStart(*[GrowFromCenter(petal) for petal in petals], lag_ratio=0.15))\n        self.wait(1)\n\n        stem = Line(center_circle.get_bottom(), center_circle.get_bottom() + DOWN * 2, color=GREEN, stroke_width=8)\n        \n        leaf = Polygon(\n            stem.get_end() + LEFT * 0.5 + UP * 0.5,\n            stem.get_end() + LEFT * 1.5 + UP * 0.2,\n            stem.get_end() + LEFT * 0.5 + DOWN * 0.2,\n            color=GREEN, fill_opacity=0.8\n        )\n        leaf.rotate(PI/4, about_point=stem.get_end() + LEFT * 0.5 + UP * 0.2)\n\n        self.play(\n            Create(stem),\n            FadeIn(leaf, shift=RIGHT)\n        )\n        self.wait(2)\n" + `

### User Request:
"%s"`

	manimCodePrompt := fmt.Sprintf(promptTemplate, prompt)

	resp, err := s.client.GenerateContent(s.ctx, genai.Text(manimCodePrompt))
	if err != nil {
		log.Errorf("Error generating content for Manim code: %v", err)
		return "", fmt.Errorf("gemini API call failed during code generation: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		log.Warn("Gemini returned no candidates or content for Manim code generation.")
		return "", fmt.Errorf("gemini API returned no content for Manim code generation")
	}

	manimCodePart := resp.Candidates[0].Content.Parts[0]
	manimCode, ok := manimCodePart.(genai.Text)
	if !ok {
		log.Errorf("Gemini response part is not text for Manim code: %v", manimCodePart)
		return "", fmt.Errorf("gemini API returned non-text content for Manim code generation")
	}

	responseString := string(manimCode)
	log.Debugf("Gemini raw Manim code response: %s", responseString)

	// Clean up potential markdown code fences from Gemini's response
	// This is important as Gemini often wraps code in triple backticks.
	cleanedCode := strings.TrimSpace(responseString)
	if strings.HasPrefix(cleanedCode, "```python") && strings.HasSuffix(cleanedCode, "```") {
		cleanedCode = strings.TrimPrefix(cleanedCode, "```python")
		cleanedCode = strings.TrimSuffix(cleanedCode, "```")
		cleanedCode = strings.TrimSpace(cleanedCode)
	} else if strings.HasPrefix(cleanedCode, "```") && strings.HasSuffix(cleanedCode, "```") { // Less specific markdown fence
		cleanedCode = strings.TrimPrefix(cleanedCode, "```")
		cleanedCode = strings.TrimSuffix(cleanedCode, "```")
		cleanedCode = strings.TrimSpace(cleanedCode)
	}

	log.Infof("Successfully generated Manim code for prompt: %s", prompt)
	return cleanedCode, nil
}

// Close gracefully closes the underlying Gemini client.
// This should be called when your application is shutting down to release resources.
func (s *Service) Close() error {
	log.Info("Closing Gemini AI service client.")
	if s.client != nil {
		log.Warn("No explicit `Close()` method available for `*genai.GenerativeModel`. Resource cleanup is handled by Go's garbage collector.")
	}
	return nil
}