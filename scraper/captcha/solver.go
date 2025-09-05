package captcha

import (
	"bufio"
	"encoding/base64"
	"path/filepath"
	"time"
	// "encoding/base64"
	"fmt"
	"os"
	"os/exec"

	// "path/filepath"
	"runtime"
	"strings"
)

// ManualCaptchaSolver displays the captcha image to the user and prompts for input
func ManualCaptchaSolver(captchaImageData string) (string, error) {
	// func ManualCaptchaSolver(imagePath string) (string, error) {
	fmt.Println("=== CAPTCHA SOLVER ===")

	// Extract base64 data (remove data:image/png;base64, prefix if present)
	base64Data := captchaImageData
	fmt.Println("Base64 Data:", base64Data)
	if strings.HasPrefix(captchaImageData, "data:image/") {
		parts := strings.Split(captchaImageData, ",")
		if len(parts) > 1 {
			base64Data = parts[1]
		}
	}

	// // Decode base64 image data
	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// // Create temporary file for the image
	tmpDir := os.TempDir()
	timestamp := time.Now().Unix()
	imagePath := filepath.Join(tmpDir, fmt.Sprintf("captcha_%d.png", timestamp))

	// // Write image data to file
	file, err := os.Create(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to create image file: %v", err)
	}
	defer file.Close()

	if _, err := file.Write(imageData); err != nil {
		return "", fmt.Errorf("failed to write image data: %v", err)
	}

	fmt.Printf("Captcha image saved to: %s\n", imagePath)

	// Try to open the image automatically
	if err := openImage(imagePath); err != nil {
		fmt.Printf("Could not open image automatically: %v\n", err)
		fmt.Printf("Please manually open: %s\n", imagePath)
	} else {
		fmt.Println("Captcha image opened in default viewer.")
	}

	// Prompt user for captcha input
	fmt.Print("\nPlease look at the captcha image and enter the text you see: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %v", err)
	}

	// Clean up the input
	solution := strings.TrimSpace(input)

	// Clean up temporary file
	defer func() {
		if removeErr := os.Remove(imagePath); removeErr != nil {
			fmt.Printf("Warning: Could not remove temporary file %s: %v\n", imagePath, removeErr)
		}
	}()

	if solution == "" {
		return "", fmt.Errorf("empty captcha solution provided")
	}

	fmt.Printf("Captcha solution received: '%s'\n", solution)
	return solution, nil
}

// openImage attempts to open the image using the system's default image viewer
func openImage(imagePath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Windows: use start command
		cmd = exec.Command("cmd", "/c", "start", imagePath)
	case "darwin":
		// macOS: use open command
		cmd = exec.Command("open", imagePath)
	case "linux":
		// Linux: try xdg-open first, then common image viewers
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", imagePath)
		} else if _, err := exec.LookPath("eog"); err == nil {
			// GNOME Image Viewer
			cmd = exec.Command("eog", imagePath)
		} else if _, err := exec.LookPath("feh"); err == nil {
			// Lightweight image viewer
			cmd = exec.Command("feh", imagePath)
		} else if _, err := exec.LookPath("display"); err == nil {
			// ImageMagick display
			cmd = exec.Command("display", imagePath)
		} else {
			return fmt.Errorf("no suitable image viewer found")
		}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Start the command without waiting for it to finish
	return cmd.Start()
}

// ValidateCaptchaInput performs basic validation on captcha input
func ValidateCaptchaInput(input string) bool {
	// Remove whitespace
	input = strings.TrimSpace(input)

	// Check if empty
	if input == "" {
		return false
	}

	// Check length (most captchas are 4-8 characters)
	if len(input) < 3 || len(input) > 10 {
		fmt.Printf("Warning: Captcha length (%d) seems unusual. Are you sure?\n", len(input))
	}

	return true
}

// GetCaptchaWithRetry allows the user to retry if they make a mistake
func GetCaptchaWithRetry(captchaImageData string, maxRetries int) (string, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("\n=== Captcha Attempt %d of %d ===\n", attempt, maxRetries)

		solution, err := ManualCaptchaSolver(captchaImageData)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		if !ValidateCaptchaInput(solution) {
			fmt.Println("Invalid input. Please try again.")
			continue
		}

		// Ask for confirmation
		fmt.Printf("You entered: '%s'. Is this correct? (y/n): ", solution)
		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.ToLower(strings.TrimSpace(confirm))

		if confirm == "y" || confirm == "yes" {
			return solution, nil
		}

		if attempt < maxRetries {
			fmt.Println("Let's try again...")
		}
	}

	return "", fmt.Errorf("exceeded maximum retry attempts (%d)", maxRetries)
}
