package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	ownerFlag       string
	repoFlag        string
	issueNumberFlag string
)

type File struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// GitHub issue/PR struct
type Issue struct {
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	User      User      `json:"user"`
	DateTime  time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GitHub comment struct
type Comment struct {
	Body     string    `json:"body"`
	User     User      `json:"user"`
	DateTime time.Time `json:"created_at"`
}

// GitHub user struct
type User struct {
	Login string `json:"login"`
}

func init() {
	flag.StringVar(&ownerFlag, "O", "", "Repository owner")
	flag.StringVar(&ownerFlag, "owner", "", "Repository owner")

	flag.StringVar(&repoFlag, "R", "", "Repository name")
	flag.StringVar(&repoFlag, "repo", "", "Repository name")

	flag.StringVar(&issueNumberFlag, "I", "", "Reference number of the issue or PR")
	flag.StringVar(&issueNumberFlag, "issueNumber", "", "Reference number of the issue or PR")
}

func main() {
	// GitHub API endpoint to fetch issue/PR and comments
	apiURL := "https://api.github.com/repos/{owner}/{repo}/issues/{issueNumber}"

	// Parse command-line flags
	flag.Parse()

	// Get the absolute path to github-tree-inputs.txt
	inputsFilePath := getAbsolutePath("github-comments-fetcher-inputs.txt")

	// Initialize variables to store inputs
	var currentOwner string
	var currentRepo string
	var accessToken string
	var currentIssueNumber string

	// Check if github-comments-fetcher-inputs.txt exists
	_, err := os.Stat(inputsFilePath)

	if err == nil {
		// The file exists, so read existing inputs from the file
		currentOwner, currentRepo, currentIssueNumber = readInputsFromFile(inputsFilePath)

		// Check if the owner and repo fields are empty
		if currentOwner == "" || currentRepo == "" {
			panic("The 'owner' and 'repo' fields in github-comments-fetcher-inputs.txt cannot be empty")
		}

		// Update inputs if flags were provided
		if ownerFlag != "" {
			currentOwner = ownerFlag
		}
		if repoFlag != "" {
			currentRepo = repoFlag
		}
		if issueNumberFlag != "" {
			currentIssueNumber = issueNumberFlag
		}

		// Update the inputs in the file
		updateInputsInFile(inputsFilePath, currentOwner, currentRepo, currentIssueNumber)
	} else {
		// The "github-comments-fetcher-inputs.txt" doesn't exist, so create it

		// Create a new inputs struct
		newInputs := struct {
			Owner       string `json:"owner"`
			Repo        string `json:"repo"`
			IssueNumber string `json:"issueNumber"`
		}{
			Owner:       ownerFlag,
			Repo:        repoFlag,
			IssueNumber: issueNumberFlag,
		}

		// Convert to JSON
		newInputsJSON, err := json.MarshalIndent(newInputs, "", "  ")
		if err != nil {
			panic(fmt.Errorf("failed to marshal new inputs: %w", err))
		}

		// Write to the file
		err = os.WriteFile(inputsFilePath, newInputsJSON, 0644)
		if err != nil {
			panic(fmt.Errorf("failed to write new inputs to file: %w", err))
		}
	}

	// Retrieve access token from environment
	accessToken = os.Getenv("GITHUB_ACCESS_TOKEN")
	if accessToken == "" {
		panic("GitHub access token not found in environment")
	}

	// GitHub repository information
	owner := currentOwner
	repo := currentRepo
	issueNumber := currentIssueNumber // Replace with the issue or PR number you want to fetch

	// Create the HTTP client
	client := &http.Client{}

	// Create the request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %s", err)
	}

	// Add the access token to the request header (optional)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	// Replace the placeholders in the API URL with the repository and issue information
	req.URL.Path = fmt.Sprintf("/repos/%s/%s/issues/%s", owner, repo, issueNumber)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to send request: %s", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %s", err)
	}

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Request failed with status: %s", resp.Status)
	}

	// Parse the response body as an Issue
	var issue Issue
	err = json.Unmarshal(body, &issue)
	if err != nil {
		log.Fatalf("Failed to parse issue response body: %s", err)
	}

	// Create or open the output file
	file, err := os.Create("comments.txt")
	if err != nil {
		log.Fatalf("Failed to create file: %s", err)
	}
	defer file.Close()

	// Write the issue details to the file
	issueLine := fmt.Sprintf("Issue Title: %s\nIssue Body: %s\nIssue Author: %s\nCreated At: %s\nUpdated At: %s\n\n",
		issue.Title, issue.Body, issue.User.Login, issue.DateTime.Format("2006-01-02 15:04:05"), issue.UpdatedAt.Format("2006-01-02 15:04:05"))
	_, err = file.WriteString(issueLine)
	if err != nil {
		log.Fatalf("Failed to write issue details to file: %s", err)
	}

	// Fetch comments
	apiCommentsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%s/comments", owner, repo, issueNumber)

	reqComments, err := http.NewRequest("GET", apiCommentsURL, nil)
	if err != nil {
		log.Fatalf("Failed to create comments request: %s", err)
	}

	if accessToken != "" {
		reqComments.Header.Set("Authorization", "Bearer "+accessToken)
	}

	respComments, err := client.Do(reqComments)
	if err != nil {
		log.Fatalf("Failed to send comments request: %s", err)
	}
	defer respComments.Body.Close()

	bodyComments, err := io.ReadAll(respComments.Body)
	if err != nil {
		log.Fatalf("Failed to read comments response body: %s", err)
	}

	if respComments.StatusCode != http.StatusOK {
		log.Fatalf("Comments request failed with status: %s", respComments.Status)
	}

	var comments []Comment
	err = json.Unmarshal(bodyComments, &comments)
	if err != nil {
		log.Fatalf("Failed to parse comments response body: %s", err)
	}

	// Write the comments to the file
	for i, comment := range comments {
		if i > 0 {
			_, err = file.WriteString("\n") // Leave two-line space between comment blocks
			if err != nil {
				log.Fatalf("Failed to write space to file: %s", err)
			}
		}

		commentHeader := fmt.Sprintf("Comment %d by %s at %s", i+1, comment.User.Login, comment.DateTime.Format("2006-01-02 15:04:05"))

		_, err = file.WriteString(commentHeader + ":\n")
		if err != nil {
			log.Fatalf("Failed to write comment header to file: %s", err)
		}

		commentBody := fmt.Sprintf("%s\n", comment.Body)
		_, err = file.WriteString(commentBody)
		if err != nil {
			log.Fatalf("Failed to write comment body to file: %s", err)
		}
	}

	fmt.Println("Issue details and comments have been fetched and saved to comments.txt.")
}

func readInputsFromFile(filePath string) (owner, repo, issueNumber string) {
	// Read the contents of the file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		panic(fmt.Errorf("failed to read inputs from file: %w", err))
	}

	// Unmarshal the JSON data into a struct
	var inputs struct {
		Owner       string `json:"owner"`
		Repo        string `json:"repo"`
		IssueNumber string `json:"issueNumber"`
	}
	err = json.Unmarshal(fileData, &inputs)
	if err != nil {
		panic(fmt.Errorf("failed to parse inputs from file: %w", err))
	}

	return inputs.Owner, inputs.Repo, inputs.IssueNumber
}

func getAbsolutePath(filePath string) string {
	currentDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("failed to get current directory: %w", err))
	}

	return filepath.Join(currentDir, filePath)
}

func updateInputsInFile(filePath, owner, repo, issueNumber string) {
	// Create the new inputs struct
	newInputs := struct {
		Owner       string `json:"owner"`
		Repo        string `json:"repo"`
		IssueNumber string `json:"issueNumber"`
	}{
		Owner:       owner,
		Repo:        repo,
		IssueNumber: issueNumber,
	}

	// Convert to JSON
	newInputsJSON, err := json.MarshalIndent(newInputs, "", "  ")
	if err != nil {
		panic(fmt.Errorf("failed to marshal new inputs: %w", err))
	}

	// Write to the file
	err = os.WriteFile(filePath, newInputsJSON, 0644)
	if err != nil {
		panic(fmt.Errorf("failed to write updated inputs to file: %w", err))
	}
}
