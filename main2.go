package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

type PrintSettings struct {
	PaperSize    string `json:"paper_size"`    // a4, letter, legal, a5
	Color        string `json:"color"`         // color, monochrome
	DoubleSided  bool   `json:"double_sided"`  // true/false
	DuplexMode   string `json:"duplex_mode"`   // vertical, horizontal
	Copies       int    `json:"copies"`        // jumlah copy
}

type PrintRequest struct {
	FileName string        `json:"file_name"` // nama file di project folder
	Printer  string        `json:"printer"`   // nama printer
	Settings PrintSettings `json:"settings"`
}

type PrintResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	JobID   string `json:"job_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

type PrinterStatus struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	JobsInQueue int    `json:"jobs_in_queue"`
	IsReady     bool   `json:"is_ready"`
	HasPaper    bool   `json:"has_paper"`
	HasError    bool   `json:"has_error"`
	ErrorMsg    string `json:"error_msg,omitempty"`
}

func main() {
	app := fiber.New(fiber.Config{
		AppName: "Printer API v1.0",
	})

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Routes
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Printer API is running",
			"version": "1.0",
		})
	})

	// Print PDF
	app.Post("/print", handlePrint)

	// Get printer status
	app.Get("/printer/status/:name", handlePrinterStatus)

	// List available files
	app.Get("/files", handleListFiles)

	// Start server
	fmt.Println("🖨️  Printer API starting...")
	fmt.Println("📡 Server running on http://localhost:3000")
	fmt.Println("📋 Endpoints:")
	fmt.Println("   POST   /print")
	fmt.Println("   GET    /printer/status/:name")
	fmt.Println("   GET    /files")
	
	app.Listen(":3000")
}

func handlePrint(c *fiber.Ctx) error {
	req := new(PrintRequest)

	// Parse request body
	if err := c.BodyParser(req); err != nil {
		return c.Status(400).JSON(PrintResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	// Set default values
	if req.Settings.PaperSize == "" {
		req.Settings.PaperSize = "a4"
	}
	if req.Settings.Color == "" {
		req.Settings.Color = "color"
	}
	if req.Settings.Copies <= 0 {
		req.Settings.Copies = 1
	}
	if req.Printer == "" {
		req.Printer = "MP230" // Default printer
	}

	// Check file exists
	if _, err := os.Stat(req.FileName); os.IsNotExist(err) {
		return c.Status(404).JSON(PrintResponse{
			Success: false,
			Error:   fmt.Sprintf("File not found: %s", req.FileName),
		})
	}

	// Get printer status first
	status, err := getPrinterStatus(req.Printer)
	if err != nil {
		return c.Status(500).JSON(PrintResponse{
			Success: false,
			Error:   fmt.Sprintf("Printer not found: %s", req.Printer),
		})
	}

	if !status.IsReady {
		return c.Status(503).JSON(PrintResponse{
			Success: false,
			Error:   fmt.Sprintf("Printer not ready: %s", status.Status),
		})
	}

	// Get full path
	fullPath, _ := filepath.Abs(req.FileName)

	// Find Ghostscript
	gsPath := findGhostscript()
	if gsPath == "" {
		return c.Status(500).JSON(PrintResponse{
			Success: false,
			Error:   "Ghostscript not found",
		})
	}

	// Build Ghostscript arguments
	args := buildGSArgs(req.Printer, fullPath, req.Settings)

	// Execute print
	cmd := exec.Command(gsPath, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return c.Status(500).JSON(PrintResponse{
			Success: false,
			Error:   fmt.Sprintf("Print failed: %v - %s", err, string(output)),
		})
	}

	return c.JSON(PrintResponse{
		Success: true,
		Message: "Print job sent successfully",
		JobID:   fmt.Sprintf("job_%d", time.Now().Unix()),
	})
}

func handlePrinterStatus(c *fiber.Ctx) error {
	printerName := c.Params("name")

	status, err := getPrinterStatus(printerName)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    status,
	})
}

func handleListFiles(c *fiber.Ctx) error {
	// List PDF files in current directory
	files, err := filepath.Glob("*.pdf")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to list files",
		})
	}

	fileList := []fiber.Map{}
	for _, file := range files {
		info, _ := os.Stat(file)
		fileList = append(fileList, fiber.Map{
			"name":     file,
			"size":     info.Size(),
			"modified": info.ModTime(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"count":   len(fileList),
		"files":   fileList,
	})
}

func getPrinterStatus(printerName string) (*PrinterStatus, error) {
	psScript := fmt.Sprintf(`
		$printer = Get-Printer -Name "%s" -ErrorAction SilentlyContinue
		if ($printer) {
			$status = $printer.PrinterStatus
			$queue = Get-PrintJob -PrinterName "%s" -ErrorAction SilentlyContinue
			$jobCount = if ($queue) { ($queue | Measure-Object).Count } else { 0 }
			
			Write-Host "STATUS:$status"
			Write-Host "JOBS:$jobCount"
			Write-Host "NAME:$($printer.Name)"
		} else {
			Write-Error "Printer not found"
			exit 1
		}
	`, printerName, printerName)

	cmd := exec.Command("powershell", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("printer tidak ditemukan")
	}

	status := &PrinterStatus{
		Name: printerName,
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "STATUS:") {
			statusCode := strings.TrimPrefix(line, "STATUS:")
			status.Status = parseStatusCode(statusCode)
			status.IsReady = strings.Contains(statusCode, "Normal") || statusCode == "0"
			status.HasPaper = !strings.Contains(strings.ToLower(statusCode), "paper")
			status.HasError = strings.Contains(strings.ToLower(statusCode), "error")
		}
		if strings.HasPrefix(line, "JOBS:") {
			fmt.Sscanf(line, "JOBS:%d", &status.JobsInQueue)
		}
	}

	return status, nil
}

func parseStatusCode(code string) string {
	code = strings.TrimSpace(code)
	switch code {
	case "0", "Normal":
		return "Ready"
	case "1":
		return "Paused"
	case "2":
		return "Error"
	case "3":
		return "Pending Deletion"
	case "4":
		return "Paper Jam"
	case "5":
		return "Paper Out"
	case "6":
		return "Manual Feed"
	case "7":
		return "Paper Problem"
	case "8":
		return "Offline"
	default:
		if strings.Contains(strings.ToLower(code), "paper") {
			return "Paper Problem"
		}
		if strings.Contains(strings.ToLower(code), "error") {
			return "Error"
		}
		return code
	}
}

func findGhostscript() string {
	gsPaths := []string{
		"C:\\Program Files\\gs\\gs10.06.0\\bin\\gswin64c.exe",
		"C:\\Program Files (x86)\\gs\\gs10.06.0\\bin\\gswin32c.exe",
		"gswin64c.exe",
	}

	for _, path := range gsPaths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func buildGSArgs(printerName, pdfPath string, settings PrintSettings) []string {
	args := []string{
		"-dPrinted",
		"-dBATCH",
		"-dNOPAUSE",
		"-dNOSAFER",
		"-q",
	}

	args = append(args, fmt.Sprintf("-dNumCopies=%d", settings.Copies))

	switch settings.PaperSize {
	case "a4":
		args = append(args, "-sPAPERSIZE=a4")
	case "letter":
		args = append(args, "-sPAPERSIZE=letter")
	case "legal":
		args = append(args, "-sPAPERSIZE=legal")
	case "a5":
		args = append(args, "-sPAPERSIZE=a5")
	default:
		args = append(args, "-sPAPERSIZE=a4")
	}

	if settings.Color == "monochrome" {
		args = append(args, "-sProcessColorModel=DeviceGray")
		args = append(args, "-sColorConversionStrategy=Gray")
		args = append(args, "-dOverrideICC")
	}

	if settings.DoubleSided {
		args = append(args, "-dDuplex=true")
		if settings.DuplexMode == "horizontal" {
			args = append(args, "-dTumble=true")
		} else {
			args = append(args, "-dTumble=false")
		}
	} else {
		args = append(args, "-dDuplex=false")
	}

	args = append(args, "-sDEVICE=mswinpr2")
	args = append(args, fmt.Sprintf("-sOutputFile=%%printer%%%s", printerName))
	args = append(args, pdfPath)

	return args
}