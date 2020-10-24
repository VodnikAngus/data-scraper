package main

import (
	"crypto/tls"
	"encoding/json"
	"os"

	// "encoding/csv"
	// "encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dlclark/regexp2"
)

const (
	settingsConfig = "settings.json"
	scrapingJSON   = "scraping.json"
	outputJSON     = "output.json"
)

var (
	config = &tls.Config{
		InsecureSkipVerify: false,
	}
	transport = &http.Transport{
		TLSClientConfig: config,
	}
	netClient = &http.Client{
		Transport: transport,
	}
)

// Selectors is struct to Marshal selector
type Selectors struct {
	ID               string
	Type             string
	ParentSelectors  []string
	Selector         string
	Multiple         bool
	Regex            string
	Delay            int
	ExtractAttribute string
}

// Scraping is struct to Marshal scraping file
type Scraping struct {
	StartURL  []string
	ID        string `json:"_id,omitempty"`
	Selectors []Selectors
}

func readSettingsJSON() {
	// open the file and read the file
	data, err := ioutil.ReadFile(settingsConfig)
	// define data struture
	type Config struct {
		JavaScript    bool
		Proxy         bool
		ProxyLists    []string
		RotatingProxy bool
		Export        string
	}
	// json data
	var settings Config
	err = json.Unmarshal(data, &settings)
	// log any errors
	if err != nil {
		log.Println(err)
	}
	// just priting it
	fmt.Println("JavaScript: ", settings.JavaScript)
	fmt.Println("Proxy: ", settings.Proxy)
	fmt.Println("ProxyLists: ", settings.ProxyLists)
	fmt.Println("RotatingProxy: ", settings.RotatingProxy)
	fmt.Println("Export: ", settings.Export)
}

func readSiteMap() *Scraping {
	// open the file and read the file
	data, err := ioutil.ReadFile(scrapingJSON)

	var scrape Scraping
	err = json.Unmarshal(data, &scrape)

	// log any errors
	if err != nil {
		log.Println(err)
	}

	return &scrape
}

// SelectorText get data text for html tag
func SelectorText(doc *goquery.Document, selector *Selectors) []string {
	// Find the review items
	// fmt.Println(selector.Selector)
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

// SelectorLink get data href for html tag
func SelectorLink(doc *goquery.Document, selector *Selectors, baseURL string) []string {
	// Find the review items
	// fmt.Println(selector.Selector)
	var links []string
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		href, ok := s.Attr("href")
		if !ok {
			fmt.Printf("HREF has not been found")
		}

		links = append(links, toFixedURL(href, baseURL))
		if selector.Multiple == false {
			return false
		}
		return true
	})
	return links
}

// SelectorElementAttribute get define attribute for html tag
func SelectorElementAttribute(doc *goquery.Document, selector *Selectors) []string {
	// Find the review items
	// fmt.Println(selector.Selector)
	var links []string
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		href, ok := s.Attr(selector.ExtractAttribute)
		if !ok {
			fmt.Printf("HREF has not been found")
		}

		links = append(links, href)
		if selector.Multiple == false {
			return false
		}
		return true
	})
	return links
}

// SelectorElement get child element of html selected element
func SelectorElement(doc *goquery.Document, selector *Selectors, startURL string) []interface{} {
	baseSiteMap := readSiteMap()
	var elementoutputList []interface{}
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		elementoutput := make(map[string]interface{})
		for _, elementSelector := range baseSiteMap.Selectors {
			if selector.ID == elementSelector.ParentSelectors[0] {
				if elementSelector.Type == "SelectorText" {
					// resultText := SelectorText(s, elementSelector)
					resultText := s.Find(elementSelector.Selector).Text()
					elementoutput[elementSelector.ID] = resultText
				} else if elementSelector.Type == "SelectorImage" {
					resultText, ok := s.Find(elementSelector.Selector).Attr("src")
					if !ok {
						fmt.Printf("HREF has not been found")
					}
					elementoutput[elementSelector.ID] = resultText
				} else if elementSelector.Type == "SelectorLink" {
					resultText, ok := s.Find(elementSelector.Selector).Attr("href")
					if !ok {
						fmt.Printf("HREF has not been found")
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

// SelectorImage get src of Image for html tag
func SelectorImage(doc *goquery.Document, selector *Selectors) []string {
	// Find the review items
	// fmt.Println(selector.Selector)
	var srcs []string
	doc.Find(selector.Selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		src, ok := s.Attr("src")
		if !ok {
			fmt.Printf("HREF has not been found")
		}
		srcs = append(srcs, src)
		if selector.Multiple == false {
			return false
		}
		return true
	})
	return srcs
}

// SelectorTable get header and row data of table
func SelectorTable(doc *goquery.Document, selector *Selectors) map[string]interface{} {
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

func crawlURL(href string) *goquery.Document {
	response, err := netClient.Get(href)
	if err != nil {
		log.Println(err)
	}
	defer response.Body.Close()

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Println(err)
	}

	return doc
}

func toFixedURL(href, baseURL string) string {
	uri, err := url.Parse(href)
	if err != nil {
		return ""
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	toFixedURI := base.ResolveReference(uri)

	return toFixedURI.String()
}

func getSiteMap(startURL []string, selector *Selectors) *Scraping {

	baseSiteMap := readSiteMap()
	newSiteMap := new(Scraping)
	newSiteMap.ID = selector.ID
	newSiteMap.StartURL = startURL
	newSiteMap.Selectors = baseSiteMap.Selectors
	return newSiteMap
}

func getChildSelector(selector *Selectors) bool {
	baseSiteMap := readSiteMap()
	var count int = 0
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

// HasElem check element is present or not in parsed list
func HasElem(s interface{}, elem interface{}) bool {
	arrV := reflect.ValueOf(s)
	if arrV.Kind() == reflect.Slice {
		for i := 0; i < arrV.Len(); i++ {

			// XXX - panics if slice element points to an unexported struct field
			// see https://golang.org/pkg/reflect/#Value.Interface
			if arrV.Index(i).Interface() == elem {
				return true
			}
		}
	}

	return false
}

func scraper(siteMap *Scraping, parent string) interface{} {

	output := make(map[string]interface{})
	urlLength := len(siteMap.StartURL)
	// for _, startURL := range siteMap.StartUrl {
	for i := 0; i < urlLength; i++ {
		startURL := siteMap.StartURL[i]
		linkOutput := make(map[string]interface{})
		fmt.Println("Start URL:", startURL)
		for _, selector := range siteMap.Selectors {
			if parent == selector.ParentSelectors[0] {
				doc := crawlURL(startURL)
				if selector.Type == "SelectorText" {
					resultText := SelectorText(doc, &selector)
					if len(resultText) != 0 {
						if len(resultText) == 1 {
							linkOutput[selector.ID] = resultText[0]
						} else {
							linkOutput[selector.ID] = resultText
						}
					}
				} else if selector.Type == "SelectorLink" {
					links := SelectorLink(doc, &selector, startURL)
					// fmt.Println(links)
					if HasElem(selector.ParentSelectors, selector.ID) {
						for _, link := range links {
							if !HasElem(siteMap.StartURL, link) {
								siteMap.StartURL = append(siteMap.StartURL, link)
							}
						}
						// fmt.Printf("appended urls : %v\n", siteMap.StartUrl)
						urlLength = len(siteMap.StartURL)
					} else {
						childSelector := getChildSelector(&selector)
						if childSelector == true {
							linkOutput[selector.ID] = links
						} else {
							newSiteMap := getSiteMap(links, &selector)
							result := scraper(newSiteMap, selector.ID)
							fmt.Printf("result = %v", result)
							linkOutput[selector.ID] = result
						}
					}
				} else if selector.Type == "SelectorElementAttribute" {
					resultText := SelectorElementAttribute(doc, &selector)
					linkOutput[selector.ID] = resultText
				} else if selector.Type == "SelectorImage" {
					resultText := SelectorImage(doc, &selector)
					if len(resultText) != 0 {
						if len(resultText) == 1 {
							linkOutput[selector.ID] = resultText[0]
						} else {
							linkOutput[selector.ID] = resultText
						}
					}
				} else if selector.Type == "SelectorElement" {
					resultText := SelectorElement(doc, &selector, startURL)
					linkOutput[selector.ID] = resultText
				} else if selector.Type == "SelectorTable" {
					resultText := SelectorTable(doc, &selector)
					linkOutput[selector.ID] = resultText
				}
			}
		}
		if len(linkOutput) != 0 {
			if parent == "_root" {
				out, err := ioutil.ReadFile(outputJSON)
				if err != nil {
					fmt.Printf("Error while reading %s file\n", outputJSON)
					os.Exit(1)
				}

				var data map[string]interface{}
				err = json.Unmarshal(out, &data)
				if err != nil {
					fmt.Printf("Failed to unmarshal %s file\n", outputJSON)
					os.Exit(1)
				}
				fmt.Printf("linkoutput: %v", linkOutput)
				data[startURL] = linkOutput
				file, err := json.MarshalIndent(data, "", " ")
				if err != nil {
					fmt.Println(err.Error())
					os.Exit(1)
				}
				// fmt.Println(file)
				_ = ioutil.WriteFile(outputJSON, file, 0644)
			} else {
				output[startURL] = linkOutput
			}
		}

	}
	return output
}

func main() {
	_ = ioutil.WriteFile(outputJSON, []byte("{}"), 0644)
	siteMap := readSiteMap()
	_ = scraper(siteMap, "_root")

}
