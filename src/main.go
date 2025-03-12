package main

import (
	"context"
	"image"
	"image/png"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v2"
)

type GUIInteratables struct {
	App          *tview.Application
	ScanInfo     *tview.TextView
	Results      *tview.TextView
	ImagePane    *tview.Image
	SuccessCount *int
	FailCount    *int
	Image        *image.Image
	RedImage     *image.Image
	Findings     *map[string]map[string][]string
}

type yamlData struct {
	Domains            []string `yaml:"domains"`
	SearchTerms        []string `yaml:"search_terms"`
	RegexTerms         []string `yaml:"regex_terms"`
	Find_emails        bool     `yaml:"find_emails"`
	Find_HTML_comments bool     `yaml:"find_HTML_comments"`
	Find_JS_comments   bool     `yaml:"find_JS_comments"`
	Find_CSS_comments  bool     `yaml:"find_CSS_comments"`
}

func main() {
	var yamlData = readDomainsFile("./config.yml")
	scannerRunning := false
	ctx, cancel := context.WithCancel(context.Background())
	findings := make(map[string]map[string][]string)
	var GUIInteratables GUIInteratables
	var wg sync.WaitGroup
	var failCount int
	var successCount int
	presetRegexStatus := make([]bool, 4)
	presetRegexStatus[0] = yamlData.Find_emails
	presetRegexStatus[1] = yamlData.Find_HTML_comments
	presetRegexStatus[2] = yamlData.Find_JS_comments
	presetRegexStatus[3] = yamlData.Find_CSS_comments

	newPrimitive := func(text string) tview.Primitive {
		return tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetText(text)
	}

	app := tview.NewApplication()

	scanInfo := tview.NewTextView().
		SetDynamicColors(false).
		SetRegions(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	editorMenu := tview.NewList()

	domainEditor := tview.NewForm()
	domainTable := tview.NewTable()
	domainEditorInfo := tview.NewTextView().
		SetDynamicColors(false).
		SetRegions(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	searchEditor := tview.NewForm()
	searchTable := tview.NewTable()
	searchEditorInfo := tview.NewTextView().
		SetDynamicColors(false).
		SetRegions(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	regexEditor := tview.NewForm()
	regexTable := tview.NewTable()
	regexEditorInfo := tview.NewTextView().
		SetDynamicColors(false).
		SetRegions(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	domainFlexBox := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(domainEditor, 5, 1, false).
		AddItem(domainEditorInfo, 2, 1, false).
		AddItem(domainTable, 0, 1, false)

	searchFlexBox := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(searchEditor, 5, 1, false).
		AddItem(searchEditorInfo, 2, 1, false).
		AddItem(searchTable, 0, 1, false)

	regexFlexBox := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(regexEditor, 15, 1, false).
		AddItem(regexEditorInfo, 2, 1, false).
		AddItem(regexTable, 0, 1, false)

	resultsPage := tview.NewTextView().SetText("HELLO")

	helpString := `HELP/INFO
	PRESS TAB TO SELECT NEXT OPTION
	SHIFT+TAB TO SELECT PREV OPTION
	PRESS ENTER TO SELECT
	PRESS ESC TO RETURN TO PREVIOUS MENU
	`
	helpText := tview.NewTextView().SetText(helpString)

	pages := tview.NewPages().
		AddPage("help", helpText, true, true).
		AddPage("editorMenu", editorMenu, true, false).
		AddPage("domainEditor", domainFlexBox, true, false).
		AddPage("searchEditor", searchFlexBox, true, false).
		AddPage("regexEditor", regexFlexBox, true, false).
		AddPage("results", resultsPage, true, false)

	scanInfo.SetTextAlign(tview.AlignCenter).SetText("\nScanned: [red]0[white]\nFailed: [red]0[white]")

	//list menu options
	list := tview.NewList().SetSelectedBackgroundColor(tcell.ColorRed)
	editorMenu.SetSelectedBackgroundColor(tcell.ColorGray)
	list.AddItem("Help", "", 'a', func() {
		pages.SwitchToPage("help")
	}).
		AddItem("Edit Configuration", "", 'b', func() {
			pages.SwitchToPage("editorMenu")
			app.SetFocus(editorMenu)
			editorMenu.SetSelectedBackgroundColor(tcell.ColorRed)
			list.SetSelectedBackgroundColor(tcell.ColorGray)
		}).
		AddItem("View Findings", "", 'c', func() {
			pages.SwitchToPage("results")
		}).
		AddItem("Start/Stop Scanner", "", 'd', func() {
			if !scannerRunning {
				scannerRunning = true
				GUIInteratables.ImagePane.SetImage(*GUIInteratables.RedImage)
				scanInfo.SetText("RUNNING")
				wg.Add(1)
				yamlData = readDomainsFile("./config.yml")
				go initScan(ctx, GUIInteratables, yamlData)
			} else if scannerRunning {
				cancel()
				scanInfo.SetText("NOT RUNNING")
				scannerRunning = false
				GUIInteratables.ImagePane.SetImage(*GUIInteratables.Image)
				ctx, cancel = context.WithCancel(context.Background())
				*GUIInteratables.SuccessCount = 0
				*GUIInteratables.FailCount = 0
			}
		}).
		AddItem("Quit", "Press q to exit", 'q', func() {
			app.Stop()
		})

	editorMenu.AddItem("Edit Domain List", "", '1', func() {
		updateTable(domainTable, "d")
		pages.SwitchToPage("domainEditor")
		domainEditorInfo.SetText("")
		app.SetFocus(domainEditor)
	}).AddItem("Edit Search Terms", "", '2', func() {
		updateTable(searchTable, "s")
		pages.SwitchToPage("searchEditor")
		searchEditorInfo.SetText("")
		app.SetFocus(searchEditor)
	}).AddItem("Add Custom Regex", "", '3', func() {
		updateTable(regexTable, "r")
		pages.SwitchToPage("regexEditor")
		regexEditorInfo.SetText("")
		app.SetFocus(regexEditor)
	})

	//FORMS
	domainEditor.AddInputField("DOMAIN", "", 20, nil, nil).
		AddButton("ADD", func() {
			domainItem := domainEditor.GetFormItemByLabel("DOMAIN").(*tview.InputField).GetText()
			errMsg := false
			for _, domain := range yamlData.Domains {
				if domainItem == domain {
					domainEditorInfo.SetText("[red]DOMAIN ALREADY ADDED[white]")
					errMsg = true
					break
				}
			}
			if !errMsg {
				yamlData.Domains = append(yamlData.Domains, domainItem)

				updateYAMLFile(yamlData)
				updateTable(domainTable, "d")
				domainEditorInfo.SetText("[green]DOMAIN ADDED[white]")
			}
		}).
		AddButton("REMOVE", func() {
			domainItem := domainEditor.GetFormItemByLabel("DOMAIN").(*tview.InputField).GetText()
			found := false

			num, err := strconv.Atoi(domainItem)
			if err != nil {
				for i, domain := range yamlData.Domains {
					if domainItem == domain {
						yamlData.Domains[i] = yamlData.Domains[len(yamlData.Domains)-1]
						yamlData.Domains = yamlData.Domains[:len(yamlData.Domains)-1]
						updateYAMLFile(yamlData)
						updateTable(domainTable, "d")
						domainEditorInfo.SetText("[green]DOMAIN REMOVED[white]")
						found = true
						break
					}
				}
				if !found {
					domainEditorInfo.SetText("[red]DOMAIN NOT FOUND[white]")
				}
			} else {
				if num < len(yamlData.Domains) && num >= 0 {
					yamlData.Domains[num] = yamlData.Domains[len(yamlData.Domains)-1]
					yamlData.Domains = yamlData.Domains[:len(yamlData.Domains)-1]
					updateYAMLFile(yamlData)
					updateTable(domainTable, "d")
					domainEditorInfo.SetText("[green]DOMAIN REMOVED[white]")
				} else {
					domainEditorInfo.SetText("[red]INDEX OUT OF BOUNDS[white]")
				}
			}
		})

	searchEditor.AddInputField("SEARCH TERM", "", 20, nil, nil).
		AddButton("ADD", func() {
			searchTerm := searchEditor.GetFormItemByLabel("SEARCH TERM").(*tview.InputField).GetText()
			errMsg := false
			for _, searchT := range yamlData.SearchTerms {
				if searchTerm == searchT {
					searchEditorInfo.SetText("[red]SEARCH TERM ALREADY ADDED[white]")
					errMsg = true
					break
				}
			}
			if !errMsg {
				yamlData.SearchTerms = append(yamlData.SearchTerms, searchTerm)
				updateYAMLFile(yamlData)
				updateTable(searchTable, "s")
				searchEditorInfo.SetText("[green]SEARCH TERM ADDED[white]")
			}
		}).
		AddButton("REMOVE", func() {
			searchTerm := searchEditor.GetFormItemByLabel("SEARCH TERM").(*tview.InputField).GetText()
			found := false

			num, err := strconv.Atoi(searchTerm)
			if err != nil {
				for i, searchT := range yamlData.SearchTerms {
					if searchT == searchTerm {
						yamlData.SearchTerms[i] = yamlData.SearchTerms[len(yamlData.SearchTerms)-1]
						yamlData.SearchTerms = yamlData.SearchTerms[:len(yamlData.SearchTerms)-1]
						updateYAMLFile(yamlData)
						updateTable(searchTable, "s")
						searchEditorInfo.SetText("[green]SEARCH TERM REMOVED[white]")
						found = true
						break
					}
				}
				if !found {
					searchEditorInfo.SetText("[red]TERM NOT FOUND[white]")
				}
			} else {
				if num < len(yamlData.SearchTerms) && num >= 0 {
					yamlData.SearchTerms[num] = yamlData.SearchTerms[len(yamlData.SearchTerms)-1]
					yamlData.SearchTerms = yamlData.SearchTerms[:len(yamlData.SearchTerms)-1]
					updateYAMLFile(yamlData)
					updateTable(searchTable, "s")
					searchEditorInfo.SetText("[green]SEARCH TERM REMOVED[white]")
				} else {
					searchEditorInfo.SetText("[red]INDEX OUT OF BOUNDS[white]")
				}
			}
		})

	regexEditor.AddCheckbox("Emails", presetRegexStatus[0], func(checked bool) {
		yamlData.Find_emails = checked
		updateYAMLFile(yamlData)
	}).
		AddCheckbox("HTML Comments", presetRegexStatus[1], func(checked bool) {
			yamlData.Find_HTML_comments = checked
			updateYAMLFile(yamlData)
		}).
		AddCheckbox("JS Comments", presetRegexStatus[2], func(checked bool) {
			yamlData.Find_JS_comments = checked
			updateYAMLFile(yamlData)
		}).
		AddCheckbox("CSS Comments", presetRegexStatus[3], func(checked bool) {
			yamlData.Find_CSS_comments = checked
			updateYAMLFile(yamlData)
		}).
		AddInputField("CUST. REGEX", "", 20, nil, nil).
		AddButton("ADD", func() {
			regexTerm := regexEditor.GetFormItemByLabel("CUST. REGEX").(*tview.InputField).GetText()
			errMsg := false
			for _, regex := range yamlData.RegexTerms {
				if regex == regexTerm {
					regexEditorInfo.SetText("[red]REGEX ALREADY ADDED[white]")
					errMsg = true
					break
				}
			}
			if !errMsg {
				yamlData.RegexTerms = append(yamlData.RegexTerms, regexTerm)
				updateYAMLFile(yamlData)
				updateTable(regexTable, "r")
				regexEditorInfo.SetText("[green]REGEX ADDED[white]")
			}
		}).
		AddButton("REMOVE", func() {
			regexTerm := regexEditor.GetFormItemByLabel("CUST. REGEX").(*tview.InputField).GetText()
			found := false
			num, err := strconv.Atoi(regexTerm)
			if err != nil {
				for i, regex := range yamlData.RegexTerms {
					if regex == regexTerm {
						yamlData.RegexTerms[i] = yamlData.RegexTerms[len(yamlData.RegexTerms)-1]
						yamlData.RegexTerms = yamlData.RegexTerms[:len(yamlData.RegexTerms)-1]
						updateYAMLFile(yamlData)
						updateTable(regexTable, "r")
						regexEditorInfo.SetText("[green]REGEX REMOVED[white]")
						found = true
						break
					}
				}
				if !found {
					regexEditorInfo.SetText("[red]REGEX NOT FOUND[white]")
				}
			} else {
				if num < len(yamlData.RegexTerms) && num >= 0 {
					yamlData.RegexTerms[num] = yamlData.RegexTerms[len(yamlData.RegexTerms)-1]
					yamlData.RegexTerms = yamlData.RegexTerms[:len(yamlData.RegexTerms)-1]
					updateYAMLFile(yamlData)
					updateTable(regexTable, "r")
					regexEditorInfo.SetText("[green]REGEX REMOVED[white]")
				} else {
					regexEditorInfo.SetText("[red]INDEX OUT OF BOUNDS[white]")
				}
			}
		})

	centeredList := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(list, 0, 1, true).
		AddItem(nil, 0, 1, false)

	//get image
	imageFile0, err := os.Open("./image.png")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	imageFile1, err := os.Open("redImage.png")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	imageData0, err := png.Decode(imageFile0)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	imageData1, err := png.Decode(imageFile1)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	imagePane := tview.NewImage().SetImage(imageData0)

	info := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("PAGE STATS"), 1, 1, false).
		AddItem(scanInfo, 0, 1, false)
	main := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("MENU"), 0, 1, false).
		AddItem(centeredList, 0, 5, true).
		AddItem(imagePane, 0, 8, false)
	sideBar := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, false)

	//main.AddItem(buttonFlexBox)

	grid := tview.NewGrid().
		SetRows(5, 0).
		SetColumns(30, 0).
		SetBorders(true)

	// Layout for screens narrower than 100 cells (menu and side bar are hidden).
	grid.AddItem(info, 0, 0, 1, 2, 0, 0, false).
		AddItem(main, 1, 0, 1, 2, 0, 0, false).
		AddItem(sideBar, 0, 2, 2, 0, 0, 0, false)

	// Layout for screens wider than 100 cells.
	grid.AddItem(info, 0, 0, 1, 2, 0, 0, false).
		AddItem(main, 1, 0, 1, 2, 0, 100, false).
		AddItem(sideBar, 0, 2, 2, 1, 0, 100, false)

	//keyboard stuff
	editorMenu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyESC:
			currentPage, _ := pages.GetFrontPage()
			if currentPage == "editorMenu" {
				app.SetFocus(list)
				editorMenu.SetSelectedBackgroundColor(tcell.ColorGray)
				list.SetSelectedBackgroundColor(tcell.ColorRed)
			}
		}
		return event
	})
	domainEditor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyESC:
			currentPage, _ := pages.GetFrontPage()
			if currentPage == "domainEditor" {
				pages.SwitchToPage("editorMenu")
				app.SetFocus(editorMenu)
			}
		}
		return event
	})
	searchEditor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyESC:
			currentPage, _ := pages.GetFrontPage()
			if currentPage == "searchEditor" {
				pages.SwitchToPage("editorMenu")
				app.SetFocus(editorMenu)
			}
		}
		return event
	})
	regexEditor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyESC:
			currentPage, _ := pages.GetFrontPage()
			if currentPage == "regexEditor" {
				pages.SwitchToPage("editorMenu")
				app.SetFocus(editorMenu)
			}
		}
		return event
	})

	//store elements in GUIInteractables

	GUIInteratables.App = app
	GUIInteratables.ScanInfo = scanInfo
	GUIInteratables.SuccessCount = &successCount
	GUIInteratables.FailCount = &failCount
	GUIInteratables.ImagePane = imagePane
	GUIInteratables.Image = &imageData0
	GUIInteratables.RedImage = &imageData1
	GUIInteratables.Findings = &findings
	GUIInteratables.Results = resultsPage

	if err := app.SetRoot(grid, true).SetFocus(list).Run(); err != nil {
		panic(err)
	}

}

func updateTable(table *tview.Table, dataType string) {
	var dataArray []string
	if dataType == "d" {
		dataArray = readDomainsFile("./config.yml").Domains
	} else if dataType == "s" {
		dataArray = readDomainsFile("./config.yml").SearchTerms
	} else if dataType == "r" {
		dataArray = readDomainsFile("./config.yml").RegexTerms
	}

	table.Clear()
	for i, item := range dataArray {
		table.InsertRow(i).SetCellSimple(i, 0, strconv.Itoa(i)+" - "+item)
	}

}

func updateYAMLFile(yamlData yamlData) {
	data, err := yaml.Marshal(&yamlData)
	if err != nil {
		log.Fatal("Critical Error")
	}

	err = os.WriteFile("./config.yml", data, 0644)
	if err != nil {
		log.Fatal("Cannot write to file")
	}
}

func readDomainsFile(path string) yamlData {
	var config yamlData
	yamlData, err := os.ReadFile(path)
	errorOutput(err, true)

	err = yaml.Unmarshal(yamlData, &config)
	if err != nil {
		os.Exit(0)
	}
	errorOutput(err, true)

	return config
}
