package main

import (
    "encoding/json"
    "html/template"
    "net/http"
    "strings"
    "math/rand"
    "strconv"
    "time"
    "fmt"
    "sort"
)

type DashboardPidData struct {
    Pid         string
    RewardTotal string
}

type DashboardData struct {
    PidData []DashboardPidData
}

type PageData struct {
    NextSyncUnix    int64
    PriceHistory    []string
    GuessHistory    []string
    RewardTotal     string
    News            string
}

type PredictionRequestBody struct {
    Guess float64 `json:"guess"`
    PID   string `json:"pid"`
}


var intervalSec int64 = 30
var nextSyncUnix = time.Now().UnixNano() / 1000000 + intervalSec * 1000 + 1000
var predictions = make([]map[string]float64, 1000)  // 1k rounds should be enough
var curPriceHist = []string{"100.00", "98.50", "97.44"}
var curRound = len(curPriceHist)
var news = map[string]string{}
var futureEffect = 0.0  // Offset to apply round n + 2 (i.e., future market movement)
var nextEffect = 0.0  // Offset to apply very next round

func UpdateSyncTime() {
    ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
    for {
        select {
        case <-ticker.C:
            nextSyncUnix = time.Now().UnixNano() / 1000000 + intervalSec * 1000 + 1000
            // Calculate mean predicted price as true price, then difference
            // from mean for each person, then convert into rewards *or*
            // penalties by making the mean = 0 (zero sum game)
            var sum float64
            for _, value := range predictions[curRound] {
                sum += value
            }
            mean := sum / float64(len(predictions[curRound])) + nextEffect
            diffs := make(map[string]float64)
            for pid, value := range predictions[curRound] {
                diff := mean - value
                if diff < 0 {
                    diff = -diff
                }
                diffs[pid] = diff
            }
            mean_diff := 0.0
            for _, diff := range diffs {
                mean_diff += diff
            }
            mean_diff = mean_diff / float64(len(diffs))
            for pid, diff := range diffs {
                predictions[curRound]["_reward_" + pid] = mean_diff - diff
            }

            if len(predictions[curRound]) > 0 {  // Some interaction, so start next round
                curPriceHist = append(curPriceHist, fmt.Sprintf("%.2f", mean))
                fmt.Println("Round", curRound, predictions[curRound])
                nextEffect = futureEffect
                futureEffect = 0
                curRound++
            }
        }
    }
}


func addNews(newsType string) {
    // Assume last round of guesses is list of current PIDs
    pids := []string{}
    for pid, _ := range predictions[len(curPriceHist) - 1] {
        if !strings.HasPrefix(pid, "_") {  // Real PID
            pids = append(pids, pid)
        }
    }
    sort.Slice(pids, func(i, j int) bool {
        a, _ := strconv.Atoi(pids[i])
        b, _ := strconv.Atoi(pids[j])
        return a < b
    })
    news = make(map[string]string)
    for i := 0; i < len(pids) / 2; i++ {
        news[pids[i]] = newsType
    }
    fmt.Println("Inserted news:", news)
}


func main() {
    for rnd := 0; rnd < len(predictions); rnd++ {  // Initialize empty maps
        predictions[rnd] = make(map[string]float64)
    }

    go UpdateSyncTime()

    fs := http.FileServer(http.Dir("./static"))
    http.Handle("/static/", http.StripPrefix("/static/", fs))

    http.HandleFunc("/prices", func(w http.ResponseWriter, r *http.Request) {
        pid := r.URL.Query().Get("id")
        if pid == "" {
            // Generate a random ID
            randomID := rand.Intn(1000000)
            // Redirect to the same page with the ID
            http.Redirect(w, r, fmt.Sprintf("/prices?id=%d", randomID), http.StatusFound)
            return
        }

        // Define the data to insert into the template
        guessHist := []string{}
        rewardTot := 0.0
        for rnd := 0; rnd < curRound; rnd++ {
            if guess, ok := predictions[rnd][pid]; ok {
                guessHist = append(guessHist, fmt.Sprintf("%.2f", guess))
                rewardTot += predictions[rnd]["_reward_" + pid]
            } else {
                guessHist = append(guessHist, " --")
            }
        }
        pidNews, _ := news[pid]
        delete(news, pid)  // Delete news after user has seen it
        data := PageData{
            NextSyncUnix: nextSyncUnix,
            PriceHistory: curPriceHist,
            GuessHistory: guessHist,
            RewardTotal: fmt.Sprintf("%.2f", rewardTot),
            News: pidNews,
        }

        // Parse the HTML template
        tmpl, err := template.ParseFiles("prices.html")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // Execute the template, inserting the data
        err = tmpl.Execute(w, data)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
    })

    // Handle predictions
    http.HandleFunc("/predict", func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" {
            // Read prediction from the request body
            var requestBody PredictionRequestBody
            err := json.NewDecoder(r.Body).Decode(&requestBody)
            if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            fmt.Printf("Received guess: %f\n", requestBody.Guess)
            // Store the prediction
            predictions[curRound][requestBody.PID] = requestBody.Guess

            // Respond with a success message
            fmt.Fprintf(w, "k")
        } else {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        }
    })

    // Dashboard
    http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
        pidRewards := make(map[string]float64)
        for rnd := 0; rnd < curRound; rnd++ {
            for pid, _ := range predictions[rnd] {
                if !strings.HasPrefix(pid, "_") {  // Real PID
                    pidRewards[pid] += predictions[rnd]["_reward_" + pid]
                }
            }
        }
        pidData := []DashboardPidData{}
        for pid, reward := range pidRewards {
            pidData = append(pidData, DashboardPidData{
                Pid: pid,
                RewardTotal: fmt.Sprintf("%.2f", reward),
            })
        }
        // Sort list in order of rewards (and PID within equal rewards)
        sort.Slice(pidData, func(i, j int) bool {
            a, _ := strconv.ParseFloat(pidData[i].RewardTotal, 64)
            b, _ := strconv.ParseFloat(pidData[j].RewardTotal, 64)
            return a < b || a == b && pidData[i].Pid < pidData[j].Pid
        })
        data := DashboardData{
            PidData: pidData,
        }
        tmpl, err := template.ParseFiles("dashboard.html")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // Execute the template, inserting the data
        err = tmpl.Execute(w, data)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
    })

    // Insert outside information for some people (lower half of PIDs)
    http.HandleFunc("/news/positive", func(w http.ResponseWriter, r *http.Request) {
        addNews("positive")
    })
    http.HandleFunc("/news/negative", func(w http.ResponseWriter, r *http.Request) {
        addNews("negative")
    })

    http.HandleFunc("/effect/positive", func(w http.ResponseWriter, r *http.Request) {
        futureEffect = 10
        fmt.Println("Added positive effect for next round")
        addNews("positive")  // Then send news to users
    })
    http.HandleFunc("/effect/negative", func(w http.ResponseWriter, r *http.Request) {
        futureEffect = -10
        fmt.Println("Added negative effect for next round")
        addNews("negative")
    })

    // Start the server
    fmt.Println("Starting server")
    http.ListenAndServe(":3008", nil)
}
