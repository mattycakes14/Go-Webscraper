package main

import (
	"fmt"
	"context"
	"net/http"
	"time"
	"strings"
	"github.com/PuerkitoBio/goquery",
	"sync"
)

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

func main (){
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

	// create wait group for goroutines
	var wg sync.WaitGroup

	
	// create a context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()

	for _, url := range urls{
		// add 1 to the wait group
		wg.Add(1)
		go func (url string) {
			defer wg.Done()
			title, err := fetchTitle(ctx, url)
			if err != nil {
				fmt.Printf("Error fetching title for %s: %v\n", url, err)
				return
			}
			fmt.Printf("Title for %s: %s\n", url, title)
		}(url) // pass the url to the goroutine to execute function
	}
	
	// block until all goroutines are done
	wg.Wait()
	title, err := fetchTitle(ctx, "https://example.com")

	if err != nil {
		panic(err)
	}

	fmt.Println("Title: ", title)

}