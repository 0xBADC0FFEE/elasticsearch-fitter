package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
)

var Version = "3.1.0"

var dailyIndex = regexp.MustCompile(`^(.+?)((?:\d{2,4}\.*?){3})$`)

func main() {

	var (
		versionFlag  bool
		serverFlag   string
		spaceFlag    int
		durationFlag string
		skipFlag     SkipFlag
	)

	flag.BoolVar(&versionFlag, "version", false, "Версия приложения")
	flag.StringVar(&serverFlag, "server", "http://localhost:9200", "Адрес ElasticSearch")
	flag.IntVar(&spaceFlag, "space", 15, "Допустимый объем свободного места на диске в процентах")
	flag.StringVar(&durationFlag, "duration", "1h", "Частота проверки")
	flag.Var(&skipFlag, "skip", "Не удалять указанный индекс")
	flag.Parse()

	if versionFlag {
		println(Version)
		return
	}
	_, err := url.Parse(serverFlag)
	if err != nil {
		log.Fatalf("Invalid url %s: %s", serverFlag, err)
	}

	duration, err := time.ParseDuration(durationFlag)
	if err != nil {
		log.Fatalf("Error parsing -duration flag: %s", err)
	}
	ticker := time.NewTicker(duration)

	for {
		log.Printf("Checking")

		for {
			ok, err := checkFreeSpace(serverFlag, spaceFlag)
			if err != nil {
				log.Printf("Error checking, %v", err)
				time.Sleep(time.Minute)
				continue
			}
			if !ok {

				if err := deleteOldIndeces(serverFlag, skipFlag); err != nil {
					log.Printf("Error checking, %v", err)
					time.Sleep(time.Minute)
					continue
				}

				continue
			}

			log.Print("Free data space enough")
			break
		}

		log.Printf("Done checking")
		<-ticker.C
	}

}

func checkFreeSpace(server string, space int) (bool, error) {
	log.Print("Checking data storage free space")

	// Space
	nodesStatsBody, err := request(http.MethodGet, fmt.Sprintf("%s/_nodes/stats", server), nil)
	if err != nil {
		return false, fmt.Errorf("error getting nodes stats, %v", err)
	}

	var stats NodeStatsResponse

	jsonErr := json.Unmarshal(nodesStatsBody, &stats)
	if jsonErr != nil {
		return false, fmt.Errorf("error unmarshalling NodeStatsResponse to json, %v", err)
	}

	log.Print("Calculating free space for nodes")

	var freePercent int64

	for nodeName := range stats.Nodes {
		log.Printf("Node: %v [%s]", stats.Nodes[nodeName].Name, stats.Nodes[nodeName].Host)

		fsTotal := stats.Nodes[nodeName].FS.Total

		for _, fsData := range stats.Nodes[nodeName].FS.Data {
			log.Printf("Node: %s, Path: %s, Device: %s, Free: %.2fGB", nodeName, fsData.Path, fsData.Device, datasize.ByteSize(fsData.Available).GBytes())
		}

		freePercent = fsTotal.Available * 100 / fsTotal.Total
		log.Printf("Node %s Free data space: %d%% (%.2fGB)", nodeName, freePercent, datasize.ByteSize(fsTotal.Available).GBytes())
	}

	return freePercent > int64(space), nil
}

func deleteOldIndeces(server string, skip SkipFlag) error {
	// Indices
	log.Print("Deleting old indices")

	aliasesBody, err := request(http.MethodGet, fmt.Sprintf("%s/_aliases", server), nil)
	if err != nil {
		return fmt.Errorf("error getting nodes stats, %v", err)
	}

	var aliases AliasesResponse

	jsonErr := json.Unmarshal(aliasesBody, &aliases)
	if jsonErr != nil {
		return fmt.Errorf("error unmarshalling AliasesResponse to json, %v", err)
	}

	var indices Indices

	for index := range aliases {
		if !skip.Has(index) {
			indices = append(indices, index)
		}
	}

	if len(indices) == 0 {
		return fmt.Errorf("no indices to remove")
	}

	sort.Sort(indices)
	index := indices[0]

	// Delete
	log.Printf("Delete index %s request", index)

	indexDeleteBody, err := request(http.MethodDelete, fmt.Sprintf("%s/%s", server, index), nil)
	if err != nil {
		return fmt.Errorf("error deleting index %s, %s, %v", index, string(indexDeleteBody), err)
	}

	log.Printf("Index %s deleted successfully", index)

	return nil
}

func request(method, url string, body io.Reader) ([]byte, error) {

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request %s %s, %v", method, url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s error, %v", url, err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error ReadAll response body, %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response status %d, %s", resp.StatusCode, respBody)
	}

	return respBody, nil
}

type SkipFlag []string

func (sf SkipFlag) String() string {
	return strings.Join(sf, ", ")
}

func (sf *SkipFlag) Set(v string) error {
	*sf = append(*sf, v)
	return nil
}

func (sf *SkipFlag) Has(v string) bool {

	for _, skip := range *sf {
		if v == skip {
			return true
		}
	}

	return false
}

type Indices []string

func (is Indices) Len() int {
	return len(is)
}

func (is Indices) Less(i, j int) bool {

	iTime, err := indexTime(is[i])
	if err != nil {
		return false
	}

	jTime, err := indexTime(is[j])
	if err != nil {
		return true
	}

	return iTime.Before(jTime)
}

func (is Indices) Swap(i, j int) {
	is[i], is[j] = is[j], is[i]
}

func indexTime(s string) (time.Time, error) {
	matches := dailyIndex.FindStringSubmatch(s)

	if len(matches) < 3 {
		return time.Time{}, errors.New("wrong format")
	}

	date, err := time.Parse("2006.01.02", matches[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing date %s to %s: %v", date, s, err)
	}

	return date, nil
}

type AliasesResponse map[string]interface{}

type NodeStatsResponse struct {
	Nodes map[string]NodeStatsNodeResponse
}

type NodeStatsNodeResponse struct {
	Name string              `json:"name"`
	Host string              `json:"host"`
	FS   NodeStatsFSResponse `json:"fs"`
}

type NodeStatsFSResponse struct {
	Timestamp int64                     `json:"timestamp"`
	Total     NodeStatsFSTotal          `json:"total"`
	Data      []NodeStatsFSDataResponse `json:"data"`
}

type NodeStatsFSTotal struct {
	Total     int64 `json:"total_in_bytes"`
	Free      int64 `json:"free_in_bytes"`
	Available int64 `json:"available_in_bytes"`
}

type NodeStatsFSDataResponse struct {
	Path      string `json:"path"`
	Mount     string `json:"mount"`
	Device    string `json:"dev"`
	Total     int64  `json:"total_in_bytes"`
	Free      int64  `json:"free_in_bytes"`
	Available int64  `json:"available_in_bytes"`
}
