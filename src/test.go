package main

import (
	"log"

	//"bufio"
	//"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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
	var path string
	var yamlData yamlData

	flag.StringVar(&path, "p", "./config.yml", "path to domains file")
	flag.Parse()

	yamlData = readDomainsFile(path)

	for _, baseDomain := range yamlData.Domains {
		globalGraph.Domain = baseDomain
		log.Println("\n\nSearching Domain: ", baseDomain, "for search terms") //UU
		globalGraph = parser(globalGraph, baseDomain)
		fmt.Println(globalGraph.NetGraph)
	}

}

func parser(graph Config, domain string) Config {
	//var urlData io.ReadCloser
	//var buf bytes.Buffer
	var URL string
	outgoingMap := make(map[string][]string)

	if !strings.Contains(domain, "http") {
		URL = "https://" + domain
	} else {
		URL = domain
	}
	graph.Domain = URL

	responseBody := getRespBody(URL)
	defer responseBody.Close()

	// UU
	//create a copy of resp body
	//tee := io.TeeReader(responseBody, &buf)
	//bytes, _ := io.ReadAll(tee)
	//siteString := string(bytes)

	//fmt.Println(siteString)

	outgoingMap[URL] = getOutgoingLinks(responseBody, URL)
	graph.NetGraph = outgoingMap
	/*
		1. Get outgoing links
	*/
	//for _, link := range graph.NetGraph[URL] {
	//	parser(graph, link)
	//}

	return graph
}

func getOutgoingLinks(htmlBody io.Reader, URL string) []string {
	var outGoingLinks []string
	doc, err := html.Parse(htmlBody)
	if err != nil {
		log.Fatal("KILLED ON URL", URL, "FOR ERROR", err)
	}

	//var f func(*html.Node, []string) []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			fmt.Println(n.Data)
			if n.Data == "a" {
				for _, attr := range n.Attr {
					if len(attr.Val) != 0 {
						if attr.Key == "href" && attr.Val != "#" && string(attr.Val[0]) != "?" {
							aPath := attr.Val
							fmt.Println(aPath)
							if !strings.HasPrefix(aPath, "http") {
								if string(aPath[0]) == "/" { //rel link
									aPath = URL + attr.Val
								} else if strings.HasPrefix(aPath, "//") {
									aPath = "https:" + attr.Val
								} else {
									aPath = URL + "/" + aPath //UU
								}
							}
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

func getRespBody(URL string) io.ReadCloser {
	resp, err := http.Get(URL)
	errorOutput(err, false, "")
	if err != nil {
		log.Println("FAILED TO GET URL: ", URL)
		return nil
	}
	return resp.Body
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
