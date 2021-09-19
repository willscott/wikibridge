package lib

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/d4l3k/wikigopher/wikitext"
	"golang.org/x/net/html"
)

type QueryResponse struct {
	Batchcomplete string             `json:"batchcomplete"`
	Query         QueryResponseQuery `json:"query"`
}

type QueryResponseQuery struct {
	Pages QueryResponsePages `json:"pages"`
}

type QueryResponsePages map[int]QueryResponsePage

type QueryResponsePage struct {
	PageID    int                    `json:"pageid"`
	NS        int                    `json:"ns"`
	Title     string                 `json:"title"`
	Revisions QueryResponseRevisions `json:"revisions,omitempty"`
}

type QueryResponseRevisions []QueryResponseRevision

type QueryResponseRevision struct {
	RevID     int           `json:"revid,omitempty"`
	ParentID  int           `json:"parentid,omitempty"`
	User      string        `json:"user,omitempty"`
	Timestamp string        `json:"timestamp,omitempty"`
	Comment   string        `json:"comment,omitempty"`
	Slots     RevisionSlots `json:"slots,omitempty"`
}

type RevisionSlots map[string]RevisionSlot

type RevisionSlot struct {
	Model  string `json:"contentmodel"`
	Format string `json:"contentformat"`
	Text   string `json:"*"`
}

// getText extracts part of a wikimedia wiki page.
func GetText(wikiBase, pageName, sectionTitle string) (string, error) {
	text, err := GetPageText(wikiBase, pageName)
	if err != nil {
		return "", err
	}

	tb := []byte(text)
	log.SetOutput(ioutil.Discard)
	v, err := wikitext.Parse("file.wikitext",
		append(tb, '\n'),
		wikitext.GlobalStore("len", len(text)),
		wikitext.GlobalStore("text", tb),
		wikitext.Recover(false),
		wikitext.Debug(false),
	)
	if err != nil {
		return "", err
	}

	switch f := v.(type) {
	case *html.Node:
		nodes, err := htmlquery.QueryAll(f, "//h2")
		if err != nil {
			return "", err
		}
		for _, n := range nodes {
			if strings.Contains(htmlquery.InnerText(n), sectionTitle) {
				headerType := n.Data
				outText := ""
				for {
					n = n.NextSibling
					if n.Type == html.ElementNode && n.Data == headerType {
						break
					}
					line := htmlquery.InnerText(n)
					if len(strings.TrimSpace(line)) > 0 {
						outText += fmt.Sprintf("* %s\n", strings.TrimSpace(line))
					}
				}
				return outText, nil
			}
		}
		return "", fmt.Errorf("failed to find section")
	default:
		return "", fmt.Errorf("failed to parse body")
	}
}

// GetLatestRevision gets the ID of a revision for a given page ID
func GetLatestRevision(wikiBase string, pageID int) (int, error) {
	revURL := fmt.Sprintf("%s/api.php?action=query&format=json&prop=revisions&pageids=%d", wikiBase, pageID)
	resp, err := http.Get(revURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("query failed with status %d", resp.StatusCode)
	}

	var qr QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return 0, err
	}

	for _, page := range qr.Query.Pages {
		for _, r := range page.Revisions {
			return r.RevID, nil
		}
		return 0, fmt.Errorf("no revisions found")
	}
	return 0, fmt.Errorf("page not found")
}

// GetPageID gets the ID of a page with a given title in the wiki.
func GetPageID(wikiBase, pageName string) (int, error) {
	latestURL := fmt.Sprintf("%s/api.php?action=query&format=json&titles=%s", wikiBase, url.QueryEscape(pageName))
	resp, err := http.Get(latestURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("query failed with status %d", resp.StatusCode)
	}

	var qr QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return 0, err
	}

	// get the first page ID
	for _, page := range qr.Query.Pages {
		return page.PageID, nil
	}
	return 0, fmt.Errorf("page not found")
}

// GetPageText gets the text contet of a page.
func GetPageText(wikiBase, pageName string) (string, error) {
	id, err := GetPageID(wikiBase, pageName)
	if err != nil {
		return "", err
	}

	rev, err := GetLatestRevision(wikiBase, id)
	if err != nil {
		return "", err
	}

	contentURL := fmt.Sprintf("%s/api.php?action=query&format=json&revids=%d&prop=revisions&rvslots=*&rvprop=content", wikiBase, rev)
	resp, err := http.Get(contentURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("query failed with status %d", resp.StatusCode)
	}

	var qr QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return "", err
	}

	for _, page := range qr.Query.Pages {
		for _, r := range page.Revisions {
			slots := r.Slots
			if m, ok := slots["main"]; ok {
				return m.Text, nil
			}
			return "", fmt.Errorf("content not found")
		}
		return "", fmt.Errorf("revision not found")
	}
	return "", fmt.Errorf("page not found")
}
