package main

import (
	"fmt"
	"io/ioutil"
	"log"
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
		fn        Func
		store     map[string]*entry
		reqStream chan request
	}

	mutexCache struct {
		fn    Func
		store map[string]*entry
		mu    *sync.Mutex
	}
)

type request struct {
	url      string
	response chan response
}
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

func newEntry(key string) *entry {
	return &entry{
		res: response{
			url: key,
		},
		ready: make(chan int),
	}
}

func (c *cache) Get(key string) response {
	respStream := make(chan response)
	c.reqStream <- request{
		url:      key,
		response: respStream,
	}
	response := <-respStream
	return response
}

func (c *cache) handleRequests(stop chan int, f Func) {
	for {
		select {
		case <-stop:
			close(c.reqStream)
			log.Println("request handler is shutdown")
			return

		case req, ok := <-c.reqStream:
			if !ok {
				return
			}
			key := req.url
			e := c.store[key]
			if e == nil {
				e = newEntry(key)
				c.store[key] = e

				go func(e *entry) {
					e.res.value, e.res.err = f(req.url)
					close(e.ready)
					req.response <- e.res
				}(e)
				continue
			}

			go func(e *entry) {
				<-e.ready
				req.response <- e.res
			}(e)
		}
	}
}

func (c *mutexCache) Get(key string) response {
	// Check for a cache hit, block until entry is ready if cache hit
	c.mu.Lock()
	e := c.store[key]

	if e == nil {
		// Cache miss, create entry and return the lock
		e = newEntry(key)
		c.store[key] = e
		c.mu.Unlock()

		// Perform fetch and signal when ready
		e.res.value, e.res.err = c.fn(key)
		close(e.ready)
		return e.res
	}

	// Cache hit, return the lock and wait for ready signal
	c.mu.Unlock()
	<-e.ready
	return e.res

}

func NewCache(stop chan int, f Func) *cache {
	c := &cache{
		fn:        f,
		store:     make(map[string]*entry),
		reqStream: make(chan request),
	}

	go c.handleRequests(stop, f)

	return c
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
func urlProducer(stop <-chan int, repeat int) <-chan string {
	var (
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
				log.Println("url producer shutting down")
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
				log.Println("url consumer shutting down")
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
	stop := make(chan int)

	c := NewCache(stop, httpGetBody)
	urlStream := urlProducer(stop, 4)
	respStream := urlConsumer(stop, urlStream, c)

	for res := range respStream {
		if res.err != nil {
			fmt.Printf("error reading %s: %s", res.url, res.err)
			continue
		}

		fmt.Printf("%s, %s, %d bytes\n", res.url, time.Since(res.start), len(res.value.([]byte)))
	}

	// Clean up the cache once done taking responses
	log.Println("starting graceful shutdown")
	close(stop)
	<-c.reqStream
	log.Println("shutdown complete, goodbye")
}
