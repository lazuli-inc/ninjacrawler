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
	bucketName := "gen_crawled_data_venturas_asia-northeast1"
	googleApplicationCredentialsFileName := app.Config.EnvString("GCP_CREDENTIALS_PATH")

	err := UploadToGCPBucket(app.Name, googleApplicationCredentialsFileName, sourceFileName, destinationFileName)
	if err != nil {
		app.Logger.Error("Failed to upload file %s to bucket %s: %v", sourceFileName, bucketName, err)
		return
	}
}
func UploadToGCPBucket(dirName, GCP_CREDENTIALS_PATH, sourceFileName, destinationFileName string) error {
	bucketName := "gen_crawled_data_venturas_asia-northeast1"
	destinationFileName = fmt.Sprintf("maker/%s/%s", dirName, destinationFileName)

	if GCP_CREDENTIALS_PATH == "" {
		return fmt.Errorf("GCP_CREDENTIALS_PATH environment variable is not set")
	}

	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", GCP_CREDENTIALS_PATH); err != nil {
		return fmt.Errorf("Failed to set GOOGLE_APPLICATION_CREDENTIALS: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("Failed to create storage client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Println("Failed to close storage client: %v", err)
		}
	}()

	file, err := os.Open(sourceFileName)
	if err != nil {
		return fmt.Errorf("Failed to open file %s: %v", sourceFileName, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Println("Failed to close file %s: %v", sourceFileName, err)
		}
	}()

	obj := client.Bucket(bucketName).Object(destinationFileName)
	writer := obj.NewWriter(ctx)

	contentType, err := detectContentType(sourceFileName)
	if err != nil {
		fmt.Println("Failed to detect content type for file %s: %v", sourceFileName, err)
		writer.ContentType = "application/octet-stream" // Default to binary stream if detection fails
	} else {
		writer.ContentType = contentType
	}

	if _, err := io.Copy(writer, file); err != nil {
		writer.Close() // Ensure writer is closed to release resources
		return fmt.Errorf("Failed to copy file data to bucket %s: %v", bucketName, err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("Failed to close writer for file %s: %v", destinationFileName, err)
	}

	return nil
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
