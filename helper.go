package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	strip "github.com/grokify/html-strip-tags-go"
)

func encrypt(plaintext string) (string, error) {
	aes, err := aes.NewCipher([]byte(SecretKey[0:32]))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return "", err
	}

	// We need a 12-byte nonce for GCM (modifiable if you use cipher.NewGCMWithNonceSize())
	// A nonce should always be randomly generated for every encryption.
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return "", err
	}

	// ciphertext here is actually nonce+ciphertext
	// So that when we decrypt, just knowing the nonce size
	// is enough to separate it from the ciphertext.
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return string(ciphertext), nil
}

func decrypt(ciphertext string) (string, error) {
	aes, err := aes.NewCipher([]byte(SecretKey[0:32]))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return "", err
	}

	// Since we know the ciphertext is actually nonce+ciphertext
	// And len(nonce) == NonceSize(). We can separate the two.
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, []byte(nonce), []byte(ciphertext), nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

//fatalln error handing with user response
func fatalln(v ...any) {
	if !ignoreFlag {
		log.Println(v...)
		fmt.Print("There was a problem!!! Read the log carefully and press ENTER key to exit.")
		fmt.Scanln()
		os.Exit(1)
	}
	log.Fatalln(v...)
}

//Get filename only
func fileNameWithoutExtTrimSuffix(fileName string) string {
	return strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
}

//Function used to convert the tempory html file to the pdf
func toPDF(htmlfile string, pdffile string) error {

	var pdfGrabber = func(url string, sel string, res *[]byte) chromedp.Tasks {
		return chromedp.Tasks{
			emulation.SetUserAgentOverride("WebScraper 1.0"),
			chromedp.Navigate(url),
			// wait for footer element is visible (ie, page is loaded)
			// chromedp.ScrollIntoView(`footer`),
			chromedp.WaitVisible(`body`, chromedp.ByQuery),
			// chromedp.Text(`h1`, &res, chromedp.NodeVisible, chromedp.ByQuery),
			chromedp.ActionFunc(func(ctx context.Context) error {
				buf, _, err := page.PrintToPDF().
					WithDisplayHeaderFooter(true).
					//WithMarginLeft(0).
					//WithMarginRight(0).
					//WithHeaderTemplate(`<div style="font-size:8px;width:100%;text-align:center;"><span class="title"></span> -- <span class="url"></span></div>`).
					WithHeaderTemplate(`<div style="font-size:8px;width:100%;text-align:center;"></div>`).
					WithFooterTemplate(`<div style="font-size:8px;width:100%;text-align:center;margin: 0mm 8mm 0mm 8mm;"><span style="float:left"><span class="title"></span> - <span >` + filepath.Base(pdffile) + `</span></span><span style="float:right">(<span class="pageNumber"></span> / <span class="totalPages"></span>)</span></div>`).
					WithPrintBackground(true).Do(ctx)
				if err != nil {
					return err
				}
				*res = buf
				return nil
			}),
		}
	}

	taskCtx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	var pdfBuffer []byte
	if err := chromedp.Run(taskCtx, pdfGrabber("file://"+htmlfile, "body", &pdfBuffer)); err != nil {
		return err
	}
	if err := ioutil.WriteFile(pdffile, pdfBuffer, 0644); err != nil {
		return err
	}
	return nil
}

//Apply the json data to the template
func toTemplate(tplName string, data *string) (string, error) {
	t, err := template.New(filepath.Base(tplName)).Funcs(template.FuncMap{
		"now": time.Now,
		"inc": func(n int) int {
			return n + 1
		},
		"strip": func(html string) string {
			data := strip.StripTags(html)
			data = strings.ReplaceAll(data, "&nbsp;", "")
			return data
		},
		"marshal": func(jsonData ...interface{}) string {
			marshaled, _ := json.MarshalIndent(jsonData[0], "", "   ")
			return string(marshaled)
		},
		"slice": func(args ...interface{}) []interface{} {
			return args
		},
	}).ParseFiles(tplName)
	if err != nil {
		return "", err
	}
	tplData := "{\"data\" :" + *data + "}"
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(tplData), &m); err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, m); err != nil {
		return "", err
	}
	tplData = tpl.String()
	return tplData, nil
}
