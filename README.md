# FtpXMLtoPDF
Download XML from FTP server and convert it to PDF using a template engine


Testing the conversion of an xml file using a template can be done locally by running for example the command below.

``
go run main.go -testFile="testdata/2023081700045.xml" -tplName="templates/dso.tpl.html" -outputDir="testdata/pdf" -save 
``

Testing a file with the default or saved config and ignoring errors from last run

``
go run main.go -testFile="testdata/2023081700045.xml" -ignore 
``

## Steps explained
* Check if there is an unexpected last terminated run (**.ftpxmltopdf.tmp**). Using the **ignore flag** you can skip this check
* The tool will login to the specified sFTP **ftpServer**, using the specified **ftpUser** and **ftpPassword**
* Use the working location **ftpDir**
* Validate if there is a lock file. Using the **ignore flag** can skip this check
* Create a lock file **.ftpxmltopdf.lck**
* Read the remoteconfig **.ftpxmltopdf.cfg**
* Select all xml (**ftpFilter**) files based on the *config.lastProcessed*
* Download the selected xml files into *./tmp* directory, overwrite when exists
* For each xml file, convert it to PDF using the template engine and save the result as pdf into **outputDir**
* Update the *config.lastProcessed* to max of last changedate of the processed xml files and save the config file back on the ftpserver.
* Remove the **.ftpxmltopdf.lck** from the ftpserver and logout
* Remove the **.ftpxmltopdf.tmp** file when **keepTemp flag** is not specified

## Error handling
The system will only exit after user confirmations in case of an error. All internal steps are logged in the *./tmp/ftpxmltopdf.log* 

## Commandline options
* *-save*  This options will save the current commandline values as defaults in encrypted file. (**.ftpxmltopdf.cfg**)
* *-ftpServer=* The ftpServer 
* *-ftpUser=* The ftpServer User 
* *-ftpPassword=* The ftpServer Password
* *-ftpDir=* The ftpServer directory (defaults to .)
* *-ftpTLS* Use a normal FTP server on port 21 with TLS support
* *-tplName=* The template file to use for conversion (defaults to ftpxmltopdf)
* *-outputDir=* The directory where endresults are stored
* *-testFile=* The xml test file to convert (ftp part wil be skipped)
* *-tempFile The name of the tempfile to be used
* *-keepTemp Will keep the temporary file
* *-remoteReset Will reset the remote configfile


# Material used for inspration
* [Sftp over SSH](https://www.inanzzz.com/index.php/post/tjp9/golang-sftp-client-server-example-to-upload-and-download-files-over-ssh-connection-streaming)
