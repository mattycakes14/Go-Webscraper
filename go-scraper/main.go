package main

import (
	"fmt"
	"context"
	"net/http"
	"time"
	"strings"
	"github.com/PuerkitoBio/goquery"
	"sync"
)

// define a struct to store the results
type Result struct {
	URL string
	Title string
	Error error
}

// worker is a function that fetches the title of a given URL
func worker(ctx context.Context, id int, jobs <-chan string, result chan<- Result, wg *sync.WaitGroup) {
	// decrement the wait group
	defer wg.Done()
	for url := range jobs {
		title, err := fetchTitle(ctx, url)
		if err != nil {
			result <- Result{URL: url, Title: "", Error: err}
		} else {
			result <- Result{URL: url, Title: title, Error: nil}
		}
	}
}
// fetchTitle is a function that fetches the title of a given URL
// it takes a context and a url as arguments
// it returns a string and an error
func fetchTitle(ctx context.Context, url string) (string, error) {

	// create a HTTP server that cancels request after 10 sec
	client := &http.Client{Timeout: 10 * time.Second}

	// make a request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return "", err
	}

	// set the header
	req.Header.Set("User-Agent", "go-scraper")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// parses the stream of bytes into a goquery document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	
	if err != nil {
		return "", err
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	return title, nil
}

// main is the main function that fetches the titles of the given URLs
func main (){
	// Start timing the entire operation
	startTime := time.Now()
	
	urls := []string{
		"https://www.amazon.com/",
		"https://www.example.com",
		"https://www.google.com",
		"https://www.facebook.com",
		"https://www.twitter.com",
		"https://www.instagram.com",
		"https://www.linkedin.com",
		"https://www.youtube.com",
		"https://www.wikipedia.org",
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

	numWorkers := 5

	//start the pool
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
    
	collectedTitles := []string{}

	// iterate over the channel and collect results until it is closed (when all goroutines are done)
	for res := range results {
		if res.Error != nil {
			fmt.Printf("Error fetching title for %s: %v\n", res.URL, res.Error)
		}else{
			collectedTitles = append(collectedTitles, res.Title)
		}
	}

	fmt.Println("Collected Titles: ", collectedTitles)

	// Calculate and display timing
	elapsed := time.Since(startTime)
	
	fmt.Printf("\n=== TIMING RESULTS ===\n")
	fmt.Printf("Total URLs processed: %d\n", len(urls))
	fmt.Printf("Successful fetches: %d\n", len(collectedTitles))
	fmt.Printf("Total time elapsed: %v\n", elapsed)
	fmt.Printf("Average time per URL: %v\n", elapsed/time.Duration(len(urls)))
	fmt.Printf("URLs per second: %.2f\n", float64(len(urls))/elapsed.Seconds())
}