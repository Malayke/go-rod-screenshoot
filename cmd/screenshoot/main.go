package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"

	_ "net/http/pprof"

	_ "github.com/mkevac/debugcharts"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/rod/lib/utils"
	"github.com/ysmood/gson"
)

type Body struct {
	Url            string `json:"url"`
	ScreenshotB64  string `json:"screenshootb64"`
	ResponseHeader string `json:"response_header"`
}

var DevToolsUrl = "ws://127.0.0.1:7317/"

// var DevToolsUrl = os.Getenv("DevToolsUrl")

func handleError(err error, url string) {
	var evalErr *rod.ErrEval
	if errors.Is(err, context.DeadlineExceeded) { // timeout error
		fmt.Printf("Error: access %s timeout\n", url)
	} else if errors.As(err, &evalErr) { // eval error
		fmt.Printf("access %s error, %v\n", url, evalErr.LineNumber)
	} else if err != nil {
		// fmt.Printf("access %s error, %s", url, err)
		fmt.Printf("access %s error\n", url)
	}
}

func BrowserPool(urlFile string) {
	pool := rod.NewBrowserPool(10)
	l := launcher.MustNewManaged(DevToolsUrl)
	create := func() *rod.Browser { return rod.New().Client(l.MustClient()).MustConnect() }
	var e proto.NetworkResponseReceived
	takeScreenshoot := func(target string) {
		err := rod.Try(func() {
			browser := pool.Get(create)
			defer pool.Put(browser)
			browser.MustIgnoreCertErrors(true)
			page := browser.MustIncognito().MustPage(target)
			wait := page.WaitEvent(&e)
			wait()
			page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
				UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36",
			})
			page.MustWindowFullscreen()
			page.Timeout(60 * time.Second)
			// page.Timeout(30 * time.Second).MustWaitLoad().MustScreenshot(fmt.Sprintf("%s.png", target[8:]))
			page.WaitLoad()
			img, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
				Format:      proto.PageCaptureScreenshotFormatJpeg,
				Quality:     gson.Int(50),
				FromSurface: false,
			})
			if err != nil {
				fmt.Println(err)
			}
			imgb64 := base64.StdEncoding.EncodeToString([]byte(img))
			result := Body{
				Url:            target,
				ScreenshotB64:  imgb64,
				ResponseHeader: utils.MustToJSON(e.Response.Headers),
			}
			//convert user struct to json
			body, _ := json.Marshal(result)

			//parse target url
			u, err := url.Parse(target)
			if err != nil {
				log.Fatal(err)
			}
			// extract domain from target then save to file with json format
			err = utils.OutputFile(fmt.Sprintf("%s.json", u.Hostname()), body)
			if err != nil {
				log.Fatal(err)
			}
		})
		handleError(err, target)
	}

	file, err := os.Open(urlFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	UrlRegexp := regexp.MustCompile(`https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`)
	scanner := bufio.NewScanner(file)
	// Run jobs concurrently
	wg := sync.WaitGroup{}

	for scanner.Scan() {
		url := scanner.Text()
		if !UrlRegexp.MatchString(url) {
			fmt.Printf("%s is not a regular url\n", url)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			takeScreenshoot(url)
		}()
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	wg.Wait()

	// cleanup pool
	pool.Cleanup(func(p *rod.Browser) { p.MustClose() })
}

func main() {
	go func() {
		ip := "0.0.0.0:6060"
		if err := http.ListenAndServe(ip, nil); err != nil {
			fmt.Printf("start pprof failed on %s\n", ip)
			os.Exit(1)
		}
	}()
	// url file
	var UrlFile string
	flag.StringVar(&UrlFile, "file", "", "Path to url file")
	flag.Parse()
	var Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage : take_screenshoot -file <file>")
		flag.PrintDefaults()
	}
	if UrlFile != "" {
		BrowserPool(UrlFile)
	} else {
		fmt.Fprintln(os.Stderr, "need forvide url file")
		Usage()
		os.Exit(1)
	}

}
