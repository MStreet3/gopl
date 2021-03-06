// Package cache exposes a Cache interface that stores responses from http requests.  The cache
// implementations in the package are each concurrent, duplicate supressing and non-blocking.
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

	// cache requires goroutines to place requests into its reqStream.  all access to the store is
	// limited to the request handler itself, which handles incoming requests in a non-blocking
	// manner.
	cache struct {
		fn        Func
		store     map[string]*entry
		reqStream chan request
	}

	// mutexCache uses a cache lock to allow concurrent access to the store from multiple goroutines
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

// call blocks until the function returns a value and an error.  entry's ready state is signalled
// and the response is sent along the passed response stream
func (e *entry) call(f Func, key string, respStream chan<- response) {
	e.res.value, e.res.err = f(key)
	close(e.ready)
	respStream <- e.res
}

// deliver blocks until an entry is ready and then sends its response on the passed response stream
func (e *entry) deliver(respStream chan<- response) {
	<-e.ready
	respStream <- e.res
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

// Get puts a new request on the cache's request stream and blocks until a response is received.
func (c *cache) Get(key string) response {
	respStream := make(chan response)
	c.reqStream <- request{
		url:      key,
		response: respStream,
	}
	response := <-respStream
	return response
}

// handleRequests is a service that processes all of the cache requests pulled from the request
// stream.  handleRequests ensures that duplicates are suppressed and that requests are processed
// in a non-blocking manner.
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

			// Cache miss, launch new goroutine to populate the entry
			if e == nil {
				e = newEntry(key)
				c.store[key] = e
				go e.call(c.fn, req.url, req.response)
				continue
			}

			// Cache hit, launch new goroutine to return response when found entry is ready
			go e.deliver(req.response)
		}
	}
}

func NewMutexCache(f Func) *mutexCache {
	return &mutexCache{
		fn:    f,
		store: make(map[string]*entry),
		mu:    &sync.Mutex{},
	}
}

// Get is called concurrently to access a cache's store and each goroutine needs mutually exclusive
// access.
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

// httpGetBody is a simple function to make a get request on a given url.  httpGetBody
// blocks until the request is complete.
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
	return []string{
		"https://golang.org",
		"https://godoc.org",
		"https://play.golang.org",
		"https://gopl.io",
	}
}

// urlProducer generates a fixed stream of urls
func urlProducer(stop <-chan int, repeat int) <-chan string {
	var (
		urlStream = make(chan string)
		urls      = incomingUrls()
	)

	go func() {
		defer func() {
			log.Println("url producer done producing")
			close(urlStream)
		}()

		for i := 0; i < repeat+1; i++ {
			select {
			case <-stop:
				log.Println("url producer was stopped")
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

// handleUrl calls a cache's Get method and forwards the response along
func handleUrl(respStream chan<- response, c Cache, url string) {
	start := time.Now()
	response := c.Get(url)
	response.start = start
	respStream <- response
}

// urlConsumer reads from a urlStream and launches a new goroutine to execute the Get method
// of the given cache.  Returns a stream of responses that is closed once all urls have been
// processed.  urlConsumer ensures that the cache requests are done in a concurrent manner.
func urlConsumer(stop <-chan int, urlStream <-chan string, c Cache) <-chan response {
	respStream := make(chan response)

	// Launch goroutine to consume the urlStream, waits until all urls are fetched
	// and then closes the response channel.
	go func() {
		var wg sync.WaitGroup

		defer func() {
			wg.Wait()
			log.Println("url consumer done consuming")
			close(respStream)
		}()

		for url := range urlStream {
			select {
			case <-stop:
				log.Println("url consumer was stopped")
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
	}()

	return respStream
}

func main() {
	var (
		msg        string
		shutdown   func()
		stop       = make(chan int)
		c          = NewCache(stop, httpGetBody)
		urlStream  = urlProducer(stop, 4)
		respStream = urlConsumer(stop, urlStream, c)
	)

	// Clean up once done taking responses
	shutdown = func() {
		log.Println("starting graceful shutdown")
		close(stop)
		<-c.reqStream
		log.Println("shutdown complete, goodbye")
	}

	defer shutdown()

	for res := range respStream {
		if res.err != nil {
			log.Println(fmt.Sprintf("error reading %s: %s", res.url, res.err))
			continue
		}

		msg = fmt.Sprintf("%s, %s, %d bytes", res.url, time.Since(res.start), len(res.value.([]byte)))
		log.Println(msg)
	}
}
