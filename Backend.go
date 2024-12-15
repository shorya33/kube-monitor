package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define Prometheus metrics
var (
	totalUploads = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "s3_file_uploads_total",
			Help: "Total number of files uploaded to S3",
		},
		[]string{"bucket_name", "region"},
	)

	uploadDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "s3_file_upload_duration_seconds",
			Help:    "Duration of S3 file uploads in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(totalUploads)
	prometheus.MustRegister(uploadDuration)
}

// S3FileUploader handles file upload to S3
func S3FileUploader(bucketName, key, region string, file io.Reader) error {
	start := time.Now()
	// Load AWS configuration with the provided region
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	// Create an S3 client
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

	// Record metrics
	duration := time.Since(start).Seconds()
	uploadDuration.Observe(duration)
	totalUploads.WithLabelValues(bucketName, region).Inc()

	return nil
}

// UploadHandler handles the file upload HTTP request
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Parse form data (with a 10MB limit)
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, "Unable to parse form data", http.StatusBadRequest)
			return
		}

		// Extract file and form values
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

		// Get the file name to use as the S3 key
		fileName := r.FormValue("fileName")
		if fileName == "" {
			http.Error(w, "File name is required", http.StatusBadRequest)
			return
		}

		// Upload the file to S3
		err = S3FileUploader(bucketName, fileName, region, file)
		if err != nil {
			http.Error(w, fmt.Sprintf("Upload failed: %v", err), http.StatusInternalServerError)
			return
		}

		// Respond with a success message
		fmt.Fprintf(w, "File uploaded successfully to S3 bucket %s in region %s", bucketName, region)
		return
	}

	http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
}

func main() {
	// Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// File upload endpoint
	http.HandleFunc("/upload", UploadHandler)

	// Start the server on port 8080
	log.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
