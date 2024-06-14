package ninjacrawler

import (
	"github.com/playwright-community/playwright-go"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// logger is an interface for logging.
type logger interface {
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
	Fatal(format string, args ...interface{})
	Printf(format string, args ...interface{})
	Html(page playwright.Page, message string)
}

// defaultLogger is a default implementation of the logger interface using the standard log package.
type defaultLogger struct {
	logger *log.Logger
}

// newDefaultLogger creates a new instance of defaultLogger.
func newDefaultLogger(siteName string) *defaultLogger {
	// Open a log file in append mode, create if it doesn't exist.

	currentDate := time.Now().Format("2006-01-02")
	directory := filepath.Join("storage", "logs", siteName)
	err := os.MkdirAll(directory, 0755)
	if err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// Construct the log file path.
	logFilePath := filepath.Join(directory, currentDate+"_application.log")

	// Open the log file in append mode, create if it doesn't exist.
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	// Create a multi-writer that writes to both the file and the terminal.
	multiWriter := io.MultiWriter(file, os.Stdout)

	return &defaultLogger{
		logger: log.New(multiWriter, "⏱️ ", log.LstdFlags),
	}
}

func (l *defaultLogger) Info(format string, args ...interface{}) {
	l.logger.Printf("📢 INFO: "+format, args...)
}

func (l *defaultLogger) Error(format string, args ...interface{}) {
	l.logger.Printf("🛑 ERROR: "+format, args...)
}

func (l *defaultLogger) Fatal(format string, args ...interface{}) {
	l.logger.Fatalf("🚨 FATAL: "+format, args...)
}

func (l *defaultLogger) Printf(format string, args ...interface{}) {
	l.logger.Printf(format, args...)
}

func (l *defaultLogger) Html(page playwright.Page, msg string) {
	l.Error(msg)
	err := writePageContentToFile(page, msg)
	if err != nil {
		l.logger.Printf("⚛️ HTML: %v", err)
	}
}
