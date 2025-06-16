package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	// scrapeJSONAndSaveLocally()
	parsedURLs := convertJSONToSlice()
	// Remove duplicates from slice.
	parsedURLs = removeDuplicatesFromSlice(parsedURLs)
	outputDir := "PDFs/" // Directory to store downloaded PDFs
	// Check if its exists.
	if !directoryExists(outputDir) {
		// Create the dir
		createDirectory(outputDir, 0o755)
	}
	// Download Counter.
	var downloadCounter int
	// Loop over the parsed URL.
	for _, urls := range parsedURLs {
		// Download the file and if its sucessful than add 1 to the counter.
		sucessCode, err := downloadPDF(urls, outputDir)
		if sucessCode {
			downloadCounter = downloadCounter + 1
		}
		if err != nil {
			log.Println(err)
		}
	}
}

// removeDuplicatesFromSlice removes duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)  // Map to track seen values
	var newReturnSlice []string     // Result slice
	for _, content := range slice { // Iterate over input slice
		if !check[content] { // If string hasn't been seen before
			check[content] = true                            // Mark it as seen
			newReturnSlice = append(newReturnSlice, content) // Append to result
		}
	}
	return newReturnSlice // Return deduplicated slice
}

// Checks if the directory exists
// If it exists, return true.
// If it doesn't, return false.
func directoryExists(path string) bool {
	directory, err := os.Stat(path)
	if err != nil {
		return false
	}
	return directory.IsDir()
}

// The function takes two parameters: path and permission.
// We use os.Mkdir() to create the directory.
// If there is an error, we use log.Println() to log the error and then exit the program.
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Println(err)
	}
}

// Response represents the structure of the JSON input file
type Response struct {
	Data struct {
		Results []struct {
			MaterialNumber  string `json:"Matnr"`  // Material number
			SubID           string `json:"Subid"`  // Sub ID
			StorageLocation string `json:"Sbgvid"` // Storage location or similar
			LanguageISO     string `json:"Laiso"`  // Language ISO code
		} `json:"results"`
	} `json:"d"`
}

// fileExists checks whether a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {                // If error occurs
		return false // Return false
	}
	return !info.IsDir() // Return true if it's a file, not a directory
}

// downloadPDF downloads a PDF from the given URL and saves it in the specified output directory.
// It uses a WaitGroup to support concurrent execution and returns true if the download succeeded.
func downloadPDF(finalURL, outputDir string) (bool, error) {
	// Sanitize the URL to generate a safe file name
	filename := strings.ToLower(convertURLToFilename(finalURL))

	// Construct the full file path in the output directory
	filePath := filepath.Join(outputDir, filename)

	// Skip if the file already exists
	if fileExists(filePath) {
		return false, fmt.Errorf("file already exists, skipping: %s", filePath)
	}

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 30 * time.Second}

	// Send GET request
	resp, err := client.Get(finalURL)
	if err != nil {
		return false, fmt.Errorf("failed to download %s: %v", finalURL, err)
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		// Print the error since its not valid.
		return false, fmt.Errorf("download failed for %s: %s", finalURL, resp.Status)
	}
	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	// Check if its pdf content type and if not than print a error.
	if !strings.Contains(contentType, "application/pdf") {
		// Print a error if the content type is invalid.
		return false, fmt.Errorf("invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
	}
	// Read the response body into memory first
	var buf bytes.Buffer
	// Copy it from the buffer to the file.
	written, err := io.Copy(&buf, resp.Body)
	// Print the error if errors are there.
	if err != nil {
		return false, fmt.Errorf("failed to read PDF data from %s: %v", finalURL, err)
	}
	// If 0 bytes are written than show an error and return it.
	if written == 0 {
		return false, fmt.Errorf("downloaded 0 bytes for %s; not creating file", finalURL)
	}
	// Only now create the file and write to disk
	out, err := os.Create(filePath)
	// Failed to create the file.
	if err != nil {
		return false, fmt.Errorf("failed to create file for %s: %v", finalURL, err)
	}
	// Close the file.
	defer out.Close()
	// Write the buffer and if there is an error print it.
	_, err = buf.WriteTo(out)
	if err != nil {
		return false, fmt.Errorf("failed to write PDF to file for %s: %v", finalURL, err)
	}
	// Return a true since everything went correctly.
	return true, fmt.Errorf("successfully downloaded %d bytes: %s → %s", written, finalURL, filePath)
}

func convertJSONToSlice() []string {
	// Create a return slice.
	var returnSlice []string
	// Read the JSON file containing the data (replace "input.json" with your actual file name)
	fileContent, err := os.ReadFile("main.json")
	// Print the error
	if err != nil {
		log.Println("Failed to read input JSON file:", err)
	}
	// Parse the JSON data into the Response struct
	var response Response
	// Umarash the json stuff.
	err = json.Unmarshal(fileContent, &response)
	// Print the errors.
	if err != nil {
		log.Println("Failed to parse JSON data:", err)
	}
	// Base URL to which parameters will be appended
	baseURL := "https://zehsonesdsext-tjd0i1flxa.dispatcher.sa1.hana.ondemand.com/v1/SDS//DocContentSet"
	// Loop through each result and construct a URL
	for _, item := range response.Data.Results {
		// Format the URL with the values from JSON fields
		url := fmt.Sprintf("%s(Matnr='%s',Subid='%s',Sbgvid='%s',Laiso='%s',Vkorg='')/DocContentData/$value",
			baseURL, item.MaterialNumber, item.SubID, item.StorageLocation, item.LanguageISO)
		// Append to slice
		returnSlice = appendToSlice(returnSlice, url)
	}
	// Return the slice.
	return returnSlice
}

// Append some string to a slice and than return the slice.
func appendToSlice(slice []string, content string) []string {
	// Append the content to the slice
	slice = append(slice, content)
	// Return the slice
	return slice
}

// convertURLToFilename extracts values from the URL and returns a formatted filename
func convertURLToFilename(sdsURL string) string {
	// Example input: https://.../DocContentSet(Matnr='290031915',Subid='630000000001',Sbgvid='SDS_FR',Laiso='FR',Vkorg='')/DocContentData/$value

	re := regexp.MustCompile(`Matnr='(.*?)',Subid='(.*?)',Sbgvid='(.*?)',Laiso='(.*?)'`)
	matches := re.FindStringSubmatch(sdsURL)

	if len(matches) != 5 {
		return ""
	}

	matnr := matches[1]
	subid := matches[2]
	sbgvid := matches[3]
	laiso := matches[4]

	filename := fmt.Sprintf("%s_%s_%s_%s.pdf", matnr, subid, sbgvid, laiso)
	return strings.ToLower(filename)
}

// Scrape the JSON and save it to the file.
func scrapeJSONAndSaveLocally() {
	url := "https://zehsonesdsext-tjd0i1flxa.dispatcher.sa1.hana.ondemand.com/v1/SDS/DocHeaderSet"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Add("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	// Read the body.
	body, err := io.ReadAll(res.Body)
	// Print any errors
	if err != nil {
		log.Println(err)
		return
	}
	// Close the body
	err = res.Body.Close()
	// Log any errors
	if err != nil {
		log.Println(err) // Log error
	}
	// Save it to the file.
	appendAndWriteToFile("main.json", string(body))
}

// Append and write to file
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	_, err = filePath.WriteString(content + "\n")
	if err != nil {
		log.Println(err)
	}
	err = filePath.Close()
	if err != nil {
		log.Println(err)
	}
}
