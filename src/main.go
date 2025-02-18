package main

import (
	"log"
	//"bufio"
	"gopkg.in/yaml.v2"
	"net/http"
	"golang.org/x/net/html"
	"os"
	"io"
	"strings"
	"bytes"
	"flag"
)

type Config struct {
	Domains 	[]string `yaml:"domains"`
	SearchTerms []string `yaml:"search_terms"`
}

func main(){

	//vars
	var config Config
	var baseDomains []string
	var searchTerms []string
	var baseDomain string
	var path string

	flag.StringVar(&path, "p", "./config.yml", "path to domains file")
	flag.Parse()

	config = readDomainsFile(path) //read entries in domains file
	baseDomains = config.Domains
	searchTerms = config.SearchTerms
	
	for _, baseDomain = range(baseDomains) {
		log.Println("\n\nSearching domain:", baseDomain, "for search terms", searchTerms)
		fetchFiles(baseDomain, baseDomain, searchTerms)
	}
}


func fetchFiles(domain string, baseDomain string, searchTerms []string) {
	var urlData io.ReadCloser
	var URL string
	var buf bytes.Buffer
	var searchTerm string
	var search int


	if !strings.Contains(domain, "http") {
		URL = "https://" + domain
	} else {
		URL = domain
	}

	urlData = getRespBody(URL)
	if urlData == nil {
		return
	}

	tee := io.TeeReader(urlData, &buf) //duplicate buffer!

	bytes, _ := io.ReadAll(tee)
	siteString := string(bytes)
	
	for _, searchTerm = range(searchTerms) {
		search = searchBody(strings.ToLower(siteString), strings.ToLower(searchTerm))
		if search == 0 {
			log.Println("FOUND AT: ", URL)
		} 
	}

	siteMap := collectInternalLinks(&buf, URL)

	//works up to here

	defer urlData.Close()
 	for key := range(siteMap) {
		if key == "CSS" {
			for _, obj := range(siteMap["CSS"]) {
				fetchFiles(obj, baseDomain, searchTerms)
			}
		} else if key == "JS" {
			for _,  obj := range(siteMap["JS"]) {
				fetchFiles(obj, baseDomain, searchTerms)
			}
		} else if key == "A" {
			for _, obj := range(siteMap["A"]) {
				if strings.Contains(strings.ToLower(obj), strings.ToLower(baseDomain)) {
					fetchFiles(obj, baseDomain, searchTerms)
				} else {
					log.Println("EXTERNAL SITE", obj)
				}
			}
		}	
	} 

}

func searchBody(siteText string, query string) int {
	if strings.Contains(siteText, query) {
		return 0
	} else {
		return 1
	}
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

func collectInternalLinks(htmlBody io.Reader, URL string) map[string][]string {
	linkMap := make(map[string][]string)


	//parse
	doc, err := html.Parse(htmlBody)
	
	if err != nil {
		log.Fatal("DEAD")
		return linkMap
	}


	var f func(*html.Node, map[string][]string) map[string][]string

	f = func (n *html.Node, linkMap map[string][]string) map[string][]string {
		if n.Type == html.ElementNode {
			var styleSheet bool
			//CSS
			if n.Data == "link" {
				for _, attr := range n.Attr {
					if attr.Key == "rel" && attr.Val == "stylesheet" {
						styleSheet = true
					} 
					if styleSheet == true && attr.Key == "href" {
						cssPath := attr.Val
						if !strings.Contains(attr.Val, "http") {
							if string(attr.Val[0]) != "/" {
								cssPath = URL + "/" + attr.Val
							} else {
								cssPath = URL + attr.Val
							}
						}
						linkMap["CSS"] = append(linkMap["CSS"], cssPath)
					} 
				}
			}
			//JS
			if n.Data == "script" {
				for _, attr := range n.Attr {
					if attr.Key == "src" {
						jsPath := attr.Val
						if !strings.Contains(attr.Val, "http") {
							if string(attr.Val[0]) != "/" {
								jsPath = URL + "/" + attr.Val
							} else {
								jsPath = URL + attr.Val
							}
						}

						linkMap["JS"] = append(linkMap["JS"], jsPath)
					}
				}
			}
			//A
			if n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key == "href" && attr.Val != "#" {
						aPath := attr.Val
						if !strings.Contains(attr.Val, "http") {
							if string(attr.Val[0]) != "/" {
								aPath = URL + "/" + attr.Val
							} else {
								aPath = URL + attr.Val
							}
						}
						linkMap["A"] = append(linkMap["A"], aPath)
					}

				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c, linkMap)
		}
		return linkMap
	}

    linkMap = f(doc, linkMap)
	return linkMap
}





func createDir(baseDomain string, subString string, sub bool) string { //create directory for domain

	var foldPath string

	if sub == false {
		foldPath = "../websites/" + baseDomain
		os.Mkdir(foldPath, 755)
		os.Mkdir(foldPath + "/MAIN", 755)
		return foldPath + "/MAIN"
	} else {
		foldPath = "../websites/" + baseDomain + subString
		os.Mkdir(foldPath, 755)
		return foldPath
	}
}


func readDomainsFile(path string) Config {
	var config Config

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
		if kill == true {
			log.Fatal(message)
		}
	}
}
