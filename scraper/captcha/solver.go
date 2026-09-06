package captcha

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	api2captcha "github.com/2captcha/2captcha-go"
)

var (
	lastSolve time.Time
)

type solveResponse struct {
	Text string `json:"text"`
	Ms   int    `json:"ms"`
}

// LocalCaptchaSolver sends captcha image to your local FastAPI server and returns the OCR result
func LocalCaptchaSolver(captchaImageData string, logger *log.Logger) (string, error) {
	// logger.Println("=== LOCAL CAPTCHA SOLVER ===")

	// strip prefix if needed
	base64Data := captchaImageData
	if strings.HasPrefix(captchaImageData, "data:image/") {
		parts := strings.Split(captchaImageData, ",")
		if len(parts) > 1 {
			base64Data = parts[1]
		}
	}

	// decode base64 to []byte (for multipart upload)
	imageBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// ---- build multipart request ----
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "captcha.png")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %v", err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		return "", fmt.Errorf("failed to write image bytes: %v", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %v", err)
	}

	// ---- send HTTP request ----
	req, err := http.NewRequest("POST", "http://127.0.0.1:8000/solve", &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to OCR server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OCR server error (%d): %s", resp.StatusCode, string(body))
	}

	// ---- parse response ----
	var sr solveResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("failed to parse OCR response: %v", err)
	}

	// logger.Printf("Captcha solved in %dms: '%s'\n", sr.Ms, sr.Text)
	return sr.Text, nil
}

var (
	captchaLock sync.Mutex
	// stdinLock serializes manual prompts so concurrent workers don't interleave
	// their prompts and read each other's typed input.
	stdinLock sync.Mutex
)

// ManualStdinCaptchaSolver saves the captcha image to a temp file, attempts to
// open it in the system image viewer, and blocks reading the answer from stdin.
// Use this for local testing when you want to fill captchas by hand instead of
// paying for / running an automated solver.
func ManualStdinCaptchaSolver(captchaImageData string, logger *log.Logger) (string, error) {
	base64Data := captchaImageData
	if strings.HasPrefix(captchaImageData, "data:image/") {
		parts := strings.Split(captchaImageData, ",")
		if len(parts) > 1 {
			base64Data = parts[1]
		}
	}

	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// Write to CAPTCHA_DIR if set (mount this to the host so you can view the
	// image without docker cp), otherwise fall back to the OS temp dir.
	outDir := os.Getenv("CAPTCHA_DIR")
	if outDir == "" {
		outDir = os.TempDir()
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create captcha dir %q: %v", outDir, err)
	}
	imagePath := filepath.Join(outDir, fmt.Sprintf("captcha_%d.png", time.Now().UnixNano()))
	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to write captcha image: %v", err)
	}
	defer os.Remove(imagePath)

	// Only one worker prompts at a time.
	stdinLock.Lock()
	defer stdinLock.Unlock()

	if err := openImage(imagePath); err != nil {
		logger.Printf("[captcha] could not auto-open image (view it at %s): %v", imagePath, err)
	}
	fmt.Printf("\n[captcha] image saved to: %s\n", imagePath)
	fmt.Print("[captcha] enter the text you see, then press Enter: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read captcha input: %v", err)
	}
	solution := strings.TrimSpace(input)
	if solution == "" {
		return "", fmt.Errorf("empty captcha solution provided")
	}
	return solution, nil
}

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

	APIKey := os.Getenv("APIKEY_2CAPTCHA")
	if APIKey == "" {
		return "", fmt.Errorf("APIKEY_2CAPTCHA environment variable not set")
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
