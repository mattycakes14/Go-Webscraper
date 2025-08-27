package main

import (
	"fmt"
	"context"
	"net/http"
	"time"
	"strings"
	"github.com/PuerkitoBio/goquery"
	"sync"
	"github.com/chromedp/chromedp"
	"io/ioutil"
	"bytes"
	"os"
	"github.com/joho/godotenv"
	"encoding/json"
)

// define a struct to store the results
type Result struct {
	URL string
	Summary string
	Error error
}

// struct for the web content
type WebContent struct {
	Title string
	Headings string
	Paragraphs string
}

// Notion API structures
type NotionPageRequest struct {
	Parent     NotionParent     `json:"parent"`
	Properties NotionProperties `json:"properties"`
	Children   []NotionBlock    `json:"children,omitempty"`
}

type NotionParent struct {
	Type       string `json:"type"`
	PageID     string `json:"page_id,omitempty"`
	DatabaseID string `json:"database_id,omitempty"`
}

type NotionProperties struct {
	Title NotionTitle `json:"title"`
}

type NotionTitle struct {
	Title []NotionText `json:"title"`
}

type NotionText struct {
	Text NotionTextContent `json:"text"`
}

type NotionTextContent struct {
	Content string `json:"content"`
}

type NotionBlock struct {
	Object     string           `json:"object"`
	Type       string           `json:"type"`
	Paragraph  NotionParagraph  `json:"paragraph,omitempty"`
}

type NotionParagraph struct {
	RichText []NotionText `json:"rich_text"`
}

// worker is a function that fetches the title of a given URL
func worker(ctx context.Context, id int, jobs <-chan string, result chan<- Result, wg *sync.WaitGroup) {
	// decrement the wait group
	defer wg.Done()
	for url := range jobs {
		webContent, err := scrapeAboveFold(ctx, url)
		summary, err := summarizeContent(webContent)
		if err != nil {
			result <- Result{URL: url, Summary: "", Error: err}
		} else {
			result <- Result{URL: url, Summary: summary, Error: nil}
		}
	}
}

// scrapeAboveFold is a function that fetches the title of a given URL
// it takes a context and a url as arguments
// it returns a string and an error
func scrapeAboveFold(ctx context.Context, url string) (WebContent, error) {

	// create a HTTP server that cancels request after 30 sec
	client := &http.Client{Timeout: 30 * time.Second}

	// make a request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return WebContent{}, err
	}

	// set the header
	req.Header.Set("User-Agent", "go-scraper")

	res, err := client.Do(req)
	if err != nil {
		return WebContent{}, err
	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return WebContent{}, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// parses the stream of bytes into a goquery document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	
	if err != nil {
		return WebContent{}, err
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	headings := []string{}
	paragraphs := []string{}
	
	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		headings = append(headings, strings.TrimSpace(s.Text()))
	})
	
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		paragraphs = append(paragraphs, strings.TrimSpace(s.Text()))
	})

	// convert into string
	headingsString := strings.Join(headings, "\n")
	paragraphsString := strings.Join(paragraphs, "\n")

	if title == "" || err != nil {
		return scrapeWithChromedp(ctx, url)
	}else{
		return WebContent{
			Title: title,
			Headings: headingsString,
			Paragraphs: paragraphsString,
		}, nil
	}
}

// scrapeWithChromedp is a function that fetches the title of a given URL
// it takes a context and a url as arguments
// it returns a string and an error
func scrapeWithChromedp(ctx context.Context, url string) (WebContent, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	headings := []string{}
	paragraphs := []string{}
	
	var title string
    err := chromedp.Run(ctx,
        chromedp.Navigate(url),
        chromedp.Text("title", &title, chromedp.NodeVisible),
        chromedp.Evaluate(`
            Array.from(document.querySelectorAll('h1, h2, h3, h4, h5, h6'))
                .map(el => el.textContent.trim())
                .filter(text => text.length > 0)
        `, &headings),
        chromedp.Evaluate(`
            Array.from(document.querySelectorAll('p'))
                .map(el => el.textContent.trim())
                .filter(text => text.length > 0)
        `, &paragraphs),
    )

	if err != nil {
		return WebContent{}, err
	}

	// convert into string or list of strings
	headingsString := strings.Join(headings, "\n")
	paragraphsString := strings.Join(paragraphs, "\n")

	return WebContent{
		Title: title,
		Headings: headingsString,
		Paragraphs: paragraphsString,
	}, nil
}

// summarizeContent is a function that summarizes the content of a given WebContent
// it takes a WebContent as an argument
// it returns a string
func summarizeContent(webContent WebContent) (string, error) {
	url := "https://api-inference.huggingface.co/models/facebook/bart-large-cnn"
	
	summarizedContent := ""

	  	// Combine headings and paragraphs for maximum content utilization
	combinedContent := webContent.Headings + "\n\n" + webContent.Paragraphs
	
	// Truncate to stay within 1024 token limit (rough approximation)
	maxChars := 1024 * 4 // ~4096 characters
	if len(combinedContent) > maxChars {
		combinedContent = combinedContent[:maxChars]
	}

	fmt.Printf("Combined content length: %d chars\n", len(combinedContent))
	// summarize the content
	// inputs: the content to summarize (string)
	// parameters: the parameters for the summarization
	// max_length: the maximum length of tokens in the summary
	// min_length: the minimum length of tokens in the summary
	// length_penalty: the penalty for the length of the summary
	// num_beams: the number of beams for the summary
	payload := map[string]interface{}{
		"inputs": combinedContent,
		"parameters": map[string]interface{}{
			"max_length": 500,
			"min_length": 100,
			"length_penalty": 2.0,
			"num_beams": 4,
		},
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// convert the payload to a json string
	body, _ := json.Marshal(payload)

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	// set the headers
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("HF_API_KEY")))
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	fmt.Println("response status: ", response.StatusCode)

	defer response.Body.Close()

	// read the stream of bytes into a string
	bod, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	summarizedContent = string(bod)

	return summarizedContent, nil
}

// createNotionPage creates a new page in Notion under a parent page
func createNotionPage(title, content, parentPageID string) (string, error) {
	// POST endpoint to create a new page
	url := "https://api.notion.com/v1/pages"
	
	// Split content into paragraphs for better formatting
	paragraphs := strings.Split(content, "\n")
	
	// Create a list of children blocks (paragraphs)
	var children []NotionBlock
	
	// Create content blocks for each line of the paragrahs 
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			children = append(children, NotionBlock{
				Object: "block",
				Type:   "paragraph",
				Paragraph: NotionParagraph{
					RichText: []NotionText{
						{
							Text: NotionTextContent{
								Content: paragraph,
							},
						},
					},
				},
			})
		}
	}
	
	// Create the page request
	pageRequest := NotionPageRequest{
		Parent: NotionParent{
			Type:   "page_id",
			PageID: parentPageID,
		},
		Properties: NotionProperties{
			Title: NotionTitle{
				Title: []NotionText{
					{
						Text: NotionTextContent{
							Content: title,
						},
					},
				},
			},
		},
		Children: children,
	}

	// Convert to JSON
	body, err := json.Marshal(pageRequest)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+os.Getenv("NOTION_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	// Parse response to get the page ID
	var pageResponse map[string]interface{}
	if err := json.Unmarshal(respBody, &pageResponse); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	pageID, ok := pageResponse["id"].(string)
	if !ok {
		return "", fmt.Errorf("could not get page ID from response")
	}

	fmt.Printf("Successfully created Notion page: %s (ID: %s)\n", title, pageID)
	return pageID, nil
}

// appendToNotionPage appends content to an existing Notion page
func appendToNotionPage(pageID, content string) (string, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/blocks/%s/children", pageID)

	// Split content into paragraphs for better formatting
	paragraphs := strings.Split(content, "\n")
	var children []NotionBlock
	
	// Create content blocks 
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			children = append(children, NotionBlock{
				Object: "block",
				Type:   "paragraph",
				Paragraph: NotionParagraph{
					RichText: []NotionText{
						{
							Text: NotionTextContent{
								Content: paragraph,
							},
						},
					},
				},
			})
		}
	}
	
	// Create the append request
	appendRequest := map[string]interface{}{
		"children": children,
	}

	// Convert to JSON
	body, err := json.Marshal(appendRequest)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+os.Getenv("NOTION_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	fmt.Printf("Successfully appended content to page: %s\n", pageID)
	return pageID, nil
}


// main is the main function that fetches the titles of the given URLs
func main (){

	// load the environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	// Start timing the entire operation
	startTime := time.Now()
	
	urls := []string{
		"https://www.edmtunes.com/2025/08/spotify-plans-to-increase-their-prices-once-again/",
		"https://medium.engineering/engineering-growth-at-medium-4935b3234d25",
		"https://medium.engineering/engineering-growth-at-medium-4935b3234d25",
	}
	

	fmt.Printf("Starting to fetch titles for %d URLs...\n", len(urls))

	// create wait group for goroutines
	var wg sync.WaitGroup

	// create a context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

	// create a channel to receive results + working pool
	results := make(chan Result, len(urls))
	jobs := make(chan string, len(urls))

	numWorkers := 5

	//start the pool of 10 workers
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go worker(ctx, w, jobs, results, & wg)
	}

	// send the jobs to the pool
	for _, url := range urls {
		jobs <- url
	}
	close(jobs)

	// close the channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()
    
	collectedWebcontent := []string{}

	// iterate over the channel and collect results until it is closed (when all goroutines are done)
	for res := range results {
		if res.Error != nil {
			fmt.Printf("Error fetching title for %s: %v\n", res.URL, res.Error)
		}else{
			collectedWebcontent = append(collectedWebcontent, res.Summary)
		}
	}

	// Create one Notion page for all summaries
	pageTitle := "Article Summaries - " + time.Now().Format("2006-01-02 15:04:05")
	parentPageID := os.Getenv("NOTION_PARENT_PAGE_ID")
	
	// Create the initial content for the page
	initialContent := "# Summarized Web Content\n\n This contains the summarized web content of the URLs that were scraped :)\n"
	
	createdPageID, err := createNotionPage(pageTitle, initialContent, parentPageID)
	if err != nil {
		fmt.Printf("Error creating Notion page: %v\n", err)
		return
	}
	
	// Process each summary and append to the page
	for i, summary := range collectedWebcontent {
		// 1. Parse the JSON string into a Go structure
		var summaryData []map[string]interface{}
		if err := json.Unmarshal([]byte(summary), &summaryData); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			fmt.Printf("Raw response: %s\n", summary)
			continue
		} 
		
		if len(summaryData) > 0 {
			// 2. Access the first element and the summary_text field
			if summaryText, ok := summaryData[0]["summary_text"].(string); ok {
				fmt.Printf("Summary %d: %s\n", i+1, summaryText[:100]+"...")
				
				// 3. Format the summary with a header
				formattedContent := fmt.Sprintf("## Summary %d\n\n%s\n\n---\n\n", i+1, summaryText)
				
				// 4. Append to the Notion page
				if _, err := appendToNotionPage(createdPageID, formattedContent); err != nil {
					fmt.Printf("Error appending to Notion page: %v\n", err)
				}
				
				// Small delay to avoid rate limiting
				time.Sleep(500 * time.Millisecond)
			} else {
				fmt.Printf("Error: summary_text not found\n")
			}
		}
		fmt.Println("--------------------------------")
	}

	// Calculate and display timing
	elapsed := time.Since(startTime)
	
	fmt.Printf("\n=== TIMING RESULTS ===\n")
	fmt.Printf("Total URLs processed: %d\n", len(urls))
	fmt.Printf("Successful fetches: %d\n", len(collectedWebcontent))
	fmt.Printf("Total time elapsed: %v\n", elapsed)
	fmt.Printf("Average time per URL: %v\n", elapsed/time.Duration(len(urls)))
	fmt.Printf("URLs per second: %.2f\n", float64(len(urls))/elapsed.Seconds())
}