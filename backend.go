package main

import (
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/dlclark/regexp2"
)

var (
	settings   settingsT
	smap       scraping
	outputFile = "output"
)

const (
	settingsConfig = "settings.json"
	scrapingConfig = "sitemap.json"
	logFile        = "logs.log"
)

type selectors struct {
	ID               string   `json:"id"`
	Type             string   `json:"type"`
	ParentSelectors  []string `json:"parentSelectors"`
	Selector         string   `json:"selector"`
	Multiple         bool     `json:"multiple"`
	Regex            string   `json:"regex"`
	Delay            int      `json:"delay"`
	ExtractAttribute string
}

type scraping struct {
	ID        string      `json:"_id,omitempty"`
	StartURL  []string    `json:"startUrl"`
	Selectors []selectors `json:"selectors"`
}

type settingsT struct {
	Gui        bool
	Log        bool
	JavaScript bool
	Workers    int
	Export     string
	UserAgents []string
	Captcha    string
	Proxy      []string
}

type jsonType struct {
	Settings settingsT
	Sitemap  scraping
}

type workerJob struct {
	startURL string
	parent   string
	siteMap  *scraping
	// doc        *goquery.Document
	linkOutput map[string]interface{}
}

func clearCache() {
	operatingSystem := runtime.GOOS
	switch operatingSystem {
	case "windows":
		os.RemoveAll(os.TempDir())
	case "darwin":
		os.RemoveAll(os.TempDir())
	case "linux":
		os.RemoveAll(os.TempDir())
	default:
		fmt.Println("Error: Temporary files can't be deleted.")
	}
}

func logErrors(error error) {
	if settings.Log {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		defer file.Close()
		log.SetOutput(file)
		log.Println(err)
	}
}

func readJSON() {
	jsonData := jsonType{}
	data, err := ioutil.ReadFile("./sitemap.json")
	if err != nil {
		frontendLog(err)
	}

	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		frontendLog(err)
	}

	smap = jsonData.Sitemap
	settings = jsonData.Settings
}

func writeJSON() {
	jsonData := jsonType{settings, smap}
	dataJSON, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		frontendLog(err)
	}

	err = ioutil.WriteFile("./sitemap.json", dataJSON, 0644)
	if err != nil {
		frontendLog(err)
	}
}

func readSiteMap() *scraping {
	data, err := ioutil.ReadFile(scrapingConfig)
	var scrape scraping
	err = json.Unmarshal(data, &scrape)
	if err != nil {
		logErrors(err)
		os.Exit(0)
	}
	return &scrape
}

func selectorText(doc *goquery.Document, selector *selectors) []string {
	var text []string
	var matchText *regexp2.Match
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if selector.Regex != "" {
			re := regexp2.MustCompile(selector.Regex, 0)
			if matchText, _ = re.FindStringMatch(s.Text()); matchText != nil {
				text = append(text, strings.TrimSpace(matchText.String()))
			} else {
				text = append(text, strings.TrimSpace(s.Text()))
			}
		} else {
			text = append(text, strings.TrimSpace(s.Text()))
		}
		if selector.Multiple == false {
			return false
		}
		return true
	})
	return text
}

func selectorLink(doc *goquery.Document, selector *selectors, baseURL string) []string {
	var links []string
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		href, ok := s.Attr("href")
		if !ok {
			fmt.Println("Error: HREF has not been found.")
		}
		links = append(links, toFixedURL(href, baseURL))
		if selector.Multiple == false {
			return false
		}
		return true
	})
	return links
}

func selectorElementAttribute(doc *goquery.Document, selector *selectors) []string {
	var links []string
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		href, ok := s.Attr(selector.ExtractAttribute)
		if !ok {
			fmt.Println("Error: HREF has not been found.")
		}
		links = append(links, href)
		if selector.Multiple == false {
			return false
		}
		return true
	})
	return links
}

func selectorElement(doc *goquery.Document, selector *selectors, startURL string) []interface{} {
	baseSiteMap := readSiteMap()
	var elementoutputList []interface{}
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		elementoutput := make(map[string]interface{})
		for _, elementSelector := range baseSiteMap.Selectors {
			if selector.ID == elementSelector.ParentSelectors[0] {
				if elementSelector.Type == "SelectorText" {
					resultText := s.Find(elementSelector.Selector).Text()
					elementoutput[elementSelector.ID] = resultText
				} else if elementSelector.Type == "SelectorImage" {
					resultText, ok := s.Find(elementSelector.Selector).Attr("src")
					if !ok {
						fmt.Println("Error: HREF has not been found.")
					}
					elementoutput[elementSelector.ID] = resultText
				} else if elementSelector.Type == "SelectorLink" {
					resultText, ok := s.Find(elementSelector.Selector).Attr("href")
					if !ok {
						fmt.Println("Error: HREF has not been found.")
					}
					elementoutput[elementSelector.ID] = resultText
				}
			}
		}
		if len(elementoutput) != 0 {
			elementoutputList = append(elementoutputList, elementoutput)
		}
		if selector.Multiple == false {
			return false
		}
		return true
	})
	return elementoutputList
}

func selectorImage(doc *goquery.Document, selector *selectors) []string {
	var srcs []string
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		src, ok := s.Attr("src")
		if !ok {
			fmt.Println("Error: HREF has not been found.")
		}
		srcs = append(srcs, src)
		if selector.Multiple == false {
			return false
		}
		return true
	})
	return srcs
}

func selectorTable(doc *goquery.Document, selector *selectors) map[string]interface{} {
	var headings, row []string
	var rows [][]string
	table := make(map[string]interface{})
	doc.Find(selector.Selector).Each(func(index int, tablehtml *goquery.Selection) {
		tablehtml.Find("tr").Each(func(indextr int, rowhtml *goquery.Selection) {
			rowhtml.Find("th").Each(func(indexth int, tableheading *goquery.Selection) {
				headings = append(headings, tableheading.Text())
			})
			rowhtml.Find("td").Each(func(indexth int, tablecell *goquery.Selection) {
				row = append(row, tablecell.Text())
			})
			if len(row) != 0 {
				rows = append(rows, row)
				row = nil
			}
		})
	})
	table["header"] = headings
	table["rows"] = rows
	return table
}

func crawlURL(href, userAgent string) *goquery.Document {
	var transport *http.Transport
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}
	if len(settings.Proxy) > 0 {
		proxyString := settings.Proxy[0]
		proxyURL, _ := url.Parse(proxyString)
		transport = &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyURL(proxyURL),
		}
	} else {
		transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}
	netClient := &http.Client{
		Transport: transport,
	}
	req, err := http.NewRequest(http.MethodGet, href, nil)
	if len(userAgent) > 0 {
		req.Header.Set("User-Agent", userAgent)
	}
	response, err := netClient.Do(req)
	if err != nil {
		logErrors(err)
		os.Exit(0)
	}
	defer response.Body.Close()
	doc, err := goquery.NewDocumentFromReader(response.Body)
	return doc
}

func toFixedURL(href, baseURL string) string {
	uri, err := url.Parse(href)
	base, err := url.Parse(baseURL)
	if err != nil {
		logErrors(err)
		os.Exit(0)
	}
	toFixedURI := base.ResolveReference(uri)
	return toFixedURI.String()
}

func getSiteMap(startURL []string, selector *selectors) *scraping {
	baseSiteMap := readSiteMap()
	newSiteMap := new(scraping)
	newSiteMap.ID = selector.ID
	newSiteMap.StartURL = startURL
	newSiteMap.Selectors = baseSiteMap.Selectors
	return newSiteMap
}

func getChildSelector(selector *selectors) bool {
	baseSiteMap := readSiteMap()
	count := 0
	for _, childSelector := range baseSiteMap.Selectors {
		if selector.ID == childSelector.ParentSelectors[0] {
			count++
		}
	}
	if count == 0 {
		return true
	}
	return false
}

func HasElem(s interface{}, elem interface{}) bool {
	arrV := reflect.ValueOf(s)
	if arrV.Kind() == reflect.Slice {
		for i := 0; i < arrV.Len(); i++ {
			if arrV.Index(i).Interface() == elem {
				return true
			}
		}
	}
	return false
}

func emulateURL(url, userAgent string) *goquery.Document {
	var opts []func(*chromedp.ExecAllocator)
	if len(settings.Proxy) > 0 {
		proxyString := settings.Proxy[0]
		proxyServer := chromedp.ProxyServer(proxyString)
		opts = append(chromedp.DefaultExecAllocatorOptions[:], proxyServer)
	} else {
		opts = append(chromedp.DefaultExecAllocatorOptions[:])
	}
	if len(userAgent) > 0 {
		opts = append(opts, chromedp.UserAgent(userAgent))
	}
	bctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, _ := chromedp.NewContext(bctx)
	defer cancel()
	var err error
	var body string
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.InnerHTML(`body`, &body, chromedp.NodeVisible, chromedp.ByQuery),
	)
	r := strings.NewReader(body)
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		logErrors(err)
		os.Exit(0)
	}
	return doc
}

func getURL(urls []string) <-chan string {
	c := make(chan string)
	go func() {
		re := regexp2.MustCompile(`(\[\d{1,10}-\d{1,10}\]$)`, 0)
		for _, urlLink := range urls {
			urlRange, _ := re.FindStringMatch(urlLink)
			if urlRange != nil {
				val2 := strings.Replace(urlLink, fmt.Sprintf("%s", urlRange), "", -2)
				urlRange2 := fmt.Sprintf("%s", urlRange)
				for _, charc := range []string{"[", "]"} {
					urlRange2 = strings.Replace(urlRange2, charc, "", -2)
				}
				rang := strings.Split(urlRange2, "-")
				int1, _ := strconv.ParseInt(rang[0], 10, 64)
				int2, _ := strconv.ParseInt(rang[1], 10, 64)
				for x := int1; x <= int2; x++ {
					c <- fmt.Sprintf("%s%d", val2, x)
				}
			} else {
				c <- urlLink
			}
		}
		close(c)
	}()
	return c
}

func worker(workerID int, jobs <-chan workerJob, results chan<- workerJob, wg *sync.WaitGroup) {
	defer wg.Done()
	userAgents := settings.UserAgents
	if len(userAgents) == 0 {
		userAgents = append(userAgents, "")
	}
	for count := 0; count < len(userAgents); count++ {
		userAgent := userAgents[count]
		for job := range jobs {
			var doc *goquery.Document
			if settings.JavaScript {
				doc = emulateURL(job.startURL, userAgent)
			} else {
				doc = crawlURL(job.startURL, userAgent)
			}
			if doc == nil {
				continue
			}
			fmt.Println("URL:", job.startURL)
			linkOutput := make(map[string]interface{})
			for _, selector := range job.siteMap.Selectors {
				if job.parent == selector.ParentSelectors[0] {
					if selector.Type == "SelectorText" {
						resultText := selectorText(doc, &selector)
						if len(resultText) != 0 {
							if len(resultText) == 1 {
								linkOutput[selector.ID] = resultText[0]
							} else {
								linkOutput[selector.ID] = resultText
							}
						}
					} else if selector.Type == "SelectorLink" {
						links := selectorLink(doc, &selector, job.startURL)
						if HasElem(selector.ParentSelectors, selector.ID) {
							for _, link := range links {
								if !HasElem(job.siteMap.StartURL, link) {
									job.siteMap.StartURL = append(job.siteMap.StartURL, link)
								}
							}
						} else {
							childSelector := getChildSelector(&selector)
							if childSelector == true {
								linkOutput[selector.ID] = links
							} else {
								newSiteMap := getSiteMap(links, &selector)
								result := scraper(newSiteMap, selector.ID)
								linkOutput[selector.ID] = result
							}
						}
					} else if selector.Type == "SelectorElementAttribute" {
						resultText := selectorElementAttribute(doc, &selector)
						linkOutput[selector.ID] = resultText
					} else if selector.Type == "SelectorImage" {
						resultText := selectorImage(doc, &selector)
						if len(resultText) != 0 {
							if len(resultText) == 1 {
								linkOutput[selector.ID] = resultText[0]
							} else {
								linkOutput[selector.ID] = resultText
							}
						}
					} else if selector.Type == "SelectorElement" {
						resultText := selectorElement(doc, &selector, job.startURL)
						linkOutput[selector.ID] = resultText
					} else if selector.Type == "SelectorTable" {
						resultText := selectorTable(doc, &selector)
						linkOutput[selector.ID] = resultText
					}
				}
			}
			job.linkOutput = linkOutput
			results <- job
		}
	}
}

func scraper(siteMap *scraping, parent string) map[string]interface{} {
	output := make(map[string]interface{})
	var wg sync.WaitGroup
	jobs := make(chan workerJob, settings.Workers)
	results := make(chan workerJob, settings.Workers)
	outputChannel := make(chan map[string]interface{})
	for x := 1; x <= settings.Workers; x++ {
		wg.Add(1)
		go worker(x, jobs, results, &wg)
	}
	go func() {
		fc := getURL(siteMap.StartURL)
		if fc != nil {
			for startURL := range fc {
				if !validURL(startURL) {
					continue
				}
				workerjob := workerJob{
					parent:   parent,
					startURL: startURL,
					siteMap:  siteMap,
				}
				jobs <- workerjob
			}
			close(jobs)
		}
	}()
	go func() {
		pageOutput := make(map[string]interface{})
		for job := range results {
			if len(job.linkOutput) != 0 {
				if job.parent == "_root" {
					out, err := ioutil.ReadFile(outputFile)
					if err != nil {
						logErrors(err)
						os.Exit(0)
					}
					var data map[string]interface{}
					err = json.Unmarshal(out, &data)
					data[job.startURL] = job.linkOutput
					switch settings.Export {
					case "xml":
						output, err := xml.MarshalIndent(data, "", " ")
						if err != nil {
							logErrors(err)
							os.Exit(0)
						}
						_ = ioutil.WriteFile(outputFile, output, 0644)
					case "csv":
						csvFile, err := os.OpenFile(outputFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
						if err != nil {
							logErrors(err)
							os.Exit(0)
						}
						csvWriter := csv.NewWriter(csvFile)
						rows := [][]string{}
						for i, v := range data {
							rows = append(rows, []string{i, fmt.Sprint(v)})
						}
						for _, row := range rows {
							_ = csvWriter.Write(row)
						}
						csvWriter.Flush()
						csvFile.Close()
					case "json":
						output, err := json.MarshalIndent(data, "", " ")
						if err != nil {
							logErrors(err)
							os.Exit(0)
						}
						_ = ioutil.WriteFile(outputFile, output, 0644)
					default:
						fmt.Println("Error: Please choose an output format.")
					}
				} else {
					pageOutput[job.startURL] = job.linkOutput
				}
			}
		}
		outputChannel <- pageOutput
	}()
	wg.Wait()
	close(results)
	output = <-outputChannel
	return output
}

func validURL(uri string) bool {
	_, err := url.ParseRequestURI(uri)
	if err != nil {
		logErrors(err)
		return false
	}
	return true
}

func outputResult() {
	userFormat := strings.ToLower(settings.Export)
	allowedFormat := map[string]bool{
		"csv":  true,
		"xml":  true,
		"json": true,
	}
	if allowedFormat[userFormat] {
		outputFile = fmt.Sprintf("output.%s", userFormat)
		_ = ioutil.WriteFile(outputFile, []byte("{}"), 0644)
	}
}

func scrape() {
	readJSON()
	clearCache()
	siteMap := smap
	outputResult()
	_ = scraper(&siteMap, "_root")
}
