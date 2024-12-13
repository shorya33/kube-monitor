package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// UploadFileToS3 uploads a file to the specified S3 bucket
func UploadFileToS3(bucketName, key string, file *os.File) error {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	// Upload the file to S3
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &bucketName,
		Key:    &key,
		Body:   file,
		ACL:    types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return nil
}

// UploadHandler handles the file upload request
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Parse form data
		err := r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			http.Error(w, "Unable to parse form data", http.StatusBadRequest)
			return
		}

		// Get the file, bucket name, and region from the form
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Unable to retrieve the file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		bucketName := r.FormValue("bucketName")
		if bucketName == "" {
			http.Error(w, "Bucket name is required", http.StatusBadRequest)
			return
		}

		region := r.FormValue("region")
		if region == "" {
			http.Error(w, "Region is required", http.StatusBadRequest)
			return
		}

		// Load AWS configuration with the provided region
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
		if err != nil {
			http.Error(w, fmt.Sprintf("Unable to load AWS configuration for region %s: %v", region, err), http.StatusInternalServerError)
			return
		}

		s3Client := s3.NewFromConfig(cfg)

		// Get the file name and upload to S3
		fileName := r.FormValue("fileName")
		if fileName == "" {
			http.Error(w, "File name is required", http.StatusBadRequest)
			return
		}

		// Create a temporary file to store the uploaded file
		tempFile, err := os.CreateTemp("", "uploaded-*")
		if err != nil {
			http.Error(w, "Unable to create temporary file", http.StatusInternalServerError)
			return
		}
		defer os.Remove(tempFile.Name())

		// Copy the uploaded file to the temporary file
		_, err = io.Copy(tempFile, file)
		if err != nil {
			http.Error(w, "Failed to copy file", http.StatusInternalServerError)
			return
		}

		// Close the temporary file after copying
		err = tempFile.Close()
		if err != nil {
			http.Error(w, "Failed to close the temporary file", http.StatusInternalServerError)
			return
		}

		// Upload the file to S3
		err = UploadFileToS3(bucketName, fileName, tempFile)
		if err != nil {
			http.Error(w, fmt.Sprintf("Upload failed: %v", err), http.StatusInternalServerError)
			return
		}

		// Respond with success message
		fmt.Fprintf(w, "File uploaded successfully to S3 bucket %s in region %s", bucketName, region)
		return
	}

	http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
}

func main() {
	http.HandleFunc("/upload", UploadHandler)

	// Start the server on port 8080
	log.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
