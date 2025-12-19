package main

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// ریلوے یا لوکل انوائرمنٹ سے پورٹ اٹھانا
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		return "8080"
	}
	return port
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Query Parameter سے 'type' نکالنا
	dataType := r.URL.Query().Get("type")
	if dataType == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error": "Missing ?type parameter"}`)
		return
	}

	var targetURL string
	var referer string

	// یو آر ایل اور ریفرر سیٹ کرنا جیسا آپ کے JS کوڈ میں تھا
	if dataType == "numbers" {
		targetURL = "http://217.182.195.194/ints/agent/res/data_smsnumbers.php?frange=&fclient=&sEcho=2&iColumns=8&sColumns=%2C%2C%2C%2C%2C%2C%2C&iDisplayStart=0&iDisplayLength=-1&mDataProp_0=0&sSearch_0=&bRegex_0=false&bSearchable_0=true&bSortable_0=false&mDataProp_1=1&sSearch_1=&bRegex_1=false&bSearchable_1=true&bSortable_1=true&mDataProp_2=2&sSearch_2=&bRegex_2=false&bSearchable_2=true&bSortable_2=true&mDataProp_3=3&sSearch_3=&bRegex_3=false&bSearchable_3=true&bSortable_3=true&mDataProp_4=4&sSearch_4=&bRegex_4=false&bSearchable_4=true&bSortable_4=true&mDataProp_5=5&sSearch_5=&bRegex_5=false&bSearchable_5=true&bSortable_5=true&mDataProp_6=6&sSearch_6=&bRegex_6=false&bSearchable_6=true&bSortable_6=true&mDataProp_7=7&sSearch_7=&bRegex_7=false&bSearchable_7=true&bSortable_7=false&sSearch=&bRegex=false&iSortCol_0=0&sSortDir_0=asc&iSortingCols=1&_=1766171476582"
		referer = "http://217.182.195.194/ints/agent/MySMSNumbers"
	} else if dataType == "sms" {
		targetURL = "http://217.182.195.194/ints/agent/res/data_smscdr.php?fdate1=2025-12-19%2000:00:00&fdate2=2085-12-19%2023:59:59&frange=&fclient=&fnum=&fcli=&fgdate=&fgmonth=&fgrange=&fgclient=&fgnumber=&fgcli=&fg=0&csstr=9df7a3e50debcd51cca20329b34c1bfe&sEcho=2&iColumns=9&sColumns=%2C%2C%2C%2C%2C%2C%2C%2C&iDisplayStart=0&iDisplayLength=-1&mDataProp_0=0&sSearch_0=&bRegex_0=false&bSearchable_0=true&bSortable_0=true&mDataProp_1=1&sSearch_1=&bRegex_1=false&bSearchable_1=true&bSortable_1=true&mDataProp_2=2&sSearch_2=&bRegex_2=false&bSearchable_2=true&bSortable_2=true&mDataProp_3=3&sSearch_3=&bRegex_3=false&bSearchable_3=true&bSortable_3=true&mDataProp_4=4&sSearch_4=&bRegex_4=false&bSearchable_4=true&bSortable_4=true&mDataProp_5=5&sSearch_5=&bRegex_5=false&bSearchable_5=true&bSortable_5=true&mDataProp_6=6&sSearch_6=&bRegex_6=false&bSearchable_6=true&bSortable_6=true&mDataProp_7=7&sSearch_7=&bRegex_7=false&bSearchable_7=true&bSortable_7=true&mDataProp_8=8&sSearch_8=&bRegex_8=false&bSearchable_8=true&bSortable_8=false&sSearch=&bRegex=false&iSortCol_0=0&sSortDir_0=desc&iSortingCols=1&_=1766171360378"
		referer = "http://217.182.195.194/ints/agent/SMSCDRStats"
	} else {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error": "Invalid type (use sms or numbers)"}`)
		return
	}

	// کلائنٹ ریکویسٹ بنانا
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// وہی ہیڈرز جو آپ نے فراہم کیے تھے
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13; V2040 Build/TP1A.220624.014) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.7499.34 Mobile Safari/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cookie", "PHPSESSID=pb3620rtcrklvvrmndf8kmt93n")
	req.Header.Set("Referer", referer)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "Fetch failed", "details": "%s"}`, err.Error())
		return
	}
	defer resp.Body.Close()

	// ڈی کمپریشن ہینڈلنگ (Gzip/Deflate)
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer reader.Close()
	case "deflate":
		reader = flate.NewReader(resp.Body)
		defer reader.Close()
	default:
		reader = resp.Body
	}

	// ڈیٹا پڑھنا اور واپس بھیجنا
	body, err := io.ReadAll(reader)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(body)
}

func main() {
	port := getPort()
	http.HandleFunc("/api/fetch", handleRequest)

	fmt.Printf("🚀 API Fetcher running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}