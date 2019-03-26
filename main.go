package main

import (
	"bufio"
	"encoding/csv"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	apiLib "github.com/hornbill/goAPILib"
	hornbillHelpers "github.com/hornbill/goHornbillHelpers"
	"github.com/hornbill/pb"
)

var (
	configAPIKey     string
	configCSV        string
	configDefaultBPM string
	configInstanceID string
	configVersion    bool
	localLogFileName string
	counters         counterStruct
)

const (
	version = "1.0.0"
)

type counterStruct struct {
	SpawnedSuccess   int
	SpawnedFail      int
	CatalogsChecked  int
	CatalogsReturned int
	CatalogsError    int
}

type xmlmcResponse struct {
	MethodResult         string `xml:"status,attr"`
	StateCode            string `xml:"state>code"`
	StateError           string `xml:"state>error"`
	BPMID                string `xml:"params>bpmProcessId"`
	ExceptionName        string `xml:"params>exceptionName"`
	ExceptionDescription string `xml:"params>exceptionDescription"`
	CatalogBPMID         string `xml:"params>primaryEntityData>record>h_bpm"`
}

func main() {
	localLogFileName = "bpmspawner_" + time.Now().Format("20060102150405") + ".log"
	flag.StringVar(&configInstanceID, "instance", "", "Instance ID")
	flag.StringVar(&configAPIKey, "apikey", "", "API Key")
	flag.StringVar(&configCSV, "csv", "", "CSV File to process.")
	flag.StringVar(&configDefaultBPM, "defaultbpm", "", "Default BPM to use for all requests in CSV.")
	flag.BoolVar(&configVersion, "version", false, "Returns tohe tool version and ends")
	flag.Parse()

	//-- If configVersion just output version number and die
	if configVersion {
		fmt.Printf("%v \n", version)
		return
	}

	hornbillHelpers.Logger(3, "Hornbill Service Manager BPM Spawner v"+version, true, localLogFileName)
	if configAPIKey == "" {
		hornbillHelpers.Logger(4, "apikey parameter not provided", true, localLogFileName)
	}

	if configInstanceID == "" {
		hornbillHelpers.Logger(4, "instance parameter not provided", true, localLogFileName)
	}

	if configCSV == "" {
		hornbillHelpers.Logger(4, "csv parameter not provided", true, localLogFileName)
	}

	hornbillHelpers.Logger(3, "Instance: "+configInstanceID, true, localLogFileName)
	hornbillHelpers.Logger(3, "CSV File: "+configCSV, true, localLogFileName)
	csvRows, err := lineCount(configCSV)
	if err != nil {
		hornbillHelpers.Logger(4, "Error retrieving line count of CSV ["+configCSV+"] "+fmt.Sprintf("%v", err), true, localLogFileName)
		return
	}
	if csvRows == 0 {
		hornbillHelpers.Logger(4, "Zero rows found in CSV ["+configCSV+"]", true, localLogFileName)
		return
	}

	hornbillHelpers.Logger(3, fmt.Sprintf("%v", csvRows)+" rows found in ["+configCSV+"]", true, localLogFileName)

	//Start XMLMC session
	espXmlmc := apiLib.NewXmlmcInstance(configInstanceID)
	espXmlmc.SetAPIKey(configAPIKey)

	//Loop through CSV, processing BPMs
	f, _ := os.Open(configCSV)
	r := csv.NewReader(bufio.NewReader(f))
	bar := pb.StartNew(int(csvRows))
	for {
		record, err := r.Read()
		// Stop at EOF.
		if err == io.EOF || len(record) != 2 {
			break
		}
		smCallID := strings.Trim(record[0], " ")
		catalogID := strings.Trim(record[1], " ")
		processBPM(smCallID, catalogID, espXmlmc)
		bar.Increment()
	}
	bar.Finish()

	hornbillHelpers.Logger(3, "", true, localLogFileName)
	hornbillHelpers.Logger(1, "Total Requests: "+fmt.Sprintf("%v", csvRows), true, localLogFileName)
	hornbillHelpers.Logger(1, "Request BPM Spawned Successfully: "+strconv.Itoa(counters.SpawnedSuccess), true, localLogFileName)
	defLogEntryType := 1
	if counters.SpawnedFail > 0 {
		defLogEntryType = 4
	}
	hornbillHelpers.Logger(defLogEntryType, "Request BPM Spawned Errors: "+strconv.Itoa(counters.SpawnedFail), true, localLogFileName)
	hornbillHelpers.Logger(1, "Catalog IDs Provided: "+strconv.Itoa(counters.CatalogsChecked), true, localLogFileName)
	hornbillHelpers.Logger(1, "Catalog Records Returned Successfully: "+strconv.Itoa(counters.CatalogsReturned), true, localLogFileName)

	defLogEntryType = 1
	if counters.CatalogsError > 0 {
		defLogEntryType = 4
	}
	hornbillHelpers.Logger(defLogEntryType, "Catalog Records Errors: "+strconv.Itoa(counters.CatalogsError), true, localLogFileName)
}

func lineCount(filename string) (int64, error) {
	lc := int64(0)
	f, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		lc++
	}
	return lc, s.Err()
}

func processBPM(callref, catalogID string, espXmlmc *apiLib.XmlmcInstStruct) {
	hornbillHelpers.Logger(3, "", false, localLogFileName)
	hornbillHelpers.Logger(2, "Processing Request ["+callref+"]", false, localLogFileName)
	if configDefaultBPM != "" {
		hornbillHelpers.Logger(1, "Using default BPM ["+configDefaultBPM+"]", false, localLogFileName)
		spawnBPM(callref, "", espXmlmc)
		return
	}
	if catalogID == "" {
		hornbillHelpers.Logger(1, "No Catalog ID provided.", false, localLogFileName)
		spawnBPM(callref, "", espXmlmc)
		return
	}
	//go get BPM from catalog
	counters.CatalogsChecked++
	hornbillHelpers.Logger(1, "Retrieving BPM for Catalog ID ["+catalogID+"]", false, localLogFileName)
	catalogBPMID := getCatalogBPM(callref, catalogID, espXmlmc)
	if catalogBPMID != "" {
		counters.CatalogsReturned++
		spawnBPM(callref, catalogBPMID, espXmlmc)
	} else {
		counters.CatalogsError++
		hornbillHelpers.Logger(5, "Unable to retrive Catalog record ["+catalogID+"]", false, localLogFileName)

	}
}

func spawnBPM(callref, bpmID string, espXmlmc *apiLib.XmlmcInstStruct) {
	espXmlmc.SetParam("requestId", callref)
	if bpmID != "" {
		espXmlmc.SetParam("defaultBpm", bpmID)
	}

	XMLBPM, xmlmcErr := espXmlmc.Invoke("apps/com.hornbill.servicemanager/Requests", "logRequestBPM")
	if xmlmcErr != nil {
		hornbillHelpers.Logger(4, "API Call Failed ["+callref+"] [logRequestBPM]: "+fmt.Sprintf("%v", xmlmcErr), false, localLogFileName)
		counters.SpawnedFail++
		return
	}
	var xmlResponse xmlmcResponse

	err := xml.Unmarshal([]byte(XMLBPM), &xmlResponse)
	if err != nil {
		hornbillHelpers.Logger(4, "Response Unmarshal Failed ["+callref+"] [logRequestBPM]: "+fmt.Sprintf("%v", err), false, localLogFileName)
		counters.SpawnedFail++
		return
	}
	if xmlResponse.MethodResult != "ok" {
		hornbillHelpers.Logger(4, "MethodResult not OK ["+callref+"] [logRequestBPM]: "+xmlResponse.StateError, false, localLogFileName)
		counters.SpawnedFail++
		return
	}
	if xmlResponse.ExceptionName != "" {
		hornbillHelpers.Logger(4, "Exception Returned ["+callref+"] [logRequestBPM]: ["+xmlResponse.ExceptionName+"] "+xmlResponse.ExceptionDescription, false, localLogFileName)
		counters.SpawnedFail++
		return
	}
	counters.SpawnedSuccess++
	hornbillHelpers.Logger(3, "[SUCCESS] BPM Spawned ["+callref+"] ["+xmlResponse.BPMID+"]", false, localLogFileName)
	return
}

func getCatalogBPM(callref, catalogID string, espXmlmc *apiLib.XmlmcInstStruct) string {
	espXmlmc.SetParam("application", "com.hornbill.servicemanager")
	espXmlmc.SetParam("entity", "Catalogs")
	espXmlmc.SetParam("keyValue", catalogID)
	XMLBPM, xmlmcErr := espXmlmc.Invoke("data", "entityGetRecord")
	if xmlmcErr != nil {
		hornbillHelpers.Logger(4, "API Call Failed ["+callref+"] ["+catalogID+"] [entityGetRecord]: "+fmt.Sprintf("%v", xmlmcErr), false, localLogFileName)
		return ""
	}
	var xmlResponse xmlmcResponse
	err := xml.Unmarshal([]byte(XMLBPM), &xmlResponse)
	if err != nil {
		hornbillHelpers.Logger(4, "Response Unmarshal Failed ["+callref+"] ["+catalogID+"] [entityGetRecord]: "+fmt.Sprintf("%v", err), false, localLogFileName)
		return ""
	}
	if xmlResponse.MethodResult != "ok" {
		hornbillHelpers.Logger(4, "MethodResult not OK ["+callref+"] ["+catalogID+"] [entityGetRecord]: "+xmlResponse.StateError, false, localLogFileName)
		return ""
	}
	hornbillHelpers.Logger(1, "Catalog BPM Returned ["+callref+"] ["+catalogID+"]: "+xmlResponse.CatalogBPMID, false, localLogFileName)
	return xmlResponse.CatalogBPMID
}
