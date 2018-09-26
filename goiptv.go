package goiptv

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"github.com/indiependente/goiptv/filter"
	log "github.com/sirupsen/logrus"
)

const (
	lastHourQueryTemplate = "https://www.google.co.uk/search?q=%s+site:pastebin.com&source=lnt&tbs=qdr:h&sa=X&ved=0ahUKEwia3_bDrNndAhXKIMAKHTrIA2YQpwUIIw&biw=1440&bih=755"
	lastDayQueryTemplate  = "https://www.google.co.uk/search?q=%s+site:pastebin.com&source=lnt&tbs=qdr:d&sa=X&ved=0ahUKEwjt-JSZrNndAhXJJMAKHXahDXIQpwUIIw&biw=1440&bih=755"
	lastWeekQueryTemplate = "https://www.google.co.uk/search?q=%s+site:pastebin.com&source=lnt&tbs=qdr:w&sa=X&ved=0ahUKEwi4pteVrNndAhURQMAKHYZcDH0QpwUIIw&biw=1440&bih=755"
	// LastHour is the argument to pass to the NewIPTVScraper function or to set as IPTVScraper.TimeSpan value,
	// in order to let the scraper search the playlist generated in the last hour.
	LastHour = "H"
	// LastDay is the argument to pass to the NewIPTVScraper function or to set as IPTVScraper.TimeSpan value,
	// in order to let the scraper search the playlist generated in the last 24 hours.
	LastDay = "D"
	// LastWeek is the argument to pass to the NewIPTVScraper function or to set as IPTVScraper.TimeSpan value,
	// in order to let the scraper search the playlist generated in the last 7 days.
	LastWeek = "W"
)

// IPTVScraper abstracts an IPTV Scraper.
type IPTVScraper struct {
	TimeSpan string
}

// NewIPTVScraper returns a new IPTV Scraper that will scrape iptv playlists
// generated in the timespan passed as the only argument.
// timeSpan: the only accepted values are
func NewIPTVScraper(timeSpan string) *IPTVScraper {
	return &IPTVScraper{
		TimeSpan: timeSpan,
	}
}

// ScrapeAll returns a channel of io.Readers from where reading the content of M3U playlist
func (is *IPTVScraper) ScrapeAll(tvchannels ...string) chan io.Reader {
	readerChan := make(chan io.Reader)
	done := make(chan struct{})
	for _, c := range tvchannels {
		go is.scrape(c, readerChan, done)
	}
	go waitAndClose(len(tvchannels), readerChan, done)
	return readerChan
}

func (is *IPTVScraper) scrape(tvchannel string, channel chan io.Reader, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	var (
		wg      sync.WaitGroup
		qFormat string
	)

	outData := make(chan *bytes.Buffer, 20)
	switch is.TimeSpan {
	case LastHour:
		qFormat = lastHourQueryTemplate

	case LastDay:
		qFormat = lastDayQueryTemplate

	case LastWeek:
		qFormat = lastWeekQueryTemplate
	}

	link := fmt.Sprintf(qFormat, strings.Replace(tvchannel, " ", "+", -1))

	c := colly.NewCollector()
	extensions.RandomUserAgent(c)
	// Find all cite and get the text
	c.OnHTML("cite", func(e *colly.HTMLElement) {
		wg.Add(1)
		if !strings.HasPrefix(e.Text, "http") {
			e.Text = "https://" + e.Text
		}
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
		reader := bytes.NewReader(r.Body)
		b, err := filter.Filter(reader)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("scan error")
		}
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
