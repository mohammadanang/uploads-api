package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/mohammadanang/uploads-api/domain"
)

type Handler interface {
	UploadFile(c *fiber.Ctx) error
	MergeChunks(c *fiber.Ctx) error
}

type ApiHandler struct{}

func NewAPIHandler() Handler {
	return &ApiHandler{}
}

func (h *ApiHandler) UploadFile(c *fiber.Ctx) error {
	// Ensure the uploads directory exists
	if _, err := os.Stat("./uploads"); os.IsNotExist(err) {
		// Create the uploads directory if it does not exist
		// This is necessary to avoid errors when saving uploaded files
		// os.MkdirAll creates a directory named path, along with any necessary parents,
		// and returns nil, or else returns an error.
		// os.ModePerm sets the permissions for the directory
		// to the default mode (read, write, and execute for owner, and read and execute for others).
		os.MkdirAll("./uploads", os.ModePerm)
	}

	// Ensure the temp directory exists
	if _, err := os.Stat("./temp"); os.IsNotExist(err) {
		// Create the temp directory if it does not exist
		// This directory can be used for temporary file storage during the upload process
		os.MkdirAll("./temp", os.ModePerm)
	}

	body := new(domain.UploadFileRequest)
	if err := c.BodyParser(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request data",
			"details": err.Error(),
		})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "File upload failed",
			"details": err.Error(),
		})
	}

	// Process the file (e.g., save it to disk or cloud storage)
	tempFile := filepath.Join("./temp", fmt.Sprintf("%s.part%d", file.Filename, body.ChunkIndex))
	// Create a temporary file to store the uploaded chunk
	outputFile, err := os.Create(tempFile)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create temporary file",
			"details": err.Error(),
		})
	}
	defer outputFile.Close()

	// Open the uploaded file
	fileReader, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to open uploaded file",
			"details": err.Error(),
		})
	}
	defer fileReader.Close()

	buf := make([]byte, 1*1024*1024) // 1 MB buffer
	// Copy the file content to the temporary file
	_, err = io.CopyBuffer(outputFile, fileReader, buf)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to write file chunk",
			"details": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error":   false,
		"message": "File uploaded successfully",
		"file":    file.Filename,
	})
}

func (h *ApiHandler) MergeChunks(c *fiber.Ctx) error {
	body := new(domain.MergeChunksRequest)
	if err := c.BodyParser(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request data",
			"details": err.Error(),
		})
	}

	outPath := filepath.Join("./uploads", body.FileName)
	// Create the output file where all chunks will be merged
	outputFile, err := os.Create(outPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create output file",
			"details": err.Error(),
		})
	}
	defer outputFile.Close()

	var mutx sync.Mutex
	var wg sync.WaitGroup
	for i := range body.TotalChunks {
		wg.Add(1)

		// Use a goroutine to handle each chunk
		// This allows concurrent processing of chunks, which can speed up the merging process
		// Each goroutine will read a chunk file and write its content to the output file
		// The chunk files are named in the format "filename.partX" where X is the chunk index
		// The mutex is used to ensure that only one goroutine writes to the output file at a time
		// This prevents data corruption or other issues that could occur if multiple goroutines try to write to the file at the same time
		go func(chunkIndex int) {
			defer wg.Done()
			chunkPath := filepath.Join("./temp", fmt.Sprintf("%s.part%d", body.FileName, chunkIndex))
			// Open the temporary file for the chunk
			chunkFile, err := os.Open(chunkPath)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("Chunk %d does not exist, %v\n", chunkIndex, err.Error())
					return
				}

				fmt.Printf("Failed to open chunk %d: %v\n", chunkIndex, err)
				return
			}
			defer chunkFile.Close()

			chunkData, err := io.ReadAll(chunkFile)
			if err != nil {
				fmt.Printf("Failed to read chunk %d: %v\n", chunkIndex, err)
				return
			}

			// Lock the mutex to ensure only one goroutine writes to the output file at a time
			mutx.Lock()
			defer mutx.Unlock()

			_, err = outputFile.Write(chunkData)
			if err != nil {
				fmt.Printf("Failed to write chunk %d to output file: %v\n", chunkIndex, err)
				return
			}

			// Optionally, you can remove the chunk file after merging
			os.Remove(chunkPath)
		}(i)
	}
	wg.Wait()

	if err := cleanUpTempFiles(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to clean up temporary files",
			"details": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error":   false,
		"message": "Chunks merged successfully",
	})
}

func cleanUpTempFiles() error {
	if _, err := os.Stat("./temp"); os.IsNotExist(err) {
		return nil // No temp directory to clean up
	}

	files, err := filepath.Glob("./temp/*.part*")
	if err != nil {
		return fmt.Errorf("failed to list temp files: %w", err)
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("failed to remove temp file %s: %w", file, err)
		}
	}

	return nil
}
