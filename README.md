# Hornbill Service Manager Request BPM Spawner Utility

This tool allows you to spawn and associate BPM workflows to requests that were missed during a failed import.

## Configure

This tool takes a CSV file as input, extracts the required request references (and optional catlog IDs) and attempts to spawn and associated new BPM workflows against the request.

The CSV file should have no headers. The first column should contain the request references, and the second column should contain the request catalog IDs (should the request be logged against a catalog item). For example:

```csv
IN00043384, 5
IN00043385, 3
IN00043386,
IN00045524,
IN00045525,
IN00045526, 3
IN00045527, 1
IN00045529, 5
```

The easiest way to create the CSV file would be to build a report in Hornbill, with the Request Reference and Catalog ID columns selected for display, relevant filters for your specific requests, CSV defined as the output type, and for the column headings and record counts disabled on output.

## Input

The utility can take the following input parameters:

* `instance` [MANDATORY] - the ID of your Hornbill instance
* `apikey` [MANDATORY] - an API key for the user who you wish to run the utility as
* `csv` [MANDATORY] - the name of the csv file you wish to pull data from
* `defaultbpm` [OPTIONAL] - the ID of a BPM workflow that you want to be spawned for ALL requests in the CSV file. The Service and/or Catalog BPMs will be ignored

### Execute

* Download the OS and architecture specific ZIP archive from [Github](https://github.com/hornbill/goHornbillRequestBPMSpawner/releases)
* Extract zip into a folder you would like the application to run from e.g. `C:\spawner\`
* Open Command Line Prompt as Administrator
* Change Directory to the folder containing the executable `C:\spawner\`
* Run the command `bpmSpawner.exe -instance=yourinstanceid -apikey=yourapikey -csv=requests.csv`

### Logging

The tool will generate a log file, dropped into the log folder (which will be created if doesn't already exist) in the same folder as the executable.