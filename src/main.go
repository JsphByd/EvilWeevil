package main

import (
	"fmt"
	"log"
	"time"

	"regexp"
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
	RegexTerms  []string `yaml: "regex_terms"`
}

type Config struct {
	Domain   string
	NetGraph map[string][]string
}

func main() {
	var globalGraph Config
	var yamlData yamlData
	args := make([]string, 3)

	flag.StringVar(&args[0], "p", "./config.yml", "path to domains file")
	flag.StringVar(&args[1], "w", "0", "wait a number of seconds between sending requests")
	flag.StringVar(&args[2], "rP", " ", "Preset regex string (email, phone numbers, code comments)")
	flag.Parse()

	yamlData = readDomainsFile(args[0])
	globalGraph.NetGraph = make(map[string][]string)

	for _, baseDomain := range yamlData.Domains {

		if string(baseDomain[len(baseDomain)-1]) == "/" { //remove trailing /
			baseDomain = baseDomain[:len(baseDomain)-1]
		}

		globalGraph.Domain = baseDomain
		log.Println("\n\nSearching Domain: ", baseDomain, "for search terms") //UU
		globalGraph = parser(globalGraph, baseDomain, yamlData, args)
	}
}

func parser(graph Config, domain string, yamlData yamlData, args []string) Config {
	//var urlData io.ReadCloser
	var URL string
	var outgoingMap []string
	var buf bytes.Buffer
	searchTerms := yamlData.SearchTerms
	regexTerms := yamlData.RegexTerms

	if string(domain[len(domain)-1]) == "/" {
		domain = domain[:len(domain)-1]
	}

	if !strings.Contains(domain, "http") {
		URL = "https://" + domain
	} else {
		URL = domain
	}

	//log.Println(URL)

	if _, ok := graph.NetGraph[URL]; !ok {
		waitTime, _ := strconv.Atoi(args[1])
		time.Sleep(time.Duration(waitTime) * time.Second)

		response := getRespBody(URL)

		if response == nil {
			return graph
		} else if strings.Contains(URL, "/./.") {
			return graph
		}

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

		//need two copies of buffer, one for the search function and another for the link parser 
		tee := io.TeeReader(responseBody, &buf)
		bytes, _ := io.ReadAll(tee)
		siteString := string(bytes)

		//searching for stuff!
		// 1. Search plaintext strings in search_terms
		for _, searchTerm := range searchTerms {
			if strings.Contains(siteString, searchTerm) {
				fmt.Fprintf(os.Stdout, "FOUND %s AT %s\n", searchTerm, URL)
			}
		}
		// 2. Search regex strings in regex_terms
		for _, regexTerm := range regexTerms {
			pattern := regexTerm
			match, err := regexp.MatchString(pattern, siteString)
			fmt.Println(regexTerm)
			if err != nil {
				errorOutput(err, false, string("Invalid regular expression: " + regexTerm))
			} else {
				fmt.Println("MATCH FOUND! ----------------------------------------", match)
			}
		}

		outgoingMap = getOutgoingLinks(&buf, URL, graph.Domain)
		graph.NetGraph[URL] = append(graph.NetGraph[URL], outgoingMap...)
		for _, link := range graph.NetGraph[URL] {
			parser(graph, link, yamlData, args)
		}
	}
	return graph
}

func getOutgoingLinks(htmlBody io.Reader, URL string, baseDomain string) []string {
	var outGoingLinks []string
	doc, err := html.Parse(htmlBody)
	if err != nil {
		log.Fatal("KILLED ON URL", URL, "FOR ERROR", err)
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Data == "a" {
			for _, attr := range n.Attr {
				if len(attr.Val) != 0 {
					if attr.Key == "href" && string(attr.Val[0]) != "#" && !strings.Contains(attr.Val, "?") && !strings.Contains(attr.Val, "..") && !strings.HasPrefix(attr.Val, "mailto:") && !strings.HasPrefix(attr.Val, "tel:") && !strings.HasPrefix(attr.Val, "fax:") && !strings.HasPrefix(attr.Val, "skype:") && !strings.HasPrefix(attr.Val, "sms:") && !strings.HasPrefix(attr.Val, "geo:") && !strings.HasPrefix(attr.Val, "callto:") && !strings.HasPrefix(attr.Val, "/.") {
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
								if strings.Contains(aPath, ".") {
									aPathSplit := strings.Split(aPath, ".")
									if (strings.Contains(aPathSplit[len(aPathSplit)-1], "php") || strings.Contains(aPathSplit[len(aPathSplit)-1], "html") || strings.Contains(aPathSplit[len(aPathSplit)-1], "htm")) && (strings.Contains(URL, aPath)) {
										aPath = URL
									} else {
										aPath = URL + "/" + aPath //UU
									}
								} else {
									aPath = URL + "/" + aPath //UU
								}
							}
						} else if aPath == "/" { //root
							aPath = baseDomain
						}

						domainSplit := strings.Split(baseDomain, ".")
						aPathSplit := strings.Split(aPath, ".")
						if strings.Contains(aPath, baseDomain) && aPathSplit[0] == domainSplit[0] && aPath != URL {
							outGoingLinks = append(outGoingLinks, aPath)
						}
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
