package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/textproto"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	flag "github.com/ogier/pflag"
)

var (
	urlInput     string
	profileInput *int
)

func main() {
	flag.Parse()
	if urlInput != "" {
		getReq(urlInput)
	}
	if *profileInput > 0 {
		getProfile(*profileInput)
	}
}

func init() {
	flag.StringVar(&urlInput, "url", "", "Make a get request using a provided URL. Usage:  --url=<url>")
	profileInput = flag.Int("profile", 0, "Creates a profile of https://my-worker.ejchen.workers.dev/links (General Engineering Assignment). Usage: --profile=<number_of_requests> ")
}

func getReq(address string) {
	u, err := url.Parse(address)
	if err != nil {
		panic(err)
	}
	host := u.Host
	path := ""
	if u.Path == "" {
		path = "/"
	} else {
		path = u.Path
	}
	if strings.Contains(address, "https://") {
		httpsRequest(host, path, true)
	} else if strings.Contains(address, "http://") {
		httpRequest(host, path, true)
	} else {
		println("Please include http or https in the URL.")
	}
}

func httpsRequest(host, path string, print bool) (time.Duration, int, string) {
	rootPEM, err := ioutil.ReadFile("rootPEM.txt")

	cp, _ := x509.SystemCertPool()               //create certificate pool
	ok := cp.AppendCertsFromPEM([]byte(rootPEM)) //append PEM to certificate pool
	if !ok {
		panic("failed to parse root certificate")
	}
	start := time.Now() //begin timing the call
	//-- establish connection --//
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", host), &tls.Config{
		RootCAs: cp,
	})
	if err != nil {
		fmt.Println("Error connecting to:", host, "\n", err.Error())
		os.Exit(1)
	}

	conn.Write([]byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", path, host)))

	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)

	defer conn.Close()
	totalBytes := 0
	statusLine, err := tp.ReadLine()
	if err != nil {
		fmt.Println("Error reading response:", err.Error())
		os.Exit(1)
	}
	totalBytes += reader.Buffered()
	if print {
		println(statusLine)
		for {
			line, err := tp.ReadLine()
			totalBytes += reader.Buffered()
			if err != nil {
				break
			}
			println(line)
		}
	} else {
		for {
			_, err := tp.ReadLine()
			if err != nil {
				break
			}
			totalBytes += reader.Buffered()
		}
	}
	elapsed := time.Since(start)
	status := statusLine[9:12]
	return elapsed, totalBytes, status
}

func httpRequest(host, path string, print bool) (time.Duration, int, string) {
	//-- establish connection --//
	tcpAddr, err := net.ResolveTCPAddr("tcp4", host+":80")
	start := time.Now() // begin timing the call
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println("Error connecting to:", host, "\n", err.Error())
		os.Exit(1)
	}

	conn.Write([]byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", path, host)))

	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)

	defer conn.Close()
	totalBytes := 0
	statusLine, err := tp.ReadLine()
	if err != nil {
		fmt.Println("Error reading response:", err.Error())
		os.Exit(1)
	}
	totalBytes += reader.Buffered()
	if print {
		println(statusLine)
		for {
			line, err := tp.ReadLine()
			if err != nil {
				break
			}
			totalBytes += reader.Buffered()
			println(line)
		}
	} else {
		for {
			_, err := tp.ReadLine()
			if err != nil {
				break
			}
			totalBytes += reader.Buffered()
		}
	}
	elapsed := time.Since(start)
	status := statusLine[9:12]

	return elapsed, totalBytes, status
}

func getProfile(numReq int) {
	if numReq == 0 {
		return
	}
	host := "my-worker.ejchen.workers.dev"
	path := "/links"

	println("----------------------------------------------------")
	println("Website: " + "https://" + host + path)
	errorCodes := make(map[string]struct{})

	times := make([]int64, numReq)
	totalTime := int64(0)
	medianTime := int64(0)
	fastestTime := int64(math.MaxInt64)
	slowestTime := int64(0)
	smallestRes := math.MaxInt64
	largestRes := 0
	failCount := 0

	for i := 0; i < numReq; i++ {
		timeTaken, totalBytes, status := httpsRequest(host, path, false)
		totalTime += timeTaken.Milliseconds()
		times[i] = timeTaken.Milliseconds()
		if fastestTime > timeTaken.Milliseconds() {
			fastestTime = timeTaken.Milliseconds()
		}
		if slowestTime < timeTaken.Milliseconds() {
			slowestTime = timeTaken.Milliseconds()
		}
		if smallestRes > totalBytes {
			smallestRes = totalBytes
		}
		if largestRes < totalBytes {
			largestRes = totalBytes
		}
		if status[0] != '2' { // not a 200 code
			if _, ok := errorCodes[status]; !ok {
				errorCodes[status] = struct{}{}
			}
			failCount++
		}
	}

	//-- Median Calculation --//

	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	if numReq%2 == 0 {
		medianTime = (times[numReq/2] + times[(numReq/2)-1]) / int64(2)
	} else {
		medianTime = times[numReq/2]
	}

	//-- Summary --//

	println("Number of requests:", numReq)
	println("Fastest response time:", fastestTime, "ms")
	println("Slowest response time:", slowestTime, "ms")
	println("Mean response time:", totalTime/int64(numReq), "ms")
	println("Median response time:", medianTime, "ms")
	numSuccessful := (float32(numReq-failCount) / float32(numReq)) * 100.0
	fmt.Printf("Percentage of requests that succeeded: %4.2f", numSuccessful)
	println("%")

	print("Error codes: ")
	for k := range errorCodes {
		fmt.Printf("%s ", k)
	}
	println()
	println("Smallest response (bytes):", smallestRes)
	println("Largest response (bytes):", largestRes)
	println("----------------------------------------------------")
}
