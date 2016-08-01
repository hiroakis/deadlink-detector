package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var elements = map[string]string{
	"a":      "href",
	"link":   "href",
	"script": "src",
	"img":    "src",
}

var client = http.Client{Timeout: 10 * time.Second}

func check(url string) (int, int, error) {
	resp, err := client.Get(url)
	if err != nil {
		return -1, 0, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, 0, err
	}
	return resp.StatusCode, len(b), nil
}

func hasMember(link string, ulinks []string) bool {
	for _, l := range ulinks {
		if link == l {
			return true
		}
	}
	return false
}

func removeDuplicatedValue(links []string) []string {
	ulinks := make([]string, 0, len(links))
	for _, link := range links {
		if !hasMember(link, ulinks) {
			ulinks = append(ulinks, link)
		}
	}
	return ulinks
}

var relativePath = regexp.MustCompile(`^[^http\.+][a-zA-Z0-9]`)
var absPath = regexp.MustCompile(`^http[s.]://`)

func formatting(targetPage, link string) string {

	var result string

	u, err := url.Parse(targetPage)
	if err != nil {
		fmt.Println(err)
	}

	// href="//xxxx"
	if strings.HasPrefix(link, "//") {
		result = fmt.Sprintf("%s:%s", u.Scheme, link)
		// href="/xxxx"
	} else if strings.HasPrefix(link, "/") {
		result = fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, link)
		// href="./xxxx"
	} else if strings.HasPrefix(link, "./") {
		result = fmt.Sprintf("%s://%s%s%s", u.Scheme, u.Host, u.Path, strings.Trim(link, "./"))
		// href="../xxxx"
	} else if strings.HasPrefix(link, "../") {
		// TODO: refactor, fix
		pathStrings := strings.Split(u.Path, "/")
		var ret string
		for _, v := range pathStrings[:len(pathStrings)-1] {
			ret = ret + v + "/"
		}
		result = fmt.Sprintf("%s://%s%s%s", u.Scheme, u.Host, ret, strings.Trim(link, "../"))
		// href="http://xxxxx"
	} else if len(absPath.FindAllStringSubmatch(link, -1)) != 0 {
		result = link
		// href="xxxxx/xxxxx/xxxx"
	} else if len(relativePath.FindAllStringSubmatch(link, -1)) != 0 {
		result = fmt.Sprintf("%s://%s%s%s", u.Scheme, u.Host, u.Path, link)
	}

	return result
}

func getLinks(targetPage string) ([]string, error) {
	var links []string

	doc, _ := goquery.NewDocument(targetPage)
	for element, attr := range elements {
		doc.Find(element).Each(func(_ int, s *goquery.Selection) {
			link, _ := s.Attr(attr)
			if link == "" {
				return
			}
			result := formatting(targetPage, link)
			links = append(links, result)
		})
	}
	return links, nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var targetPage string
	flag.StringVar(&targetPage, "url", "", "target url")
	flag.Parse()

	links, err := getLinks(targetPage)
	if err != nil {
		fmt.Println(err)
	}
	ulinks := removeDuplicatedValue(links)

	ch := make(chan bool)
	for _, link := range ulinks {
		go func(link string) {
			code, size, err := check(link)
			if err != nil {
				fmt.Println(fmt.Sprintf("ERROR(%s): %v", link, err))
				ch <- false
			} else {
				fmt.Println(fmt.Sprintf("%d %d %s", code, size, link))
				ch <- true
			}
		}(link)
	}

	for i := 0; i < len(ulinks); i++ {
		<-ch
	}
	close(ch)
}
