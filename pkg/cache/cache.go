package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var _ Cache = (*cache)(nil)

type Func func(string) (interface{}, error)

type (
	Cache interface {
		Get(string) (interface{}, error)
	}

	cache struct {
		fn Func
	}
)

func (c *cache) Get(key string) (interface{}, error) {
	return c.fn(key)
}

func NewCache(f Func) *cache {
	return &cache{
		fn: f,
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

// incomingUrlStream generates a fixed stream of urls
func incomingUrlStream(stop <-chan int) <-chan string {
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

func main() {
	c := NewCache(httpGetBody)
	for _, url := range incomingUrls() {
		start := time.Now()
		value, err := c.Get(url)
		if err != nil {
			log.Println(err)
			continue
		}

		fmt.Printf("%s, %s, %d bytes\n", url, time.Since(start), len(value.([]byte)))
	}
}
