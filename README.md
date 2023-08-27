# FtpXMLtoPDF
Download a XML file from a FTP(TLS) or SFTP(over SSH) server and convert it to PDF using a HTML template engine.

## Using the tool
You can download the native exceutable ``ftpxmltopdf`` using the [latest release](https://github.com/partner4it/FtpXMLtoPDF/releases/latest) archive and unziping it on you local system. This archive includes a demo template to inspire you.
You can allways just clone the project and run it using ``go run . <parameters>``. 

### First time usage
The tool needs paramters, like servername,username,... to perform it task. To make te tool easy in use whe hav implemented an option to save these parameters in an encypted config file. This encrypted configfile can also be distributed to user so they will not see the security sensitive data. To update the information in the configfile run.
``ftpxmltopdf -ftpServer=??? -ftpUser=??? -ftpPassword=??? ..... -save``

Next time you want to run the same action using the saved config, just run ``ftpxmltopdf ``

## Commandline options
* **-save**  This options will save the current commandline values into an encrypted configfile. (defaults to **.ftpxmltopdf.cfg**)
* **-configFile=** Specify witch configfile to use
* **-ftpServer=** The ftp Server 
* **-ftpUser=** The ftp User 
* **-ftpPassword=** The ftp Password
* **-ftpDir=** The ftp directory (defaults to *.*)
* **-ftpTLS** Use a normal FTP server on port 21 with TLS support, instead of SFTP over SSH
* **-ftpRemove** When set we will remove the remote file on successfull processing
* **-tplName=** The template file to use for conversion (defaults to *ftpxmltopdf.tpl.html*)
* **-outputDir=** The directory where endresults are stored
* **-localFile=** Name of the local test xmlfile to test template converversion (ftp server is)
* **-tempFile** The name of the temporary html file to be used (defaults to *.ftpxmltopdf.tmp*)
* **-keepTemp** Will keep the temporary html file
* **-remoteReset** Will reset the remote configfile
* **-logFile=** When set this file is used for logging instead of console 
* **-silent** When set and logfile is empty we will not show any logging
* **-browser=** Set the path to the chrome browser


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

## Creating and testing a template
Testing the conversion of an xml file is easy. By using a template and xmlfile locally. The template provide is a demonstation only and was intended for a water authority to use as a base translation of a DSO request. If you want to no more about the go html template options read the [documentation](https://pkg.go.dev/html/template). 

Example testing a local template and xml file

``
ftpxmltopdf -localFile="testdata/2023081700045.xml" -tplName="templates/dso.tpl.html" -outputDir="testdata/pdf" -save 
``

or if have cloned the project

``
go run . -localFile="testdata/2023081700045.xml" -tplName="templates/dso.tpl.html" -outputDir="testdata/pdf" -save 
``

Testing a file with the default or saved config and ignoring errors from last run.

``
ftpxmltopdf -localFile="testdata/2023081700045.xml" -ignore 
``

## Error handling
All internal steps are logged to the console. On an error the user has to confirm that the error is read by hitting ***Enter***. When using the **-ignore** flag, no user confirmation is asked on an error.
