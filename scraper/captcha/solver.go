package captcha

import (
	"encoding/base64"
	"log"
	"path/filepath"
	"sync"
	"time"

	// "encoding/base64"
	"fmt"
	"os"
	"os/exec"

	// "path/filepath"
	"runtime"
	"strings"

	api2captcha "github.com/2captcha/2captcha-go"
)

var (
	captchaLock sync.Mutex
	lastSolve   time.Time
)

// ManualCaptchaSolver displays the captcha image to the user and prompts for input
func ManualCaptchaSolver(captchaImageData string, logger *log.Logger) (string, error) {
	logger.Println("=== CAPTCHA SOLVER ===")

	// Extract base64 data (remove data:image/png;base64, prefix if present)
	base64Data := captchaImageData
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

	// Create temporary file for the image
	tmpDir := os.TempDir()
	timestamp := time.Now().Unix()
	imagePath := filepath.Join(tmpDir, fmt.Sprintf("captcha_%d.png", timestamp))

	// // Write image data to file
	file, err := os.Create(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to create image file: %v", err)
	}
	// Clean up temporary file
	defer func() {
		if removeErr := os.Remove(imagePath); removeErr != nil {
			logger.Printf("Warning: Could not remove temporary file %s: %v\n", imagePath, removeErr)
		}
	}()
	defer file.Close()

	if _, err := file.Write(imageData); err != nil {
		return "", fmt.Errorf("failed to write image data: %v", err)
	}

	logger.Printf("Captcha image saved to: %s\n", imagePath)

	// Try to open the image automatically
	// if err := openImage(imagePath); err != nil {
	// 	fmt.Printf("Could not open image automatically: %v\n", err)
	// 	fmt.Printf("Please manually open: %s\n", imagePath)
	// } else {
	// 	fmt.Println("Captcha image opened in default viewer.")
	// }

	// Prompt user for captcha input
	// fmt.Print("\nPlease look at the captcha image and enter the text you see: ")

	// reader := bufio.NewReader(os.Stdin)
	// input, err := reader.ReadString('\n')
	// if err != nil {
	// 	return "", fmt.Errorf("failed to read user input: %v", err)
	// }

	APIKey := os.Getenv("CAPTCHA_API_KEY")
	if APIKey == "" {
		return "", fmt.Errorf("CAPTCHA_API_KEY environment variable not set")
	}

	client := api2captcha.NewClient(APIKey)

	cap := api2captcha.Normal{
		File: imagePath,
	}

	captchaLock.Lock()
	wait := time.Duration(0)
	if !lastSolve.IsZero() {
		elapsed := time.Since(lastSolve)
		if elapsed < 5*time.Second {
			wait = 5*time.Second - elapsed
		}
	}
	captchaLock.Unlock()

	if wait > 0 {
		time.Sleep(wait)
	}

	captchaLock.Lock()
	lastSolve = time.Now()
	captchaLock.Unlock()

	logger.Println("Sending captcha to API...")
	solution, _, err := client.Solve(cap.ToRequest())
	if err != nil {
		switch err {
		case api2captcha.ErrTimeout:
			return "", fmt.Errorf("[captcha] API Timeout")
		case api2captcha.ErrApi:
			return "", fmt.Errorf("[captcha] API error")
		case api2captcha.ErrNetwork:
			return "", fmt.Errorf("[captcha] Network error")
		default:
			return "", fmt.Errorf("[captcha] Unknown error: %v", err)
		}
	}

	logger.Println("Captcha solution received:", solution)

	if solution == "" {
		return "", fmt.Errorf("empty captcha solution provided")
	}

	logger.Printf("Captcha solution received: '%s'\n", solution)
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
