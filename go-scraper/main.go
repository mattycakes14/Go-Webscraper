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
	WebContent WebContent
	Error error
}

// struct for the web content
type WebContent struct {
	Title string
	Main string
	Div string
}

// worker is a function that fetches the title of a given URL
func worker(ctx context.Context, id int, jobs <-chan string, result chan<- Result, wg *sync.WaitGroup) {
	// decrement the wait group
	defer wg.Done()
	for url := range jobs {
		webContent, err := scrapeAboveFold(ctx, url)
		if err != nil {
			result <- Result{URL: url, WebContent: WebContent{}, Error: err}
		} else {
			result <- Result{URL: url, WebContent: webContent, Error: nil}
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
	main := doc.Find("main").First()
	div := main.Find("div").First()

	return WebContent{
		Title: title,
		Main: main.Text(),
		Div: div.Text(),
	}, nil
}

// main is the main function that fetches the titles of the given URLs
func main (){
	// Start timing the entire operation
	startTime := time.Now()
	
	urls := []string{
		"https://www.wikipedia.org",
		"https://www.wikimedia.org",
		"https://www.bbc.com",
		"https://www.cnn.com",
		"https://www.reuters.com",
		"https://www.npr.org",
		"https://www.nytimes.com",
		"https://www.theguardian.com",
		"https://www.forbes.com",
		"https://www.bloomberg.com",
		"https://www.ft.com",
		"https://www.aljazeera.com",
		"https://www.ap.org",
		"https://www.nationalgeographic.com",
		"https://www.scientificamerican.com",
		"https://www.nature.com",
		"https://www.sciencenews.org",
		"https://www.space.com",
		"https://www.si.edu",
		"https://www.loc.gov",
		"https://www.imdb.com",
		"https://www.rottentomatoes.com",
		"https://www.metacritic.com",
		"https://www.allmusic.com",
		"https://www.last.fm",
		"https://www.discogs.com",
		"https://www.pitchfork.com",
		"https://www.billboard.com",
		"https://www.rollingstone.com",
		"https://www.spin.com",
		"https://www.nme.com",
		"https://www.kexp.org",
		"https://www.bandcamp.com",
		"https://www.soundcloud.com",
		"https://www.mixcloud.com",
		"https://www.shazam.com",
		"https://www.genres.com",   
		"https://www.producthunt.com",
		"https://www.techcrunch.com",
		"https://www.wired.com",
		"https://www.theverge.com",
		"https://www.cnet.com",
		"https://www.gizmodo.com",
		"https://www.engadget.com",
		"https://www.tomshardware.com",
		"https://www.anandtech.com",
		"https://www.pcgamer.com",
		"https://www.gamespot.com",
		"https://www.ign.com",
		"https://www.gameinformer.com",
		"https://www.metacritic.com/game",
		"https://www.polygon.com",
		"https://www.destructoid.com",
		"https://www.kotaku.com",
		"https://www.relay.fm",
		"https://www.medium.com",
		"https://www.substack.com",
		"https://www.quora.com",
		"https://www.reddit.com",
		"https://www.stackoverflow.com",
		"https://www.github.com",
		"https://www.gitlab.com",
		"https://www.bitbucket.org",
		"https://www.sourceforge.net",
		"https://www.python.org",
		"https://www.nodejs.org",
		"https://www.rust-lang.org",
		"https://www.java.com",
		"https://www.php.net",
		"https://www.djangoproject.com",
		"https://www.flask.palletsprojects.com",
		"https://www.ruby-lang.org",
		"https://www.go.dev",
		"https://www.tensorflow.org",
		"https://www.pytorch.org",
		"https://www.openai.com",
		"https://www.anthropic.com",
		"https://www.deepmind.com",
		"https://www.ibm.com",
		"https://www.microsoft.com",
		"https://www.apple.com",
		"https://www.google.com",
		"https://www.amazon.com",
		"https://www.netflix.com",
		"https://www.disneyplus.com",
		"https://www.hulu.com",
		"https://www.primevideo.com",
		"https://www.spotify.com",
		"https://www.tidal.com",
		"https://www.deezer.com",
		"https://www.qobuz.com",
		"https://www.soundonsound.com",
		"https://www.musicradar.com",
		"https://www.gearspace.com",
		"https://www.korg.com",
		"https://www.roland.com",
		"https://www.native-instruments.com",
		"https://www.ableton.com",
		"https://www.image-line.com",
		"https://www.steinberg.net",
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
    
	collectedTitles := []WebContent{}

	// iterate over the channel and collect results until it is closed (when all goroutines are done)
	for res := range results {
		if res.Error != nil {
			fmt.Printf("Error fetching title for %s: %v\n", res.URL, res.Error)
		}else{
			collectedTitles = append(collectedTitles, res.WebContent)
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