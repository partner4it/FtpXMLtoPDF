package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/partner4it/secure"
)

// Struct definition of the local configfile
type ConfigSettings struct {
	FtpServer   string `json:"ftpServer"`
	FtpUser     string `json:"ftpUser"`
	FtpPassword string `json:"ftpPassword"`
	FtpDir      string `json:"ftpDir"`
	FtpTLS      bool   `json:"ftpTLS"`
	FtpFilter   string `json:"ftpFilter"`
	FtpRemove   bool   `json:"ftpRemove"`
	TplName     string `json:"tplName"`
	OutputDir   string `json:"outputDir"`
	TempFile    string `json:"tempFile"`
	BrowserPath string `json:"browserPath"`
	LogFile     string `json:"logFile"`
}

// Struct definition of the remote configfile
type RemoteConfig struct {
	LastProcessed time.Time `json:"lastProcessed"`
}

// The local config
var config = ConfigSettings{"", "", "", "", false, "*.xml", false,
	BaseName + ".tpl.html", "", "." + BaseName + ".tmp.html", "", ""}

// The remote config
var remoteConfig = RemoteConfig{}

// The module version, will be replaced during build
var Version string = "code"

// The build version, will be replaced during build
var BuildVersion string = "local"

// The base name of all default files used
var BaseName = "ftpxmltopdf"

// The default local configFile
var configFile string = "." + BaseName + ".cfg"

// Should we ignore faults
var ignoreFlag = false

// Should we keep the temp file
var keepTemp = false

// Should we do a remote reset of the config file
var remoteReset = false

// Should we save the settings
var saveFlag = false

// Should we just run conversion test for template
var localFile = ""

// PipeMode will allow a localFile to be read using a commandline pipe
var pipeMode = false

// The name of the log file to use
var silent = false //When set en logfile is empty we will not show any logging

var SecretKey string = "N1PCdw3M2B1TfJhoaY2mL736p2vCUc47"

// Read os flags and setup log file
func initVars() {
	//Overwrite the SecretKey used within Secure
	secure.SecretKey = SecretKey
	//Check if the is a config file with settings

	//Because whe have to load the configfile before flags is finish we need to parse it by manually
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-configFile" && i+1 < len(os.Args) {
			configFile = os.Args[i+1]
			break
		}
		if strings.HasPrefix(os.Args[i], "-configFile=") {
			configFile = strings.TrimPrefix(os.Args[i], "-configFile=")
			break
		}
	}

	//Load the config file if exists
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		//There is a config file read it
		content, err := os.ReadFile(configFile)
		if err != nil {
			log.Fatal(err)
		}
		contentstr, err := secure.Decrypt(string(content))
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
	flag.StringVar(&configFile, "configFile", configFile, "The configFile to use.")
	flag.StringVar(&config.FtpServer, "ftpServer", config.FtpServer, "The ftpServer to connect to.")
	flag.StringVar(&config.FtpUser, "ftpUser", config.FtpUser, "The User used during connecting to the ftpServer.")
	//Prevent that password is shown during default parameters
	var pwdValue string = ""
	if config.FtpPassword != "" {
		pwdValue = "******"
	}
	pwd := flag.String("ftpPassword", pwdValue, "The password used during connecting to the ftpServer.")
	flag.StringVar(&config.FtpDir, "ftpDir", config.FtpDir, "The directory changed to on the ftpServer after valid login.")
	flag.BoolVar(&config.FtpTLS, "ftpTLS", config.FtpTLS, "Should we use standard ftp server with TLS")
	flag.BoolVar(&config.FtpRemove, "ftpRemove", config.FtpRemove, "Should we remove file after succesfull processing")
	flag.StringVar(&config.FtpFilter, "ftpFilter", config.FtpFilter, "The filter to select xml files")
	flag.StringVar(&config.TplName, "tplName", config.TplName, "The name of the template to use during conversion.")
	flag.StringVar(&config.OutputDir, "outputDir", config.OutputDir, "The location where pdf's are stored.")
	flag.StringVar(&config.TempFile, "tempFile", config.TempFile, "The filename and location of temp file.")
	flag.StringVar(&localFile, "localFile", localFile, "The location of the local xml file to convert to pdf's.")
	flag.StringVar(&config.LogFile, "logFile", config.LogFile, "The name of the log file to use instead of console")
	flag.BoolVar(&saveFlag, "save", saveFlag, "Should we save to the encypted logfile")
	flag.BoolVar(&ignoreFlag, "ignore", ignoreFlag, "When set faults will be ignored")
	flag.BoolVar(&keepTemp, "keepTemp", keepTemp, "When set we will keep the tempfile")
	flag.BoolVar(&remoteReset, "remoteReset", remoteReset, "Should we do a remote reset of the configfile")
	flag.BoolVar(&silent, "silent", silent, "When set en logFile is empty we will not show any logging")
	flag.StringVar(&config.BrowserPath, "browser", config.BrowserPath, "The full path to the chrome or edge exceutalbe")

	flag.Parse()          // after declaring flags we need to call it
	if *pwd != pwdValue { //Check if password changed
		config.FtpPassword = *pwd
	}
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
		contentStr, err := secure.Encrypt(string(content))
		if err != nil {
			fatalln(err)
		}
		err = os.WriteFile(configFile, []byte(contentStr), 0644)
		if err != nil {
			fatalln(err)
		}

	}

	//Autodetect pipeMode
	fi, err := os.Stdin.Stat()
	if err != nil {
		fatalln(err)
	}
	if fi.Mode()&os.ModeNamedPipe != 0 {
		pipeMode = true
	}

	//Check if we should write log logfile instead of console
	if silent || pipeMode {
		log.SetOutput(io.Discard)
	} else if config.LogFile != "" { //Write to file instead of console
		f, err := os.OpenFile(config.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening logfile: %v", err)
		}

		log.SetOutput(f)
	}
}

// fatalln error handing with user response
func fatalln(v ...any) {
	if !ignoreFlag {
		log.Println(v...)
		fmt.Print("There was a problem!!! Read the log carefully and press ENTER key to exit.")
		fmt.Scanln()
		os.Exit(1)
	}
	log.Fatalln(v...)
}

// Get filename only
func fileNameWithoutExt(fileName string) string {
	return strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
}

// Remove a file in nice way when exits
func removeFile(filename string) {
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		err = os.Remove(filename)
		if err != nil {
			fatalln("Failed removing file (" + filename + ")")
		}
	}
}

// Remove the tempfile if exists
func removeTempFile() {
	removeFile(config.TempFile)
}
