package captcha

// import (
// 	"fmt"
// 	"log"
// 	"os"

// 	"github.com/otiai10/gosseract/v2"
// )

// func main() {
// 	// Check if image path is provided
// 	if len(os.Args) < 2 {
// 		fmt.Println("Usage: go run main.go <image_path>")
// 		fmt.Println("Example: go run main.go sample.png")
// 		os.Exit(1)
// 	}

// 	imagePath := os.Args[1]

// 	// Check if file exists
// 	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
// 		log.Fatalf("Image file does not exist: %s", imagePath)
// 	}

// 	// Create a new Tesseract client
// 	client := gosseract.NewClient()
// 	defer client.Close()

// 	// Set image source
// 	err := client.SetImage(imagePath)
// 	if err != nil {
// 		log.Fatalf("Failed to set image: %v", err)
// 	}

// 	// Optional: Set language (default is English)
// 	// client.SetLanguage("eng")

// 	// Optional: Set page segmentation mode
// 	// PSM_SINGLE_BLOCK = 6 (default)
// 	// PSM_SINGLE_LINE = 7
// 	// PSM_SINGLE_WORD = 8
// 	// client.SetPageSegMode(gosseract.PSM_SINGLE_BLOCK)

// 	// Extract text from image
// 	text, err := client.Text()
// 	if err != nil {
// 		log.Fatalf("Failed to extract text: %v", err)
// 	}

// 	// Output the extracted text
// 	fmt.Println("Extracted Text:")
// 	fmt.Println("================")
// 	fmt.Println(text)

// 	// Optional: Get confidence score
// 	confidence, err := client.GetMeanConfidence()
// 	if err != nil {
// 		log.Printf("Warning: Could not get confidence score: %v", err)
// 	} else {
// 		fmt.Printf("\nConfidence Score: %.2f%%\n", confidence)
// 	}
// }

// // Alternative function to extract text with custom configuration
// func extractTextWithConfig(imagePath string, language string, psm gosseract.PageSegMode) (string, error) {
// 	client := gosseract.NewClient()
// 	defer client.Close()

// 	// Set image
// 	if err := client.SetImage(imagePath); err != nil {
// 		return "", fmt.Errorf("failed to set image: %w", err)
// 	}

// 	// Configure language
// 	if language != "" {
// 		client.SetLanguage(language)
// 	}

// 	// Configure page segmentation mode
// 	client.SetPageSegMode(psm)

// 	// Extract text
// 	text, err := client.Text()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to extract text: %w", err)
// 	}

// 	return text, nil
// }

// // Function to process multiple images
// func processMultipleImages(imagePaths []string) {
// 	for i, path := range imagePaths {
// 		fmt.Printf("\n--- Processing Image %d: %s ---\n", i+1, path)

// 		client := gosseract.NewClient()

// 		if err := client.SetImage(path); err != nil {
// 			fmt.Printf("Error processing %s: %v\n", path, err)
// 			client.Close()
// 			continue
// 		}

// 		text, err := client.Text()
// 		if err != nil {
// 			fmt.Printf("Error extracting text from %s: %v\n", path, err)
// 			client.Close()
// 			continue
// 		}

// 		fmt.Printf("Text from %s:\n%s\n", path, text)
// 		client.Close()
// 	}
// }
