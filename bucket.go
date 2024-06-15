package ninjacrawler

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gabriel-vasile/mimetype"
)

// uploadToBucket uploads a file to a Google Cloud Storage bucket.
func uploadToBucket(app *Crawler, sourceFileName, destinationFileName string) {
	startTime := time.Now()
	bucketName := "gen_crawled_data_venturas_asia-northeast1"
	destinationFileName = fmt.Sprintf("maker/%s/%s", app.Name, destinationFileName)
	googleApplicationCredentialsFileName := app.Config.EnvString("GCP_CREDENTIALS_PATH")

	if googleApplicationCredentialsFileName == "" {
		app.Logger.Error("GCP_CREDENTIALS_PATH environment variable is not set")
		return
	}

	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", googleApplicationCredentialsFileName); err != nil {
		app.Logger.Error("Failed to set GOOGLE_APPLICATION_CREDENTIALS: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := storage.NewClient(ctx)
	if err != nil {
		app.Logger.Error("Failed to create storage client: %v", err)
		return
	}
	defer func() {
		if err := client.Close(); err != nil {
			app.Logger.Error("Failed to close storage client: %v", err)
		}
	}()

	file, err := os.Open(sourceFileName)
	if err != nil {
		app.Logger.Error("Failed to open file %s: %v", sourceFileName, err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			app.Logger.Error("Failed to close file %s: %v", sourceFileName, err)
		}
	}()

	obj := client.Bucket(bucketName).Object(destinationFileName)
	writer := obj.NewWriter(ctx)

	contentType, err := detectContentType(sourceFileName)
	if err != nil {
		app.Logger.Error("Failed to detect content type for file %s: %v", sourceFileName, err)
		writer.ContentType = "application/octet-stream" // Default to binary stream if detection fails
	} else {
		writer.ContentType = contentType
	}

	if _, err := io.Copy(writer, file); err != nil {
		app.Logger.Error("Failed to copy file data to bucket %s: %v", bucketName, err)
		writer.Close() // Ensure writer is closed to release resources
		return
	}

	if err := writer.Close(); err != nil {
		app.Logger.Error("Failed to close writer for file %s: %v", destinationFileName, err)
		return
	}

	timeTaken := time.Since(startTime)
	app.Logger.Info("File %s uploaded to bucket successfully. Time taken: %s", sourceFileName, timeTaken)
}
func detectContentType(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Use mimetype.DetectFile to determine content type.
	mime, err := mimetype.DetectFile(filePath)
	if err != nil {
		return "", err
	}

	return mime.String(), nil
}
