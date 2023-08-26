package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"ftpxmltopdf/sftp"
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

	xmlToJson "github.com/basgys/goxml2json"
	strip "github.com/grokify/html-strip-tags-go"
)

//The module version, will be replaced during build
var Version string = "code"

//The build version, will be replaced during build
var BuildVersion string = "local"

//The base name of all default files used
var BaseName = "ftpxmltopdf"

//This key will be changed during build
var SecretKey string = "N1PCdw3M2B1TfJhoaY2mL736p2vCUc47"

type ConfigSettings struct {
	FtpServer   string `json:"ftpServer"`
	FtpUser     string `json:"ftpUser"`
	FtpPassword string `json:"ftpPassword"`
	FtpDir      string `json:"ftpDir"`
	FtpTLS      bool   `json:"ftpTLS"`
	FtpFilter   string `json:"ftpFilter"`
	TplName     string `json:"tplName"`
	OutputDir   string `json:"outputDir"`
	TempFile    string `json:"tempFile"`
}

type RemoteConfig struct {
	LastProcessed time.Time `json:"lastProcessed"`
}

//The config
var config = ConfigSettings{"", "", "", "", false, "*.xml", BaseName + ".tpl", "", "." + BaseName + ".tmp.html"}
var remoteConfig = RemoteConfig{}

//Should we ignore faults
var ignoreFlag = false

//Should we keep the temp file
var keepTemp = false

//Should we do a remote reset of the config file
var remoteReset = false

//Should we save the settings
var saveFlag = false

//Should we just run conversion test for template
var testFile = ""

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
		fmt.Print("There was a problem, read logfile and take action, exit by pressing the ENTER key")
		fmt.Scanln()
		os.Exit(1)
	}
	log.Fatalln(v...)
}

//Read os flags
func Init() {
	//Check if the is a config file with settings

	if _, err := os.Stat("." + BaseName + ".cfg"); !os.IsNotExist(err) {
		//There is a config file read it
		content, err := ioutil.ReadFile("." + BaseName + ".cfg")
		if err != nil {
			log.Fatal(err)
		}
		contentstr, err := decrypt(string(content))
		//Check if decrypt vaild
		if err == nil {
			err = json.Unmarshal([]byte(contentstr), &config)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Println("Config file invalid starting with default config")
		}
	}

	// flags declaration using flag package
	version := flag.Bool("version", false, "prints current version ("+Version+")")
	flag.StringVar(&config.FtpServer, "ftpServer", config.FtpServer, "The ftpServer to connect to.")
	flag.StringVar(&config.FtpUser, "ftpUser", config.FtpUser, "The User used during connecting to the ftpServer.")
	flag.StringVar(&config.FtpPassword, "ftpPassword", config.FtpPassword, "The Password used during connecting to the ftpServer.")
	flag.StringVar(&config.FtpDir, "ftpDir", config.FtpDir, "The Directory changed to on the ftpServer after valid login.")
	flag.BoolVar(&config.FtpTLS, "ftpTLS", config.FtpTLS, "Should we use standard ftp with TLS")
	flag.StringVar(&config.FtpFilter, "ftpFilter", config.FtpFilter, "The filter to select xml files")
	flag.StringVar(&config.TplName, "tplName", config.TplName, "The name of the template to use during conversion.")
	flag.StringVar(&config.OutputDir, "outputDir", config.OutputDir, "The location where pdf's are stored.")
	flag.StringVar(&config.TempFile, "tempFile", config.TempFile, "The filename and location of temp file.")
	flag.StringVar(&testFile, "testFile", testFile, "The location of the xml test file to convert to pdf's.")
	flag.BoolVar(&saveFlag, "save", saveFlag, "Should we save to the encypted logfile")
	flag.BoolVar(&ignoreFlag, "ignore", ignoreFlag, "When set faults will be ignored")
	flag.BoolVar(&keepTemp, "keepTemp", keepTemp, "When set we will keep the tempfile")
	flag.BoolVar(&remoteReset, "remoteReset", remoteReset, "Should we do a remote reset of the configfile")

	flag.Parse() // after declaring flags we need to call it
	if *version {
		fmt.Println("Version v"+Version, ", Build:", BuildVersion)
		os.Exit(0)
	}

	//Save the config file
	if saveFlag {
		content, err := json.Marshal(config)
		if err != nil {
			fatalln(err)
		}
		contentStr, err := encrypt(string(content))
		if err != nil {
			fatalln(err)
		}
		err = ioutil.WriteFile("."+BaseName+".cfg", []byte(contentStr), 0644)
		if err != nil {
			fatalln(err)
		}

	}
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
				buf, _, err := page.PrintToPDF().WithPrintBackground(true).Do(ctx)
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

func convert(fileIn string, fileOut string) error {
	xml, err := os.ReadFile(fileIn)
	if err != nil {
		return err
	}
	// Convert the xml to Json
	json, err := xmlToJson.Convert(strings.NewReader(string(xml)))
	if err != nil {
		return err
	}

	// Make for the bytebuffer a string and run it through the template
	jsonString := json.String()
	content, err := toTemplate(config.TplName, &jsonString)
	if err != nil {
		return err
	}

	//Save the content to the tempfile
	if err = ioutil.WriteFile(config.TempFile, []byte(content), 0644); err != nil {
		return err
	}

	// Convert the tempfile to outputfile pdf
	path, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = toPDF(path+"/"+config.TempFile, fileOut); err != nil {
		return err
	}
	return nil
}

//Remove the tempfile if exists
func removeTempFile() {
	if _, err := os.Stat(config.TempFile); !os.IsNotExist(err) {
		err = os.Remove(config.TempFile)
		if err != nil {
			fatalln("Failed removing temp file (" + config.TempFile + ")")
		}
	}
}

//Get filename only
func fileNameWithoutExtTrimSuffix(fileName string) string {
	return strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
}

func main() {
	Init()
	//Check if the temp file exists and we are not ignoring
	if _, err := os.Stat(config.TempFile); !os.IsNotExist(err) && !ignoreFlag {
		fatalln("Looks like the last run was not successfull (" + config.TempFile + " exists)")
	}

	//Check if output dir exists in not create it
	if config.OutputDir != "" {
		if _, err := os.Stat(config.OutputDir); os.IsNotExist(err) {
			if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
				fatalln("Failed to create output directory", err)
			}
		}
		//Add slash to ensure valid directory
		config.OutputDir = config.OutputDir + "/"
	}

	//Check if ftpdir is set, add slash
	if config.FtpDir != "" {
		config.FtpDir = config.FtpDir + "/"
	}

	//Check if we should only do testing of converion
	if testFile != "" {
		//Run the conversion
		if err := convert(testFile, config.OutputDir+fileNameWithoutExtTrimSuffix(testFile)+".pdf"); err != nil {
			fatalln("Failed to convert file ("+testFile+")", err)
		}
		removeTempFile()
		return
	}

	if config.FtpServer == "" || config.FtpUser == "" || config.FtpPassword == "" {
		fatalln("Please ensure that ftpServer,ftpUser,ftpPassword are set ")
	}

	//Login
	ftpconfig := sftp.Config{
		Username: config.FtpUser,
		Password: config.FtpPassword, // required only if password authentication is to be used
		//PrivateKey:   string(pk), // required only if private key authentication is to be used
		Server: config.FtpServer,
		//KeyExchanges: []string{"diffie-hellman-group-exchange-sha256", "diffie-hellman-group14-sha256"}, // optional
		TLS:     config.FtpTLS,    //Should we use FTP with TLS instead of SSH
		Timeout: time.Second * 30, // 0 for not timeout
	}
	client, err := sftp.New(ftpconfig)
	if err != nil {
		fatalln(err)
	}
	defer client.Close()

	var lockFile string = "." + BaseName + ".lck"
	//Search lockfile on ftpserver
	files, err := client.Glob(lockFile)
	if err != nil {
		fatalln(err)
	}

	// Check if there is a lock file in response, skip on ignore
	if len(files) != 0 && !ignoreFlag {
		fatalln("Lockfile (" + lockFile + ") found on ftpserver")
	}

	// Write the lock file
	err = client.UploadFile(lockFile, strings.NewReader(string("lock")))
	if err != nil {
		fatalln(err)
	}

	// Read the remoteConfig file if exist and not reset
	var configFile string = "." + BaseName + ".cfg"
	if !remoteReset {
		cf, err := client.Download(configFile)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				fatalln(err)
			}
		} else {
			defer cf.Close()
			content, err := ioutil.ReadAll(cf)
			if err != nil {
				fatalln(err)
			}
			err = json.Unmarshal(content, &remoteConfig)
			if err != nil {
				fatalln(err)
			}
		}
	}

	// Scan ftpDir using the filter and limiting to greater than lastprocess
	files, err = client.Glob(config.FtpDir + config.FtpFilter)
	if err != nil {
		fatalln(err)
	}
	// Exit if no files where found
	if len(files) == 0 {
		return
	}

	// Create output location if not exits
	// For each file
	lastProcessed := remoteConfig.LastProcessed
	for _, remoteFile := range files {
		// Get remote file stats to check if it should be processed
		fileInfo, err := client.Info(remoteFile)
		if err != nil {
			log.Fatalln(err)
		}
		//Check if file is newer the our lastProcess date
		if fileInfo.ModTime().After(remoteConfig.LastProcessed) {
			// update the max of lastprocess
			if fileInfo.ModTime().After(lastProcessed) {
				lastProcessed = fileInfo.ModTime()
			}
			// Download the file into the tempfile
			df, err := client.Download(remoteFile)
			if err != nil {
				fatalln(err)
			}
			defer df.Close()
			content, err := ioutil.ReadAll(df)
			if err != nil {
				fatalln(err)
			}
			err = ioutil.WriteFile(config.TempFile, content, 0644)
			if err != nil {
				fatalln(err)
			}

			//Run the conversion on the tempfile
			if err := convert(config.TempFile, config.OutputDir+fileNameWithoutExtTrimSuffix(remoteFile)+".pdf"); err != nil {
				fatalln("Failed to convert file ("+remoteFile+")", err)
			}
		}
	}
	//Save the lastProcessed date
	remoteConfig.LastProcessed = lastProcessed
	content, err := json.Marshal(remoteConfig)
	if err != nil {
		fatalln(err)
	}
	// Write back the config file
	err = client.UploadFile(configFile, strings.NewReader(string(content)))
	if err != nil {
		fatalln(err)
	}

	// Remove the lock file
	err = client.Remove(lockFile)
	if err != nil {
		fatalln(err)
	}
	// Remove the temp file
	if !keepTemp {
		removeTempFile()
	}

}
