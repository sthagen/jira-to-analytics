// Method:
// 1. Parse command-line flags and config file
// 2. Fetch Jira Issue Keys in batches
// 3. Fetch Jira Issues by key in batches, using them to build work items
// 4. Write out CSV or JSON, one work item per line

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const version = "1.0-beta.9"
const issueBatchSize = 100
const maxTries = 5
const retryDelay = 5 // seconds per retry

func main() {
	start := time.Now()

	// parse command-line args
	yamlName := flag.String("i", "config.yaml", "set the input config file name")
	outName := flag.String("o", "data.csv", "set the output file name (.csv or .json)")
	printVersion := flag.Bool("v", false, "print the version number and exit")
	showQuery := flag.Bool("q", false, "display the query used")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Println("Unexpected argument \"" + flag.Args()[0] + "\"")
		fmt.Println("For help use", os.Args[0], "-h")
		os.Exit(1)
	}
	if *printVersion {
		fmt.Println(version)
		os.Exit(0)
	}
	if !strings.HasSuffix(*outName, ".csv") && !strings.HasSuffix(*outName, ".json") {
		fmt.Println("Error: output file name must end with .csv or .json")
		os.Exit(1)
	}

	// load config and set password
	fmt.Println("Reading config file", *yamlName)
	config, err := LoadConfigFromFile(*yamlName)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(0)
	}
	if len(config.Password) == 0 {
		config.Password = getPassword()
	}

	// collect the work items for the keys
	fmt.Println("Fetching issues...")
	currentStartIndex := 0
	var items []*Item

	success := false
	for tries := 0; tries < maxTries; tries++ {
		fmt.Println("Loading Issues ", currentStartIndex, "-", currentStartIndex + issueBatchSize)
		itemBatch, err := getItems(issueBatchSize, currentStartIndex, *showQuery, config);

		if err == nil {
			items = append(items, itemBatch...)
			fmt.Println("\t\t==> Request ok")
			fmt.Println("\t\t==> Fetched ", len(itemBatch), " items")
			success = true
			currentStartIndex += issueBatchSize
			tries = 0

			if( itemBatch == nil || len(itemBatch) == 0) {
				break
			}
		} else {
			fmt.Println("\t\t ==> Request caused an error")
			fmt.Println("\t\t ==>", err)
			break
		}
		if (tries > 0 && tries < maxTries-1) {
			fmt.Println("\t\tRetrying issues ", currentStartIndex, "-", currentStartIndex + issueBatchSize, ": ")
			time.Sleep(time.Duration(tries*(retryDelay+1)) * time.Second) // delay increases
		}
	}
	if !success {
		fmt.Println("Error: Issues", currentStartIndex, "-", currentStartIndex + issueBatchSize, "failed to load")
		os.Exit(1)
	}

	// output work items
	fmt.Println("Writing", *outName)
	if strings.HasSuffix(*outName, ".json") {
		writeJSON(items, config, *outName)
	} else {
		writeCSV(items, config, *outName)
	}

	// show elapsed time
	elapsed := time.Since(start)
	fmt.Printf("Total Time: %s\n", elapsed)
}

func writeCSV(items []*Item, config *Config, fileName string) {

	// open the file
	file, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// write the header line
	file.WriteString("ID,Link,Name")
	for _, stage := range config.StageNames {
		file.WriteString("," + stage)
	}
	for _, attr := range config.Attributes {
		file.WriteString("," + attr.ColumnName)
	}
	file.WriteString("\n")

	// write a line for each work item
	writeLink := true
	counter := 0
	for _, item := range items {
		if item.HasDate() {
			file.WriteString(item.toCSV(config, writeLink))
			file.WriteString("\n")
			writeLink = false
			counter++
		}
	}

	// display some stats
	skipped := len(items) - counter
	if skipped > 0 {
		fmt.Println(skipped, "empty work items omitted")
	}
	fmt.Println(counter, "work items written")
}

func writeJSON(items []*Item, config *Config, fileName string) {

	// open the file
	file, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// write the header line
	file.WriteString("[[\"ID\",\"Link\",\"Name\"")
	for _, stage := range config.StageNames {
		file.WriteString("," + quoteString(stage))
	}
	for _, attr := range config.Attributes {
		file.WriteString("," + quoteString(attr.ColumnName))
	}
	file.WriteString("]")

	// write a line for each work item
	writeLink := true
	counter := 0
	for _, item := range items {
		if item.HasDate() {
			file.WriteString(",\n")
			file.WriteString(item.toJSON(config, writeLink))
			writeLink = false
			counter++
		}
	}
	file.WriteString("]\n")

	// display some stats
	skipped := len(items) - counter
	if skipped > 0 {
		fmt.Println(skipped, "empty work items omitted")
	}
	fmt.Println(counter, "work items written")
}
