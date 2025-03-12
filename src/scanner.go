/*
Project: WeevilSniffer
File Name: gui.go
Author: Joseph Boyd
Purpose: scanner.go handles all the scraping and file processing.
*/
package main

import (
	"regexp"
	"time"

	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type Config struct {
	Domain   string
	NetGraph map[string][]string
}

func initScan(ctx context.Context, GUIInteratables GUIInteratables, yamlData yamlData) {
	var globalGraph Config

	args := make([]string, 3)
	args[0] = "./config.yml"
	args[1] = "0"
	args[2] = " "

	globalGraph.NetGraph = make(map[string][]string)

	for _, baseDomain := range yamlData.Domains {

		if string(baseDomain[len(baseDomain)-1]) == "/" { //remove trailing /
			baseDomain = baseDomain[:len(baseDomain)-1]
		}

		globalGraph.Domain = baseDomain
		//log.Println("\n\nSearching Domain: ", baseDomain, "for search terms") //UU

		findingsArray := make(map[string][]string)
		(*GUIInteratables.Findings)[globalGraph.Domain] = findingsArray

		globalGraph = parser(globalGraph, baseDomain, yamlData, args, ctx, GUIInteratables)
	}
	GUIInteratables.ImagePane.SetImage(*GUIInteratables.Image)

	//update the results panel
	resultsString := ""
	for domainKey, findings := range *GUIInteratables.Findings {
		resultsString += domainKey + "\n"
		for searchTerm, urlArray := range findings {
			resultsString += "\t" + searchTerm + "\n"
			for _, URL := range urlArray {
				resultsString += "\t\t" + URL + "\n"
			}
		}
		resultsString += "\n"
	}

	GUIInteratables.Results.SetText(resultsString)

}

func parser(graph Config, domain string, yamlData yamlData, args []string, ctx context.Context, GUIInteratables GUIInteratables) Config {
	//var urlData io.ReadCloser
	var URL string
	var outgoingMap []string
	var buf bytes.Buffer
	searchTerms := yamlData.SearchTerms
	regexTerms := yamlData.RegexTerms
	updateGUI(GUIInteratables, domain)

	select {
	case <-ctx.Done():
		GUIInteratables.ScanInfo.SetText("SCANNER OFF")
		return graph
	default:
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
			//GUIInteratables.ScanInfo.SetText(URL)

			if response == nil {
				return graph
			} else if strings.Contains(URL, "/./.") {
				return graph
			}

			//check status code
			GUIInteratables.ScanInfo.SetText(URL)
			if response.StatusCode == 404 {
				*GUIInteratables.FailCount += 1
				updateGUI(GUIInteratables, URL)
				return graph
			} else if response.StatusCode == 429 {
				*GUIInteratables.FailCount += 1
				updateGUI(GUIInteratables, URL)
				return graph
			} else {
				*GUIInteratables.SuccessCount += 1
				updateGUI(GUIInteratables, URL)
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
					(*GUIInteratables.Findings)[graph.Domain][searchTerm] = append((*GUIInteratables.Findings)[graph.Domain][searchTerm], URL)
				}
			}

			// 2. Search regex strings in regex_terms
			for _, regexTerm := range regexTerms {
				searchRegex(regexTerm, siteString)
			}

			// 3. Search using presets
			if yamlData.Find_emails {
				emailString := `[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}`
				if searchRegex(emailString, siteString) {
					(*GUIInteratables.Findings)[graph.Domain]["EMAIL"] = append((*GUIInteratables.Findings)[graph.Domain]["EMAIL"], URL)
				}
			}
			if yamlData.Find_HTML_comments {
				HTMLCommentString := `<!--.*-->`
				if searchRegex(HTMLCommentString, siteString) {
					(*GUIInteratables.Findings)[graph.Domain]["HTML_COMMENT"] = append((*GUIInteratables.Findings)[graph.Domain]["HTML_COMMENT"], URL)
				}
			}
			if yamlData.Find_JS_comments {
				JSCommentString := `\/\//*`
				JSCommentStringAlt := `\/\*.*\*\/`
				if searchRegex(JSCommentString, siteString) || searchRegex(JSCommentStringAlt, siteString) {
					(*GUIInteratables.Findings)[graph.Domain]["JS_COMMENT"] = append((*GUIInteratables.Findings)[graph.Domain]["JS_COMMENT"], URL)
				}
			}
			if yamlData.Find_CSS_comments {
				CSSCommentString := `\/\*.*\*\/`
				if searchRegex(CSSCommentString, siteString) {
					(*GUIInteratables.Findings)[graph.Domain]["CSS_COMMENT"] = append((*GUIInteratables.Findings)[graph.Domain]["CSS_COMMENT"], URL)
				}
			}

			outgoingMap = getOutgoingLinks(&buf, URL, graph.Domain)
			graph.NetGraph[URL] = append(graph.NetGraph[URL], outgoingMap...)

			for _, link := range graph.NetGraph[URL] {
				parser(graph, link, yamlData, args, ctx, GUIInteratables)
			}
		}
	}
	return graph
}

func getOutgoingLinks(htmlBody io.Reader, URL string, baseDomain string) []string {
	var outGoingLinks []string
	doc, err := html.Parse(htmlBody)
	if err != nil {
		//log.Fatal("KILLED ON URL", URL, "FOR ERROR", err)
		os.Exit(0)
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Data == "a" {
			for _, attr := range n.Attr {
				if len(attr.Val) != 0 {
					//A links
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

						if strings.Contains(aPath, baseDomain) && (aPathSplit[0] == domainSplit[0] || "https://"+domainSplit[0] == aPathSplit[0] || "https://www."+domainSplit[0] == aPathSplit[0]+"."+aPathSplit[1]) && aPath != URL {
							outGoingLinks = append(outGoingLinks, aPath)
						}
					}
				}
			}
		} else if n.Data == "link" { //CSS
			stylesheet := false
			for _, attr := range n.Attr {
				if attr.Key == "rel" && attr.Val == "stylesheet" {
					stylesheet = true
				}
				if stylesheet && attr.Key == "href" {
					aPath := attr.Val
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
					if strings.Contains(aPath, baseDomain) && (aPathSplit[0] == domainSplit[0] || "https://"+domainSplit[0] == aPathSplit[0] || "https://www."+domainSplit[0] == aPathSplit[0]+"."+aPathSplit[1]) && aPath != URL {
						outGoingLinks = append(outGoingLinks, aPath)
					}

				}
			}
		} else if n.Data == "script" { //JS
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					aPath := attr.Val
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
					if strings.Contains(aPath, baseDomain) && (aPathSplit[0] == domainSplit[0] || "https://"+domainSplit[0] == aPathSplit[0] || "https://www."+domainSplit[0] == aPathSplit[0]+"."+aPathSplit[1]) && aPath != URL {
						outGoingLinks = append(outGoingLinks, aPath)
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
	errorOutput(err, false)
	if err != nil {
		return nil
	}
	return resp
}

func errorOutput(err error, kill bool) {
	if err != nil {
		//log.Println(err)
		if kill {
			os.Exit(0)
		}
	}
}

func updateGUI(GUIInteratables GUIInteratables, URL string) {
	successfulScanString := strconv.Itoa(*GUIInteratables.SuccessCount)
	failScanString := strconv.Itoa(*GUIInteratables.FailCount)

	if len(URL) > 70 {
		URL = URL[:70] + "..."
	}

	GUIInteratables.ScanInfo.SetText("[red]" + URL + "[white]" + "\n" + "\nScanned: [red]" + successfulScanString + "[white]\nFailed: [red]" + failScanString + "[white]")
}

func searchRegex(regexString string, parseText string) bool {
	regex, err := regexp.Compile(regexString)
	if err != nil {
		os.Exit(1)
	}
	found := regex.MatchString(parseText)
	return found
}
