package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"ftpxmltopdf/sftp"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	xmlToJson "github.com/basgys/goxml2json"
)

//The module version, will be replaced during build
var Version string = "code"

//The build version, will be replaced during build
var BuildVersion string = "local"

//The base name of all default files used
var BaseName = "ftpxmltopdf"

//This key will be changed during build
var SecretKey string = "N1PCdw3M2B1TfJhoaY2mL736p2vCUc47"

//The default local configFile
var configFile string = "." + BaseName + ".cfg"

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
}

type RemoteConfig struct {
	LastProcessed time.Time `json:"lastProcessed"`
}

//The config
var config = ConfigSettings{"", "", "", "", false, "*.xml", false, BaseName + ".tpl.html", "", "." + BaseName + ".tmp.html"}
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

//Read os flags
func Init() {
	//Check if the is a config file with settings

	flag.StringVar(&configFile, "configFile", configFile, "The configFile to use.")
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		//There is a config file read it
		content, err := ioutil.ReadFile(configFile)
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
	flag.BoolVar(&config.FtpTLS, "ftpTLS", config.FtpTLS, "Should we use standard ftp server with TLS")
	flag.BoolVar(&config.FtpRemove, "ftpRemove", config.FtpRemove, "Should we remove file after succesfull processing")
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
		err = ioutil.WriteFile(configFile, []byte(contentStr), 0644)
		if err != nil {
			fatalln(err)
		}

	}
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

func main() {
	Init()
	log.Println(BaseName, "v"+Version+"-"+BuildVersion)
	log.Println("Using configFile", configFile)
	//Check if the temp file exists and we are not ignoring
	if _, err := os.Stat(config.TempFile); !os.IsNotExist(err) {
		if !ignoreFlag && !keepTemp {
			fatalln("Looks like the last run was not successfull (" + config.TempFile + " exists)")
		} else {
			log.Println("The local tempfile is still present, but is ignored")
		}
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
		log.Println("Using testfile", testFile)
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
	if len(files) != 0 {
		if !ignoreFlag {
			fatalln("Lockfile (" + lockFile + ") found on ftpserver")
		} else {
			log.Println("Lockfile found on remote server, but ignored")
		}
	}

	// Write the lock file
	err = client.UploadFile(lockFile, strings.NewReader(string("lock")))
	if err != nil {
		fatalln(err)
	}
	log.Println("Lockfile placed on ftpserver")

	// Read the remoteConfig file if exist and not reset
	if !remoteReset {
		cf, err := client.Download(filepath.Base(configFile))
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
		log.Println("Remote configfile read", filepath.Base(configFile))
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
			log.Println("Downloaded new xmlfile", remoteFile)
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
			log.Println("Created PDF", config.OutputDir+fileNameWithoutExtTrimSuffix(remoteFile)+".pdf")
			//Check if we should remove file
			if config.FtpRemove {
				err = client.Remove(remoteFile)
				if err != nil {
					log.Println("Removal of remotefile failed", remoteFile)
					fatalln(err)
				}
				log.Println("Removed remotefile succesfully", remoteFile)
			}
		}
	}
	//Save the lastProcessed date
	if remoteConfig.LastProcessed != lastProcessed {
		remoteConfig.LastProcessed = lastProcessed
		content, err := json.Marshal(remoteConfig)
		if err != nil {
			fatalln(err)
		}
		// Write back the config file
		err = client.UploadFile(filepath.Base(configFile), strings.NewReader(string(content)))
		if err != nil {
			fatalln(err)
		}
		log.Println("Remote configfile uploaded", filepath.Base(configFile))
		// Remove the temp file
		if !keepTemp {
			removeTempFile()
		} else {
			log.Println("As requested is the tempfile not removed", config.TempFile)
		}
	} else {
		log.Println("No new files found to process")
	}

	// Remove the lock file
	err = client.Remove(lockFile)
	if err != nil {
		fatalln(err)
	}
	log.Println("Remote lockfile removed")
	log.Println("Finished processing")
}
