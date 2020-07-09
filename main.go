package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/sunabozu/binance-price-change-go/utils"

	"github.com/adshao/go-binance"
)

func main() {
	parentPath, err := utils.GetParentPath()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	keys, err := utils.LoadKeys(parentPath + "/env.json")

	deltaThreshold := 130.0 // default

	// read the delta from a file
	delta, err := readDelta()

	if err != nil {
		log.Println(err)
	} else {
		deltaThreshold = delta
	}

	deltaChan := make(chan float64)
	go saveDelta(deltaChan)

	client := binance.NewClient(keys.BinanceKey, keys.BinanceSecret)

	interval := 20

	lastPriceChan := make(chan float64)
	go retreiveLastPrice(client, lastPriceChan, interval)
	go processLastPrice(keys, lastPriceChan, interval, &deltaThreshold)

	log.Println("starting the http server...")

	// root handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		result := fmt.Sprintf(`
	<html>
	<body>
		<form method="post" action="/update">
			Delta: <input type="text" name="delta" value="%d" focused />
			<input type="submit" value="save" />
		</form>
	</body>
  </html>
		`, int(deltaThreshold))
		fmt.Fprintf(w, result)
	})

	// update handler
	http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.FormValue("delta"))
		temp, err := strconv.ParseFloat(r.FormValue("delta"), 64)

		if err != nil {
			log.Println(err)
		} else {
			deltaThreshold = temp
			deltaChan <- deltaThreshold
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.ListenAndServe(":8082", nil)
	// select {}
}

func readDelta() (float64, error) {
	parentPath, err := utils.GetParentPath()

	if err != nil {
		return 0, err
	}

	log.Println("reading delta from a file...")
	content, err := ioutil.ReadFile(parentPath + "/delta.txt")

	if err != nil {
		log.Println("oops, can't read the delta!")
		return 0, err
	}

	text := string(content)
	number, err := strconv.ParseFloat(text, 64)

	if err != nil {
		log.Println("oops, can't parse the delta!")
		return 0, err
	}

	return number, nil
}

func retreiveLastPrice(client *binance.Client, lastPriceChan chan<- float64, interval int) error {
	ticker := time.NewTicker(time.Duration(interval * 1000000000))

	for _ = range ticker.C {
		priceDoc, err := client.NewListPricesService().Symbol("BTCUSDT").Do(context.Background())

		if err != nil || len(priceDoc) < 1 {
			log.Println(err)
			continue
		}

		lastPrice, err := strconv.ParseFloat(priceDoc[0].Price, 64)
		if err != nil {
			continue
		}

		lastPriceChan <- lastPrice
	}

	return nil
}

func processLastPrice(keys *utils.Env, lastPriceChan <-chan float64, interval int, deltaThreshold *float64) {
	prices := []float64{}

	changeTime := 60 * 60 // an hour
	elementsInterval := changeTime / interval
	lastPush := time.Time{} // new Date().getTime()

	for lastPrice := range lastPriceChan {
		log.Printf("Current delta: %f", *deltaThreshold)
		prices = append(prices, lastPrice)

		// remove the 1st element if the slice is too long
		if len(prices) > elementsInterval {
			prices = prices[1:]
			log.Println("removing the first element...")
		}
		// log.Printf("Number of the prices: %d\n", len(prices))
		// log.Printf("%+v\n", prices)

		// look for the highest price
		var topPrice float64
		for i, v := range prices {
			if i == 0 || topPrice < v {
				topPrice = v
			}
		}

		// log.Printf("Top price is %f", topPrice)

		// check if the difference is bigger than the threshold
		delta := topPrice - lastPrice
		if delta < *deltaThreshold {
			continue
		}

		log.Println("seems like need to push! 🤔")

		// check if pushed recently
		timeDifference := time.Now().Sub(lastPush).Seconds() < float64(changeTime)
		log.Println(timeDifference)
		if lastPush != (time.Time{}) && timeDifference {
			log.Println("already pushed recently... 😂")
			continue
		}

		lastPush = time.Now()

		// proceed with pushing a notification
		log.Println("pushing now... 😎")
		textToPush := fmt.Sprintf("🔥 Bitcoin dropped by %.2f to %.0f in the past %d minute(s)! 🔥", *deltaThreshold, lastPrice, changeTime/60)
		go sendPushNotification(keys, textToPush)
	}
}

// saving the delta to a file
func saveDelta(deltaChan chan float64) {
	for delta := range deltaChan {
		parentPath, err := utils.GetParentPath()

		if err != nil {
			return
		}

		log.Println("writing to a file...")
		f, err := os.Create(parentPath + "/delta.txt")

		if err != nil {
			log.Println(err)
			return
		}

		strToWrite := strconv.FormatFloat(delta, 'f', 6, 64)

		_, err = f.WriteString(strToWrite)

		if err != nil {
			log.Println(err)
			return
		}

		f.Sync()
		defer f.Close()
	}
}

func sendPushNotification(keys *utils.Env, text string) {
	formData := url.Values{
		"app_key":     {keys.PushedKey},
		"app_secret":  {keys.PushedSecret},
		"target_type": {"app"},
		"content":     {text},
	}

	resp, err := http.PostForm("https://api.pushed.co/1/push", formData)

	if err != nil {
		log.Println(err)
	} else {
		log.Println(resp)
	}
}
