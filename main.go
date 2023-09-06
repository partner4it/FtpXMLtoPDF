package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	html "html/template"

	"github.com/partner4it/sftp"
	"github.com/partner4it/template"
)

func main() {
	initVars()

	//Some handy extra functions for template engine
	customFuncs := html.FuncMap{
		"toTime": func(timeStamp string) time.Time {
			t, err := time.Parse("2006-01-02T15:04:05.999999", timeStamp)
			if err != nil {
				log.Println(err)
			}
			return t
		},
		"asRange": func(v any) any {
			//Incase of nil return empty array
			if v == nil {
				return make([]any, 0)
			}
			//Incase of array return the array
			if reflect.TypeOf(v).Kind() == reflect.Array ||
				reflect.TypeOf(v).Kind() == reflect.Slice {
				return v
			}
			//In all othercase wrap de element in an array
			r := make([]any, 1)
			r[0] = v
			return r
		},
		"iif": func(c any, t any, f any) any {
			if r, ok := c.(bool); ok {
				if r {
					return t
				} else {
					return f
				}
			}
			if r, ok := c.(string); ok {
				if r != "" {
					return t
				} else {
					return f
				}
			}
			if r, ok := c.(int); ok {
				if r != 0 {
					return t
				} else {
					return f
				}
			}
			if r, ok := c.(float32); ok {
				if r != 0 {
					return t
				} else {
					return f
				}
			}
			if r, ok := c.(float64); ok {
				if r != 0 {
					return t
				} else {
					return f
				}
			}
			if c != nil {
				return t
			} else {
				return f
			}
		},
	}

	//Print some log information, like version, build and configfile used
	log.Println(BaseName, "v"+Version+"-"+BuildVersion)
	log.Println("Using configFile", configFile)

	//Check if the tempfile exists and we are not ignoring
	if _, err := os.Stat(config.TempFile); !os.IsNotExist(err) {
		if !ignoreFlag && !keepTemp {
			removeTempFile() //For next time remove the temp file
			fatalln("Looks like the last run was not successfull (" + config.TempFile + " exists). Cleared now")
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

	//Set the browser path
	template.BrowserPath = config.BrowserPath

	//Check if we should only do testing of converion
	if localFile != "" {
		log.Println("Using localFile", localFile)
		//Run the conversion
		if err := template.XmlToPdfFunc(localFile, config.OutputDir+fileNameWithoutExt(localFile)+".pdf",
			config.TplName, config.TempFile, customFuncs); err != nil {
			fatalln("Failed to convert file ("+localFile+")", err)
		}
		removeTempFile()
		return
	}

	//In pipeMode we receive xml from stdin and write it ot stdout
	if pipeMode {
		log.Println("Using pipeMode")
		//Read the pipedata
		stdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			fatalln(err)
		}
		var pipeFile = "." + BaseName + ".pip"
		if err = os.WriteFile(pipeFile, stdin, 0644); err != nil {
			fatalln(err)
		}

		//Run the conversion
		if err := template.XmlToPdfFunc(pipeFile, pipeFile,
			config.TplName, config.TempFile, customFuncs); err != nil {
			fatalln("Failed to convert file ("+localFile+")", err)
		}
		removeTempFile()
		//Dump the PDF to the console
		pdf, err := os.ReadFile(pipeFile)
		if err != nil {
			fatalln(err)
		}
		os.Stdout.Write(pdf)
		//Remove the PDF
		removeFile(pipeFile)
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
			content, err := io.ReadAll(cf)
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

	// For each remote filename
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
			content, err := io.ReadAll(df)
			if err != nil {
				fatalln(err)
			}
			err = os.WriteFile(config.TempFile, content, 0644)
			if err != nil {
				fatalln(err)
			}

			//Run the conversion on the tempfile
			if err := template.XmlToPdfFunc(config.TempFile, config.OutputDir+fileNameWithoutExt(remoteFile)+".pdf",
				config.TplName, config.TempFile, customFuncs); err != nil {
				fatalln("Failed to convert file ("+remoteFile+")", err)
			}
			log.Println("Created PDF", config.OutputDir+fileNameWithoutExt(remoteFile)+".pdf")
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
