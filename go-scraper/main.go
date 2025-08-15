package main

import (
	"fmt"
	"context"
	"net/http"
	"time"
	"strings"
	"github.com/PuerkitoBio/goquery"
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


	doc, err := goquery.NewDocumentFromReader(res.Body)
	
	if err != nil {
		return "", err
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	return title, nil
}

func main (){
    ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()

	title, err := fetchTitle(ctx, "https://example.com")

	if err != nil {
		panic(err)
	}

	fmt.Println("Title: ", title)

}