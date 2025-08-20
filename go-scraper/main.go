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

	// create a HTTP server that cancels request after 10 sec
	client := &http.Client{Timeout: 10 * time.Second}

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

	// split the content into chunks
	chunks := splitIntoChunks(webContent.Paragraphs, 5000)

	// summarize each chunk
	for _, chunk := range chunks {
		
		// inputs: the content to summarize (string)
		// parameters: the parameters for the summarization
		// max_length: the maximum length of tokens in the summary
		// min_length: the minimum length of tokens in the summary
		// length_penalty: the penalty for the length of the summary
		// num_beams: the number of beams for the summary
		payload := map[string]interface{}{
			"inputs": chunk,
			"parameters": map[string]interface{}{
				"max_length": 150,
				"min_length": 30,
				"length_penalty": 2.0,
				"num_beams": 4,
			},
		}

		client := &http.Client{Timeout: 10 * time.Second}

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

		summarizedContent += string(bod)
	}

	fmt.Println("summarizedContent: ", summarizedContent)

	return summarizedContent, nil
}

// splitIntoChunks is a function that splits the text into chunks of a given size
// it takes a text and a chunk size as arguments
// it returns a list of strings
func splitIntoChunks(text string, chunkSize int) []string {
    chunks := []string{}
    for i := 0; i < len(text); i += chunkSize {
        end := i + chunkSize
        if end > len(text) {
            end = len(text)  // Don't go beyond the string length
        }
        chunks = append(chunks, text[i:end])
    }
    return chunks
}

// main is the main function that fetches the titles of the given URLs
func main (){

	// load the environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	fmt.Println("os.Getenv(\"HF_API_KEY\"): ", os.Getenv("HF_API_KEY"))

	// Start timing the entire operation
	startTime := time.Now()
	
	urls := []string{
		"https://www.mckinsey.com/capabilities/mckinsey-digital/our-insights/the-top-trends-in-tech",
	}
	

	fmt.Printf("Starting to fetch titles for %d URLs...\n", len(urls))

	// create wait group for goroutines
	var wg sync.WaitGroup

	// create a context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

	// create a channel to receive results + working pool
	results := make(chan Result, len(urls))
	jobs := make(chan string, len(urls))

	numWorkers := 10

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

	for _, summary := range collectedWebcontent {
		fmt.Printf("Summary: %s\n", summary)
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