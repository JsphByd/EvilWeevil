package main

import (
	"fmt"
	"log"
	"time"

	//"bufio"

	"bytes"
	"flag"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"gopkg.in/yaml.v2"
)

type yamlData struct {
	Domains     []string `yaml:"domains"`
	SearchTerms []string `yaml:"search_terms"`
}

type Config struct {
	Domain   string
	NetGraph map[string][]string
}

func main() {
	var globalGraph Config
	var yamlData yamlData
	args := make([]string, 2)

	flag.StringVar(&args[0], "p", "./config.yml", "path to domains file")
	flag.StringVar(&args[1], "w", "0", "wait a number of seconds between sending requests")
	flag.Parse()

	yamlData = readDomainsFile(args[0])
	globalGraph.NetGraph = make(map[string][]string)

	for _, baseDomain := range yamlData.Domains {

		if string(baseDomain[len(baseDomain)-1]) == "/" { //remove trailing /
			baseDomain = baseDomain[:len(baseDomain)-1]
		}
		if !strings.HasPrefix("https://", baseDomain) {
			baseDomain = "https://" + baseDomain
		}

		globalGraph.Domain = baseDomain
		log.Println("\n\nSearching Domain: ", baseDomain, "for search terms", yamlData.SearchTerms) //UU
		globalGraph = parser(globalGraph, baseDomain, yamlData.SearchTerms, args, yamlData)
	}

}

func parser(graph Config, domain string, searchTerms []string, args []string, yamlData yamlData) Config {
	//var urlData io.ReadCloser
	var URL string
	var outgoingMap []string
	var buf bytes.Buffer

	if string(domain[len(domain)-1]) == "/" {
		domain = domain[:len(domain)-1]
	}

	if !strings.Contains(domain, "http") {
		URL = "https://" + domain
	} else {
		URL = domain
	}

	if _, ok := graph.NetGraph[URL]; !ok {
		waitTime, _ := strconv.Atoi(args[1])
		time.Sleep(time.Duration(waitTime) * time.Second)

		response := getRespBody(URL)

		//check status code
		if response.StatusCode == 404 {
			log.Println("404 @", URL)
		} else if response.StatusCode == 429 {
			log.Println("429 @", URL)
			time.Sleep(10 * time.Second)
			response = getRespBody(URL)
		}
		responseBody := response.Body
		defer responseBody.Close()

		tee := io.TeeReader(responseBody, &buf)
		bytes, _ := io.ReadAll(tee)
		siteString := string(bytes)

		search(siteString, searchTerms, URL)

		outgoingMap = getOutgoingLinks(&buf, URL, graph.Domain, yamlData)
		graph.NetGraph[URL] = append(graph.NetGraph[URL], outgoingMap...)
		for _, link := range graph.NetGraph[URL] {
			parser(graph, link, searchTerms, args, yamlData)
		}
	}
	return graph
}

func search(siteText string, searchTerms []string, URL string) {
	for _, searchTerm := range searchTerms {
		if strings.Contains(siteText, searchTerm) {
			fmt.Fprintf(os.Stdout, "FOUND %s AT %s\n", searchTerm, URL)
		}
	}
}

func getOutgoingLinks(htmlBody io.Reader, URL string, baseDomain string, yamlData yamlData) []string {
	var outGoingLinks []string
	doc, err := html.Parse(htmlBody)
	if err != nil {
		log.Fatal("KILLED ON URL", URL, "FOR ERROR", err)
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		var styleSheet bool
		domainSplit := strings.Split(baseDomain, ".")

		if n.Data == "a" { // A - links to other pages
			for _, attr := range n.Attr {
				if len(attr.Val) != 0 {
					if attr.Key == "href" && string(attr.Val[0]) != "#" && string(attr.Val[0]) != "?" {
						aPath := attr.Val

						if string(URL[len(URL)-1]) == "/" {
							URL = URL[:len(URL)-1]
						}

						if !strings.HasPrefix(aPath, "http") && aPath != "/" {
							if string(aPath[0]) == "/" && string(aPath[1]) != "/" { //abs link
								aPath = baseDomain + attr.Val
							} else if strings.HasPrefix(aPath, "//") {
								aPath = "https:" + attr.Val
							} else { //rel link
								aPath = URL + "/" + aPath //UU
							}
						} else if aPath == "/" { //root
							aPath = baseDomain
						}
						aPathSplit := strings.Split(aPath, ".")
						if strings.Contains(aPath, baseDomain) && aPathSplit[0] == domainSplit[0] {
							outGoingLinks = append(outGoingLinks, aPath)
						}
					}
				}
			}
		} else if n.Data == "link" {
			for _, attr := range n.Attr {
				if attr.Key == "rel" && attr.Val == "stylesheet" {
					styleSheet = true
				}
				if styleSheet && attr.Key == "href" && len(attr.Val) != 0 {
					cssPath := attr.Val
					if !strings.Contains(attr.Val, "http") {
						if string(attr.Val[0]) == "/" && string(attr.Val[1]) != "/" {
							cssPath = baseDomain + attr.Val
						} else if strings.HasPrefix(cssPath, "//") {
							cssPath = "https:" + attr.Val
						} else {
							cssPath = URL + "/" + attr.Val
						}
					}
					cssPathSplit := strings.Split(cssPath, ".")
					if strings.Contains(cssPath, baseDomain) && cssPathSplit[0] == domainSplit[0] {
						resp := getRespBody(cssPath)
						respBody := resp.Body
						bytes, _ := io.ReadAll((respBody))
						siteString := string(bytes)

						search(siteString, yamlData.SearchTerms, cssPath)
					}
				}
			}
		} else if n.Data == "script" {
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					jsPath := attr.Val
					if !strings.Contains(attr.Val, "http") {
						if string(attr.Val[0]) == "/" && string(attr.Val[1]) != "/" {
							jsPath = baseDomain + attr.Val
						} else if string(attr.Val[:2]) == "//" {
							jsPath = "https:" + attr.Val
						} else {
							jsPath = URL + "/" + attr.Val
						}
					}
					jsPathSplit := strings.Split(jsPath, ".")
					if strings.Contains(jsPath, baseDomain) && jsPathSplit[0] == domainSplit[0] {
						resp := getRespBody(jsPath)
						respBody := resp.Body
						bytes, _ := io.ReadAll((respBody))
						siteString := string(bytes)

						search(siteString, yamlData.SearchTerms, jsPath)
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return outGoingLinks
}

func getRespBody(URL string) *http.Response {
	resp, err := http.Get(URL)
	errorOutput(err, false, "")
	if err != nil {
		log.Println("FAILED TO GET URL: ", URL)
		return nil
	}
	return resp
}

func readDomainsFile(path string) yamlData {
	var config yamlData
	yamlData, err := os.ReadFile("config.yml")
	message := "INVALID FILE PATH " + path
	errorOutput(err, true, message)

	err = yaml.Unmarshal(yamlData, &config)
	if err != nil {
		log.Fatal(err)
	}
	errorOutput(err, true, "Invalid Yaml")

	return config
}

func errorOutput(err error, kill bool, message string) {
	if err != nil {
		log.Println(err)
		if kill {
			log.Fatal(message)
		}
	}
}
