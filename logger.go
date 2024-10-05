package ninjacrawler

import (
	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/logging"
	"context"
	"fmt"
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
	Summary(format string, args ...interface{})
	Html(page playwright.Page, message string)
}

// defaultLogger is a default implementation of the logger interface using the standard log package.
type defaultLogger struct {
	logger         *log.Logger
	app            *Crawler
	gcpLogger      *logging.Logger // GCP Summary logger
	gcpDebugLogger *logging.Logger // GCP Debug logger
}

// newDefaultLogger creates a new instance of defaultLogger.
func newDefaultLogger(app *Crawler, siteName string) *defaultLogger {
	logFile, err := os.OpenFile(getLogFileName(siteName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	multiWriter := io.MultiWriter(logFile, os.Stdout)

	// Create the default logger
	dLogger := &defaultLogger{
		logger: log.New(multiWriter, "【"+app.Name+"】", log.LstdFlags),
		app:    app,
	}

	// Initialize GCP logger if requested
	if metadata.OnGCE() {
		dLogger.gcpLogger = getGCPLogger(app.Config, "summary_log")
		dLogger.gcpDebugLogger = getGCPLogger(app.Config, "dev_log")
	}

	return dLogger

}
func getLogFileName(siteName string) string {
	currentDate := time.Now().Format("2006-01-02")
	directory := filepath.Join("storage", "logs", siteName)

	if err := os.MkdirAll(directory, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	// Return the log file path with the formatted current date.
	return filepath.Join(directory, fmt.Sprintf("%s_application.log", currentDate))
}
func getGCPLogger(config *configService, logID string) *logging.Logger {

	os.Setenv("GCP_LOG_CREDENTIALS_PATH", "log-key.json")
	projectID := config.EnvString("PROJECT_ID", "lazuli-venturas-stg")

	client, err := logging.NewClient(context.Background(), projectID)
	if err != nil {
		panic(fmt.Sprintf("Failed to create GCP logging client: %v", err))
	}

	return client.Logger(logID)
}

// logWithGCP logs both to the local logger
func (l *defaultLogger) logWithGCP(level string, format string, args ...interface{}) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)

	// Log to local logger
	l.logger.Printf("☁️ "+format, args...)

	if l.gcpLogger != nil && level == "summary" {
		l.gcpLogger.Log(logging.Entry{
			Payload: map[string]interface{}{
				"level":     "info",
				"caller":    "ninjacrawler/logger.go",
				"ts":        ts,
				"msg":       msg,
				"site_name": l.app.Name,
			},
			Severity: logging.Info,
		})
		// Flush GCP logger to ensure the log is sent immediately
		defer l.gcpLogger.Flush()
	}
	if l.gcpDebugLogger != nil && level == "debug" {
		l.gcpDebugLogger.Log(logging.Entry{
			Payload: map[string]interface{}{
				"level":     "error",
				"caller":    "ninjacrawler/logger.go",
				"ts":        ts,
				"msg":       msg,
				"site_name": l.app.Name,
			},
			Severity: logging.Debug,
		})
		// Flush GCP logger to ensure the log is sent immediately
		defer l.gcpDebugLogger.Flush()
	}
}
func (l *defaultLogger) Summary(format string, args ...interface{}) {
	l.logWithGCP("summary", format, args...)
}
func (l *defaultLogger) Debug(format string, args ...interface{}) {
	l.logWithGCP("debug", format, args...)
}
func (l *defaultLogger) Info(format string, args ...interface{}) {
	l.logger.Printf("✔ "+format, args...)
}
func (l *defaultLogger) Warn(format string, args ...interface{}) {
	l.logger.Printf("⚠️ "+format, args...)
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

func (l *defaultLogger) Html(html, url, msg string, dir ...string) {
	if l.app.IsValidPage(url) {
		err := l.app.writePageContentToFile(html, url, msg, dir...)
		if err != nil {
			l.logger.Printf("⚛️ HTML: %v", err)
		}
	}
}
