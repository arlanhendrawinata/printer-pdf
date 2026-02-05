package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PrintSettings struct {
	PaperSize    string
	Color        string
	DoubleSided  bool
	DuplexMode   string
	Copies       int
}

type PrinterStatus struct {
	Name        string
	Status      string
	JobsInQueue int
	IsReady     bool
	HasPaper    bool
	HasError    bool
	ErrorMsg    string
}

func main() {
	pdfFile := "test.pdf"
	printerName := "MP230"

	// 1. Print SATU SISI (paling umum)
	// settings := PrintSettings{
	// 	PaperSize:   "a4",
	// 	Color:       "color",
	// 	DoubleSided: false,    // ← Print 1 sisi aja
	// 	Copies:      1,
	// }

	// 2. Print BOLAK-BALIK vertikal (buku/majalah)
	// settings := PrintSettings{
	// 	PaperSize:   "a4",
	// 	Color:       "monochrome",
	// 	DoubleSided: true,         // ← Print bolak-balik
	// 	DuplexMode:  "vertical",   // ← Flip vertikal (long edge)
	// 	Copies:      1,
	// }

	// 3. Print BOLAK-BALIK horizontal (kalender)
	// settings := PrintSettings{
	// 	PaperSize:   "a4",
	// 	Color:       "color",
	// 	DoubleSided: true,          // ← Print bolak-balik
	// 	DuplexMode:  "horizontal",  // ← Flip horizontal (short edge)
	// 	Copies:      2,
	// }

	settings := PrintSettings{
		PaperSize:   "a4",
		Color:       "color",
		DoubleSided: false,
		DuplexMode:  "vertical",
		Copies:      1,
	}

	fmt.Println("🖨️  Checking printer status...")
	
	// Cek status printer sebelum print
	status, err := getPrinterStatus(printerName)
	if err != nil {
		fmt.Printf("❌ Error checking printer: %v\n", err)
		return
	}

	displayStatus(status)

	// Cek apakah printer ready
	if !status.IsReady {
		fmt.Println("\n⚠️  Printer tidak ready! Cek printer dulu.")
		return
	}

	if status.HasError {
		fmt.Printf("\n⚠️  Printer error: %s\n", status.ErrorMsg)
		return
	}

	// Cek file ada
	if _, err := os.Stat(pdfFile); os.IsNotExist(err) {
		fmt.Println("\n❌ File test.pdf tidak ditemukan!")
		return
	}

	fmt.Println("\n📄 Starting print job...")

	// Print PDF
	fullPath, _ := filepath.Abs(pdfFile)
	gsPath := findGhostscript()
	if gsPath == "" {
		fmt.Println("❌ Ghostscript tidak ditemukan!")
		return
	}

	args := buildGSArgs(printerName, fullPath, settings)
	cmd := exec.Command(gsPath, args...)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		fmt.Printf("❌ Print error: %v\n", err)
		fmt.Printf("📝 Detail: %s\n", string(output))
		return
	}

	fmt.Println("✅ Print command sent!")

	// Monitor print job
	fmt.Println("\n📊 Monitoring print job...")
	monitorPrintJob(printerName, 0) // Monitor selama 30 detik
}

func getPrinterStatus(printerName string) (*PrinterStatus, error) {
	// PowerShell command untuk ambil status printer
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

	// Parse output
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

func monitorPrintJob(printerName string, timeoutSec int) {
	startTime := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	previousJobs := -1
	paperWarningShown := false  // Track agar warning gak spam

	for {
		select {
		case <-ticker.C:
			status, err := getPrinterStatus(printerName)
			if err != nil {
				fmt.Printf("⚠️  Error: %v\n", err)
				return
			}

			if status.JobsInQueue == 0 {
				fmt.Println("✅ Print job selesai!")
				return
			}

			// CEK PAPER OUT
			if !status.HasPaper {
				if !paperWarningShown {
					fmt.Println("\n❌ KERTAS HABIS!")
					fmt.Println("📄 Silakan isi kertas, print akan otomatis lanjut...")
					paperWarningShown = true
				}
			} else {
				// Reset flag kalau kertas udah diisi
				if paperWarningShown {
					fmt.Println("✅ Kertas terdeteksi! Print dilanjutkan...")
					paperWarningShown = false
				}
			}

			if status.HasError {
				fmt.Printf("❌ Printer error: %s\n", status.ErrorMsg)
				return
			}

			// Tampilkan progress kalau ada perubahan
			if status.JobsInQueue != previousJobs {
				fmt.Printf("⏳ Jobs in queue: %d - Status: %s (%.0fs elapsed)\n", 
					status.JobsInQueue, status.Status, time.Since(startTime).Seconds())
				previousJobs = status.JobsInQueue
			}

			if timeoutSec > 0 && time.Since(startTime) > time.Duration(timeoutSec)*time.Second {
				fmt.Printf("⏱️  Timeout setelah %d detik - print job masih berjalan\n", timeoutSec)
				fmt.Println("💡 Tip: Tingkatkan timeout atau set 0 untuk unlimited")
				return
			}
		}
	}
}

func displayStatus(status *PrinterStatus) {
	fmt.Printf("\n📋 Printer: %s\n", status.Name)
	fmt.Printf("📊 Status: %s\n", status.Status)
	fmt.Printf("📑 Jobs in queue: %d\n", status.JobsInQueue)
	
	if status.IsReady {
		fmt.Println("✅ Printer ready")
	} else {
		fmt.Println("⚠️  Printer not ready")
	}
	
	if !status.HasPaper {
		fmt.Println("⚠️  Kertas habis/masalah!")
	}
	
	if status.HasError {
		fmt.Printf("❌ Error: %s\n", status.ErrorMsg)
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