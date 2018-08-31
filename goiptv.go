package goiptv

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	log "github.com/sirupsen/logrus"
)

// ScrapeAll returns a channel of io.Readers from where reading the content of M3U playlist
func ScrapeAll(tvchannels ...string) chan io.Reader {
	readerChan := make(chan io.Reader)
	done := make(chan struct{})
	for _, c := range tvchannels {
		go scrape(c, readerChan, done)
	}
	go waitAndClose(len(tvchannels), readerChan, done)
	return readerChan
}

func scrape(tvchannel string, channel chan io.Reader, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	var wg sync.WaitGroup

	outData := make(chan *bytes.Buffer, 20)
	qFormat := "https://www.google.co.uk/search?q=%s+site:pastebin.com&source=lnt&tbs=qdr:d&sa=X&ved=0ahUKEwipyeLr9o3dAhUIIMAKHfsmBdAQpwUIIA&biw=1308&bih=761"
	link := fmt.Sprintf(qFormat, strings.Replace(tvchannel, " ", "+", -1))

	c := colly.NewCollector()
	extensions.RandomUserAgent(c)
	// Find all cite and get the text
	c.OnHTML("cite", func(e *colly.HTMLElement) {
		wg.Add(1)
		log.WithFields(log.Fields{"e.Text": e.Text}).Debug("element")
		go scrapeTextArea(e.Text, outData, &wg)
	})
	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		log.WithFields(log.Fields{"Request URL": r.Request.URL, "response": r, "error": err}).Error("error")
	})
	c.OnResponse(func(r *colly.Response) {
		log.WithFields(log.Fields{"status": r.StatusCode}).Debug("response")
	})

	c.Visit(link)
	wg.Wait()
	close(outData)
	for b := range outData {
		log.WithFields(log.Fields{"bodyLen": b.Len()}).Debug("forLoop")
		if b.Len() == 0 {
			log.Errorf("Zero length buffer: %v", b)
		}
		channel <- b
	}
}

func scrapeTextArea(url string, outCh chan *bytes.Buffer, wg *sync.WaitGroup) {
	defer wg.Done()

	c := colly.NewCollector()
	extensions.RandomUserAgent(c)
	c.OnRequest(func(r *colly.Request) {
		log.WithFields(log.Fields{"url": r.URL.String(), "ua": r.Headers.Get("user-agent")}).Debug("request")
	})

	c.OnResponse(func(r *colly.Response) {
		log.WithFields(log.Fields{"status": r.StatusCode}).Debug("response")
		var b bytes.Buffer
		b.Write(r.Body)
		log.WithFields(log.Fields{"bodyLen": b.Len()}).Debug("scrapeResponse")
		outCh <- &b
	})
	url = strings.Replace(url, ".com/", ".com/raw/", 1)
	log.WithFields(log.Fields{"url": url}).Debug("raw url")
	c.Visit(url)
}

func waitAndClose(goroutines int, channel chan io.Reader, done chan struct{}) {
	for goroutines > 0 {
		_ = <-done
		goroutines--
	}
	close(channel)
	close(done)
}
