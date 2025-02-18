package main

import (
	"log"
	//"io/ioutil"
	"bufio"
	"net/http"
	"golang.org/x/net/html"
	//"regexp"
	"os"
	"io"
	"strings"
	"bytes"
	"flag"
)


func main(){

	//vars
	var domainList []string
	var baseDomain string
	var searchTerm string
	var path string

	flag.StringVar(&searchTerm, "t", "search term", "Search Term")
	flag.StringVar(&path, "p", "./domainList", "path to domains file")
	flag.Parse()

	domainList = readDomainsFile(path) //read entries in domains file
	
	for _, baseDomain = range(domainList) {
		log.Println("Searching domain:", baseDomain, "for search term", searchTerm)
		fetchFiles(baseDomain, baseDomain, searchTerm)
	}
}


func fetchFiles(domain string, baseDomain string, searchTerm string) {
	var urlData io.ReadCloser
	var URL string
	var buf bytes.Buffer


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
	
	search := searchBody(strings.ToLower(siteString), strings.ToLower(searchTerm))
	if search == 0 {
		log.Println("FOUND AT: ", URL)
	} 

	siteMap := collectInternalLinks(&buf, URL)

	//works up to here

	defer urlData.Close()
 	for key := range(siteMap) {
		if key == "CSS" {
			for _, obj := range(siteMap["CSS"]) {
				fetchFiles(obj, baseDomain, searchTerm)
			}
		} else if key == "JS" {
			for _,  obj := range(siteMap["JS"]) {
				fetchFiles(obj, baseDomain, searchTerm)
			}
		} else if key == "A" {
			for _, obj := range(siteMap["A"]) {
				if strings.Contains(strings.ToLower(obj), strings.ToLower(baseDomain)) {
					fetchFiles(obj, baseDomain, searchTerm)
				} else {
					log.Println("EXTERNAL SITE")
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


func readDomainsFile(path string) []string {
	var domainList []string
	
	file, err := os.Open(path)
	message := "INVALID FILE PATH " + path
	errorOutput(err, true, message)

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domainList = append(domainList, scanner.Text())
	}

	return domainList
}


func errorOutput(err error, kill bool, message string) {
	if err != nil {
		log.Println(err)
		if kill == true {
			log.Fatal(message)
		}
	}
}
