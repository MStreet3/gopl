package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

var _ Cache = (*cache)(nil)

var _ Cache = (*mutexCache)(nil)

type Func func(key string) (interface{}, error)

type (
	Cache interface {
		Get(key string) response
	}

	cache struct {
		fn    Func
		store map[string]*entry
	}

	mutexCache struct {
		fn    Func
		store map[string]*entry
		mu    *sync.Mutex
	}
)

type response struct {
	start time.Time
	url   string
	value interface{}
	err   error
}

type entry struct {
	res   response
	ready chan int
}

func (c *cache) Get(key string) response {
	return response{}
}

func (c *mutexCache) Get(key string) response {
	// Check for a cache hit, block until entry is ready if cache hit
	c.mu.Lock()
	e := c.store[key]

	if e != nil {
		c.mu.Unlock()
		<-e.ready
		return e.res
	}

	// Cache miss, create entry and return the lock
	e = &entry{
		ready: make(chan int),
	}
	c.store[key] = e
	c.mu.Unlock()

	// Perform fetch and signal when ready
	value, err := c.fn(key)

	e.res = response{
		value: value,
		err:   err,
		url:   key,
	}
	close(e.ready)
	return e.res
}

func NewCache(f Func) *cache {
	return &cache{
		fn:    f,
		store: make(map[string]*entry),
	}
}

func NewMutexCache(f Func) *mutexCache {
	return &mutexCache{
		fn:    f,
		store: make(map[string]*entry),
		mu:    &sync.Mutex{},
	}
}

func httpGetBody(url string) (interface{}, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// incomingUrls returns a fixed length slice of urls
func incomingUrls() []string {
	var (
		urls = []string{
			"https://golang.org",
			"https://godoc.org",
			"https://play.golang.org",
			"https://gopl.io",
		}
	)

	return append(urls, urls...)
}

// urlProducer generates a fixed stream of urls
func urlProducer(stop <-chan int) <-chan string {
	var (
		repeat    = 1
		urlStream = make(chan string)
		urls      = []string{
			"https://golang.org",
			"https://godoc.org",
			"https://play.golang.org",
			"https://gopl.io",
		}
	)

	go func() {
		defer close(urlStream)
		for i := 0; i < repeat+1; i++ {
			select {
			case <-stop:
				return

			default:
			}
			for _, url := range urls {
				urlStream <- url
			}
		}
	}()

	return urlStream
}

func handleUrl(respStream chan<- response, c Cache, url string) {
	start := time.Now()
	response := c.Get(url)
	response.start = start
	respStream <- response
}

// urlConsumer reads from a urlStream and launches a new goroutine to execute the Get method
// of the given cache.  Returns a stream of responses that is closed once all urls have been
// processed.
func urlConsumer(stop <-chan int, urlStream <-chan string, c Cache) <-chan response {
	respStream := make(chan response)

	// Launch goroutine to consume the urlStream, waits until all urls are fetched
	// and then closes the response channel.
	go func() {
		var wg sync.WaitGroup
		defer close(respStream)

		for url := range urlStream {
			select {
			case <-stop:
				return

			default:
			}

			// For each url, launch goroutine to fetch the url
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				handleUrl(respStream, c, url)
			}(url)
		}

		wg.Wait()
	}()
	return respStream
}

func main() {
	var (
		c    = NewMutexCache(httpGetBody)
		stop = make(chan int)
	)

	respStream := urlConsumer(stop, urlProducer(stop), c)

	for res := range respStream {
		if res.err != nil {
			fmt.Printf("error reading %s: %s", res.url, res.err)
			continue
		}

		fmt.Printf("%s, %s, %d bytes\n", res.url, time.Since(res.start), len(res.value.([]byte)))
	}
}
