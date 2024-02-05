var last_price = 0


function submit_prediction() {
    const guess = parseFloat(document.getElementById('guess').value)
    if (!(guess >= 0)) {
        alert('Please enter your guess as a positive amount (no dollar sign).')
        return
    }
    if (last_price && Math.abs(last_price - guess) > 50) {
        alert('Guess must be within $50 of current price.')
        return
    }
    const params = new URLSearchParams(window.location.search)
    const pid = params.get('id')
    document.getElementById('btn-submit').style.display = 'none'
    document.getElementById('guess').style.display = 'none'

    // Using fetch to post data to the /predict endpoint
    fetch('/predict', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({guess: guess, pid: pid}),
    })
    .then(response => response.text())
    .then(data => {
        if (data === 'k') {
            document.getElementById('your-guess').textContent = 'Your guess: $' + guess.toFixed(2)
        } else {
            alert('Server error!')
        }
    })
    .catch((error) => {
        console.error('Error:', error)
        alert('Failed to submit prediction.')
    })
}

function start_countdown(target_unix) {
    const countdownElement = document.getElementById('countdown')

    const interval = setInterval(() => {
        const delta_sec = Math.round((target_unix - Date.now()) / 1000)
        countdownElement.textContent = `Next round in: ${delta_sec} seconds`

        if (delta_sec <= 0) {  // When countdown reaches zero, stop updating
            clearInterval(interval)
            countdownElement.textContent = 'Countdown has ended!'
            location.reload()
        }
    }, 300)
}

document.addEventListener('DOMContentLoaded', function() {
    const next_sync_unix = document.getElementById('next-sync-unix').value
    start_countdown(next_sync_unix)
    document.getElementById('btn-submit').onclick = () => submit_prediction()

    const guess_elem = document.getElementById('guess')
    guess_elem.addEventListener('keypress', function(event) {
        if (event.key === 'Enter') {
            submit_prediction()
        }
    })
    guess_elem.focus()

    // Get last price to use as validation point of comparison
    const last_elem = document.querySelector('ul#price-history li:last-child')
    last_price = parseFloat(last_elem?.textContent?.slice(1))

    // Show any news
    const pos_elem = document.querySelector('div#news.positive')
    if (pos_elem) {
        pos_elem.innerHTML = 'INFO: News reports suggest the price is likely to increase by $10 in the next round'
    }
    const neg_elem = document.querySelector('div#news.negative')
    if (neg_elem) {
        neg_elem.innerHTML = 'INFO: News reports suggest the price is likely to decrease by $10 in the next round'
    }
})
